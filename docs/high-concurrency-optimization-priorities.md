# MeshCart 高并发优化优先级建议

本文档基于当前项目前 10 轮 `k6` 压测结果整理，重点参考第九轮和第十轮结果，不再停留在“可能的高并发方案”层面，而是回答三个更直接的问题：

- 现在系统最先扛不住的是哪里
- 接下来应该先做哪些优化
- 哪些优化现在值得做，哪些可以后置

原始压测过程与逐轮记录见 [k6-loadtest-plan.md](/Users/ruitong/GolandProjects/MeshCart/docs/k6-loadtest-plan.md)。

## 1. 结论先行

基于第九轮和第十轮压测结果，当前系统最值得优先优化的，不是“全面上缓存”或“全面上消息队列”，而是订单创建这条同步交易主链路。

当前优先级最高的 4 类优化项是：

1. 收缩 `order-service` 下单主链路里的同步编排和串行依赖
2. 给下单依赖的商品 / SKU 快照读路径加缓存
3. 调整 `order-service`、`product-service`、`inventory-service` 的数据库连接池和访问模式
4. 把支付成功后的后置动作、审计日志、统计类写入从主链路移走，逐步事件化

当前不建议一上来就把重点放在：

1. 把库存主流程改成缓存优先
2. 用 MQ 直接替代订单创建主链路
3. 把订单创建整体改成纯异步

原因很明确：

- 第九轮显示系统在高强度下已经暴露出订单链路和登录链路的明显失稳
- 第十轮在关闭网关限流干扰后进一步确认，真实首瓶颈已经落在 `gateway -> order-service` 的同步下单路径，而不是支付服务先崩，也不是热点库存先成为唯一矛盾

## 2. 第九轮和第十轮告诉了我们什么

### 2.1 第九轮确认了高并发下的主要薄弱点

第九轮的关键现象见 [k6-loadtest-plan.md](/Users/ruitong/GolandProjects/MeshCart/docs/k6-loadtest-plan.md#L1695)。

从第九轮可以确认：

1. 读链路在更高强度下已经进入业务级失稳区，但还不是最重的问题
2. 登录链路在更高请求速率下出现明显 RT 抬升和业务失败
3. 订单链路在 `50 VUs` 已经进入高延迟区，`p95` 明显升高
4. 支付链路的失败，主要受前置链路拖累

换句话说，第九轮的判断是：

- 入口链路有问题
- 订单链路问题更大
- 支付服务不是当前最先该动刀的地方

### 2.2 第十轮进一步确认首瓶颈在订单主链路

第十轮的关键现象见 [k6-loadtest-plan.md](/Users/ruitong/GolandProjects/MeshCart/docs/k6-loadtest-plan.md#L1990)。

第十轮在关闭网关限流干扰后，重点压了核心交易链路，结果更清楚：

1. 热点 SKU 下单和普通 SKU 下单，在 `10 VUs` 与 `40 VUs` 下都已经接近 `100%` 业务失败
2. checkout 链路的高失败率，主要来自订单创建前置失败
3. `gateway` 日志里出现大量 `order rpc create returned business error` 和 `rpc timeout`
4. 热点 SKU 和普通 SKU 的失败率差异不大，说明系统还没走到“库存热点竞争才是首矛盾”的阶段

因此第十轮实际确认的是：

- 当前首瓶颈不是支付服务
- 当前首瓶颈也不是库存热点本身
- 当前首瓶颈是订单服务同步下单路径太重

## 3. 当前最该着手的优化项

## 3.1 优先级 P0：先瘦身订单创建主链路

这是当前最应该先动的地方。

`order-service` 当前的下单主流程里，除了幂等记录和订单写库，还会同步做商品和 SKU 校验、库存预占，以及失败后的库存回滚。[create_order.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/create_order.go#L18)

更关键的是，校验阶段不是一次批量完成，而是：

1. 先 `BatchGetSKU`
2. 再按商品逐个 `GetProductDetail`
3. 再生成订单快照和库存预占参数

这段逻辑见 [helpers.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/helpers.go#L105) 和 [helpers.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/helpers.go#L140)。

这意味着一次下单请求至少会放大成：

- 一次幂等记录读写
- 一次 SKU 批量 RPC
- 多次商品详情 RPC
- 一次库存预占 RPC
- 一次订单写库

在高并发下，这条链路非常容易先把 `order-service` 自己压垮。

### 当前已完成的改动

这一项已经连续落了两刀，当前目标是先把下单校验里的同步放大点真正收缩掉。

已经完成的改动：

1. 给 `product-service` 增加了真正的批量商品读取 RPC：`batchGetProducts`
2. 把 `order-service` 里的商品校验从“逐商品详情 RPC”进一步收敛成“单次批量商品读取 RPC”
3. 保留了 `BatchGetSKU`，但商品侧已经不再逐个 `GetProductDetail`
4. 给 `CreateOrder` 增加了分阶段耗时日志，至少拆出了：
   - `validation_duration`
   - `reserve_duration`
   - `persist_duration`
   - `total_duration`

对应代码：

- `product-service` 批量商品读取能力：[batch_get_products.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/biz/service/batch_get_products.go#L14)
- `product-service` 批量商品 RPC handler：[batch_get_products.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/rpc/handler/batch_get_products.go#L15)
- 订单侧商品批量校验接入：[helpers.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/helpers.go#L171)
- 订单侧产品 RPC client 新增批量接口：[client.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/rpcclient/product/client.go#L45)
- 订单创建分阶段耗时日志：[create_order.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/create_order.go#L18)

这一步带来的直接收益是：

1. 把“1 次 SKU 批量 RPC + N 次商品详情 RPC”收敛成“1 次 SKU 批量 RPC + 1 次商品批量 RPC”
2. 明显减少订单校验阶段的 RPC 次数和串行放大风险
3. 为下一轮压测提供更细粒度的主链路证据，不再只能看总 RT

### 这一项还没有结束

当前这版已经达到“商品读取改成批量化”的目标，但 P0 这一项还没有完全结束。

还剩下的核心优化项：

1. 继续明确主链路最小同步集合，只保留必要校验、库存预占、订单入库和返回订单号
2. 把非关键日志扩展、统计、通知类逻辑从主链路移出去
3. 继续细化分阶段观测，把幂等检查耗时也单独拆出来
4. 评估幂等记录路径是否还能减少一次 DB 往返

建议下一步按下面顺序继续：

1. 继续盘点 `CreateOrder` 主路径里是否还有非必要同步动作
2. 把幂等检查和动作记录路径再做一轮收缩
3. 在下一轮压测里重点验证 `validation_duration` 是否明显下降

如果不先把主链路做瘦，单纯调大连接池或加 MQ，都只是缓解，不会真正解决第十轮暴露出来的问题。

## 3.2 优先级 P0：给下单依赖的商品 / SKU 快照加缓存

从第十轮结果看，缓存现在值得做，但重点不是先给商品详情页做读缓存，而是先服务于下单链路。

原因：

1. 下单时需要的商品标题、SKU 标题、售价、商品状态、SKU 状态，本质上都是高频读、低频改的数据
2. 当前订单服务在校验阶段对这些数据做了同步 RPC 访问
3. 这些读取在高并发下会直接扩大 `order-service -> product-service` 压力

建议优先考虑的缓存项：

1. 商品基础快照缓存
2. SKU 基础信息缓存
3. 批量商品 / SKU 查询结果缓存

建议的目标不是“完全依赖缓存决定交易”，而是：

- 优先让订单服务拿到构建快照所需的只读数据
- 减少同步 RPC 次数
- 缩短订单主链路的前置校验时间

### 当前状态

这一项现在已经按更合理的方式落地了第一版：缓存放在 `product-service` 内部，用 Redis 做 cache-aside，而不是让 `order-service` 直接依赖 Redis。

当前判断是：

1. `order-service` 继续只调用 `product-service`，不直接碰缓存中间件
2. `product-service` 在 `BatchGetProducts` 和 `BatchGetSKU` 两条读路径里先查 Redis，未命中再回源数据库，并把结果回填 Redis
3. 商品创建、更新、上下线时会主动删缓存，避免旧快照长期残留
4. Redis 不可用时会自动降级回数据库，不会阻塞主流程

对应代码：

- 商品批量读取 cache-aside：[batch_get_products.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/biz/service/batch_get_products.go#L14)
- SKU 批量读取 cache-aside：[batch_get_sku.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/biz/service/batch_get_sku.go#L14)
- Redis 缓存封装：[redis.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/dal/redis/redis.go#L14)
- 商品 / SKU 缓存失效：[cache_helpers.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/biz/service/cache_helpers.go#L12)

### 这一项后续更合理的演进方向

1. 继续观察 `validation_duration`，确认 Redis 命中后下单校验耗时是否明显下降
2. 视下一轮压测结果决定是否需要单飞合并，避免缓存击穿时的瞬时回源放大
3. 根据商品更新频率和命中率，再调整 TTL、key 粒度和是否拆分更轻量的快照结构

这里要强调边界：

- 可以缓存商品和 SKU 基础信息
- 不建议直接把强一致库存判断改成缓存优先

## 3.3 优先级 P0：先调数据库连接池，再补数据库观测

当前多个服务的 MySQL 连接池默认都比较保守：

- [services/order-service/dal/db/db.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/dal/db/db.go#L21)
- [services/product-service/dal/db/db.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/dal/db/db.go#L21)
- [services/inventory-service/dal/db/db.go](/Users/ruitong/GolandProjects/MeshCart/services/inventory-service/dal/db/db.go#L21)

这些服务默认都使用：

- `SetMaxOpenConns(20)`
- `SetMaxIdleConns(10)`

对于第十轮已经暴露出的大量同步订单失败，这个池子规模偏小。

建议按这个顺序调整：

1. 先调大 `order-service` 连接池
2. 再调大 `product-service` 和 `inventory-service` 连接池
3. 同时补指标，不要只改参数不看证据

建议补的观测项：

1. 连接池等待时间
2. 活跃连接数 / 空闲连接数
3. 慢 SQL
4. 事务时长
5. 锁等待
6. 幂等表和库存表的热点 SQL 耗时

这里的原则是：

- 连接池要调
- 但连接池不是根因本身
- 必须和主链路瘦身、SQL 优化一起推进

### 当前已完成的改动

这一项已经落地了第一版，覆盖了当前最关键的 3 个服务：

1. `order-service`
2. `product-service`
3. `inventory-service`

已经完成的改动：

1. 把这 3 个服务的 MySQL 连接池参数改成可配置，不再写死 `20/10/30m`
2. 本地默认值调整为：
   - `max_open_conns=60`
   - `max_idle_conns=20`
   - `conn_max_lifetime_minutes=30`
3. 给 admin / metrics 侧补了数据库连接池指标采集，定时上报到 Prometheus

对应代码：

- 通用 DB pool metrics：[db.go](/Users/ruitong/GolandProjects/MeshCart/app/metrics/db.go#L10)
- `order-service` 连接池配置：[config.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/config/config.go#L12)
- `product-service` 连接池配置：[config.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/config/config.go#L12)
- `inventory-service` 连接池配置：[config.go](/Users/ruitong/GolandProjects/MeshCart/services/inventory-service/config/config.go#L11)
- `order-service` DB pool 接线与采集：[bootstrap.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/rpc/bootstrap/bootstrap.go#L139)

当前已经能从 metrics 里直接看到这些指标：

1. `meshcart_db_open_connections`
2. `meshcart_db_in_use_connections`
3. `meshcart_db_idle_connections`
4. `meshcart_db_wait_count_total`
5. `meshcart_db_wait_duration_seconds_total`
6. `meshcart_db_max_idle_closed_total`
7. `meshcart_db_max_idle_time_closed_total`
8. `meshcart_db_max_lifetime_closed_total`

这意味着下一轮压测时，已经可以直接判断：

1. 是不是连接池太小导致大量等待
2. 是不是连接长期处于高占用
3. 是不是连接频繁因为 idle / lifetime 被关闭

### 这一项还没有结束

当前这版先解决了“参数可调”和“连接池可观测”，但还没有把数据库观测补到完整形态。

还剩下的核心项：

1. 补慢 SQL 统计和按表 / 语句维度的聚合
2. 补事务时长观测
3. 补锁等待观测
4. 下一轮压测时，把 `validation_duration`、`reserve_duration`、连接池等待一起对齐分析

建议下一步优先做的不是继续盲目放大连接池，而是：

1. 启动服务后复跑核心下单压测
2. 对比 `order-service`、`product-service`、`inventory-service` 的连接池等待是否明显下降
3. 如果连接池等待仍不高，而 RT 依旧高，就继续往 SQL / 事务 / 幂等路径里找

## 3.4 优先级 P1：优化幂等记录和动作记录路径

从第十轮日志观察，订单创建前会先查 `order_action_records`，再决定是否创建 pending 记录，再继续主流程。

这条路径的问题不在于“有幂等保护”，而在于高并发下它可能放大为额外的数据库读写成本。

建议检查并优化：

1. 当前是否存在明显的“先查再写”往返
2. 是否可以依赖唯一键约束减少一次显式查询
3. 是否能避免一次请求触发多次动作记录状态更新
4. 幂等记录表上的索引是否已经覆盖 `action_type + action_key`

这个优化的价值是：

- 它不一定像缓存那样直观提升吞吐
- 但可以明显降低下单主链路里的 DB 往返次数

## 3.5 优先级 P1：把支付成功后的后置动作事件化

消息队列不是当前的第一刀，但不是不做。

结合第九轮和第十轮结果，MQ 现在最适合承接的是“交易后置动作”，不是“下单主流程本身”。

适合先异步化的内容：

1. 支付成功后的订单状态推进通知
2. 库存确认扣减后的非主流程通知
3. 审计日志
4. 统计聚合
5. 消息通知、积分、运营事件

这样做的收益是：

- 缩短支付确认和订单后处理链路
- 避免把不影响主交易结果的动作留在同步请求里

不建议当前就做的事情：

1. 下单请求先入 MQ 再慢慢创建订单
2. 库存预占改成纯异步
3. 把支付主状态更新完全改成最终一致返回

这些会明显增加一致性复杂度，而当前压测还没有证明必须走到那一步。

## 3.6 优先级 P1：库存热点治理放在下一阶段，不是当前第一刀

第九轮说明库存相关业务失败已经出现，第十轮则说明热点 SKU 和普通 SKU 的失败率差异并不大。

这两个结果拼在一起，比较合理的判断是：

- 库存链路有优化价值
- 但当前系统还没走到“热点库存竞争是唯一首瓶颈”的阶段

因此库存相关优化建议放在主链路瘦身之后推进：

1. 检查热点 SKU 预占 SQL 和索引
2. 缩短库存预占事务
3. 给热点 SKU 补锁等待和事务时长观测
4. 在后续专项压测里，再决定是否要做分片、排队或更激进的热点治理

## 4. 现在可以明确推进的改进清单

如果按“本周就可以开始做”的粒度拆解，建议先落下面这些动作。

### 4.1 第一批：直接改订单主链路

1. 收敛 `CreateOrder` 里的商品校验 RPC，减少逐商品详情查询
2. 给订单链路增加分阶段耗时日志和指标
3. 盘点并移出主链路中的非关键同步动作
4. 评估幂等记录路径能否减少一次显式查询

### 4.2 第二批：直接改缓存

1. 给订单创建依赖的商品快照加缓存
2. 给 SKU 基础信息加缓存
3. 给批量商品 / SKU 读取加短 TTL 缓存

缓存策略建议：

- 先做短 TTL
- 先做只读快照
- 先服务订单创建，不先追求大而全的商品读缓存体系

### 4.3 第三批：直接改资源参数和观测

1. 调大 `order-service`、`product-service`、`inventory-service` 的 MySQL 连接池
2. 补齐连接池指标
3. 补齐库存预占和订单落库相关慢 SQL 观测
4. 补齐 RPC 分阶段 RT 和错误码分桶

### 4.4 第四批：开始做事件化

1. 把支付后的非关键动作从同步请求里拆出去
2. 引入订单 / 支付领域事件，先做最小闭环
3. 审计、统计、通知类逻辑优先走异步

## 5. 当前最不建议的误区

### 5.1 不要把“加缓存”当成当前所有问题的答案

缓存现在值得做，但必须打在正确位置。

当前最有效的缓存不是“先把所有商品接口都缓存掉”，而是优先服务于下单链路的商品 / SKU 快照读取。

### 5.2 不要因为有 MQ 就把下单主流程异步化

第十轮暴露的是同步主链路过重，不等于应该直接把主交易一致性让位给异步。

当前 MQ 更适合做减负，而不是替代核心交易主流程。

### 5.3 不要只调连接池，不改访问路径

如果订单主链路仍然保留多跳同步 RPC 和逐商品详情查询，那么连接池调大之后，只会把堵点往后推，不会真正消失。

### 5.4 不要跳过观测就直接重构

第九轮和第十轮已经给出了方向，但后续具体改造仍然要靠更细的指标来判断收益。

否则很容易出现：

- 做了缓存，但真正耗时在幂等写库
- 调大连接池，但真正瓶颈在库存事务
- 上了 MQ，但主链路仍然太重

## 6. 推荐的实施顺序

如果按投入产出比排序，建议按下面顺序推进。

### 6.1 第一阶段：立刻做

1. 瘦身订单创建主链路
2. 给下单依赖的商品 / SKU 快照加缓存
3. 调大 `order-service`、`product-service`、`inventory-service` 的连接池
4. 补订单链路、库存链路、幂等路径的细粒度指标

### 6.2 第二阶段：紧接着做

1. 优化幂等记录和动作记录路径
2. 把支付成功后的后置动作事件化
3. 复跑核心交易链路压测，验证失败率和分阶段 RT 是否明显下降

### 6.3 第三阶段：压测验证后再做

1. 做库存热点专项治理
2. 决定是否需要热点 SKU 分片、排队或更重的库存架构演进
3. 再评估是否需要更完整的 MQ 事件总线或更复杂的缓存一致性体系

## 7. 最终建议

如果只保留一句话建议，那就是：

当前最该做的，是先把订单创建主链路做薄、把下单依赖的商品 / SKU 读取缓存起来、把数据库连接池和观测补起来，而不是急着把系统全面 MQ 化或把库存流程缓存化。

更具体地说，建议按这个优先级执行：

1. 先优化 `order-service` 同步下单主流程
2. 先给订单依赖的商品 / SKU 快照加缓存
3. 先调大连接池并补观测
4. 再异步化支付后的后置动作
5. 最后再做库存热点专项治理

这条路线最符合第九轮和第十轮已经暴露出来的真实问题，也最符合当前工程投入产出比。
