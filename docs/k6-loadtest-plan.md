# MeshCart k6 压测计划

## 1. 目标

本文档用于规范 `MeshCart` 的压测方式，统一：

- 压测工具：`k6`
- 压测对象：优先 `gateway` HTTP 接口
- 压测顺序：先单接口，再热点场景，最后交易链路
- 结果记录：每轮压测按固定模板留档
- 后续分析：每次压测结束后，把结果发给我，我继续做统计和结论输出

当前仓库已经具备基础观测能力：

- `gateway` metrics：`http://127.0.0.1:9092/metrics`
- `user-service` metrics：`http://127.0.0.1:9091/metrics`
- `product-service` metrics：`http://127.0.0.1:9093/metrics`
- 其余服务也有各自 admin metrics 端口
- `Prometheus`：`http://localhost:9090`
- `Grafana`：`http://localhost:3000`

因此本项目的压测不是只看 `k6` 输出，还要结合 Prometheus / Grafana 指标一起判断瓶颈。

## 2. 为什么使用 k6

不建议把 `Postman`、`Apifox` 作为正式压测工具。

原因：

- 它们更适合接口调试，不适合稳定地做并发和阶段升压
- 压测脚本难以版本化管理
- 阈值、场景、批次管理不如 `k6` 清晰

选择 `k6` 的原因：

- 易于脚本化定义用户行为
- 方便做分阶段升压
- 能直接输出 `p95`、`p99`、错误率、吞吐等核心指标
- 便于后续把压测脚本放进仓库长期维护

## 3. 压测范围

结合当前项目实现，建议分三层推进。

### 3.1 第一层：单接口压测

优先压以下接口：

1. `POST /api/v1/user/login`
2. `GET /api/v1/products`
3. `GET /api/v1/products/detail/:product_id`
4. `POST /api/v1/orders`
5. `POST /api/v1/payments`

目标：

- 建立每个核心接口的单独基线
- 快速识别是入口、读链路还是写链路先出问题

### 3.2 第二层：热点场景压测

优先压以下热点：

1. 单商品详情热点读
2. 单 SKU 高频下单
3. 单订单重复创建支付

目标：

- 暴露热点数据竞争
- 验证库存、订单、支付在并发下的稳定性

### 3.3 第三层：交易链路压测

建议链路：

1. 登录
2. 查商品
3. 创建订单
4. 创建支付单
5. 模拟支付成功

目标：

- 判断完整链路的总 RT、失败点和瓶颈放大位置

说明：

- 不建议一上来做全链路大压测
- 先完成单接口和热点场景，再做链路压测，定位效率更高

## 4. 压测前准备

每轮压测前，至少确认以下条件。

### 4.0 安装 k6

建议在 macOS 上优先使用 `Homebrew` 安装。

安装命令：

```bash
brew install k6
```

安装完成后检查版本：

```bash
k6 version
```

如果终端能正常输出版本号，说明安装成功。

如果你还没有安装 `Homebrew`，可以先检查：

```bash
brew --version
```

如果提示找不到 `brew`，说明需要先安装 `Homebrew`，再执行 `brew install k6`。

建议压测前再做一次最小验证：

```bash
k6 run --vus 1 --duration 5s - <<'EOF'
import http from 'k6/http';

export default function () {
  http.get('http://127.0.0.1:8080/healthz');
}
EOF
```

预期：

- `k6` 可以正常启动
- 能正常发出请求
- 没有语法错误或命令不存在错误

### 4.1 服务状态

- `gateway` 已启动
- 相关下游服务已启动
- `Consul` 注册正常
- `healthz` / `readyz` 正常
- 压测期间不要同时做代码发布、依赖升级、数据库迁移

### 4.2 数据准备

建议准备一套专门的压测数据，不要直接混用日常开发数据。

至少准备：

- 压测用户
- 可读商品数据
- 可下单商品和对应 SKU 库存
- 支付测试数据

建议固定以下测试对象：

- `test_user`
- `hot_product_id`
- `hot_sku_id`
- 必要时准备多个普通 `product_id`，用于对比“热点”和“非热点”

说明：

- 正式压测时，不建议一边压接口一边大规模造数据
- 更合理的方式是先准备测试数据，再用 `k6` 消费这些数据
- 后续开始压测前，我会按照本文档的策略帮你生成一批测试数据

### 4.2.1 数据准备策略

压测数据分成两类。

第一类是“基础测试数据”，需要在正式压测前准备好：

- 压测用户
- 商品
- SKU
- 库存
- 必要的订单或支付前置数据

第二类是“请求级动态字段”，可以在 `k6` 脚本里按请求动态生成：

- `request_id`
- 请求幂等键
- 随机请求备注
- 从预置商品池中随机挑选 `product_id`

### 4.2.2 为什么不建议正式压测时大量注册新用户

如果脚本一边压测一边持续注册用户，结果会混入很多非目标因素：

- 注册接口本身的写入开销
- 用户表持续膨胀
- 唯一键冲突和脏数据
- 登录压测和注册压测混在一起
- 不同轮结果难以比较

所以默认策略是：

1. 先准备一批测试用户
2. 登录压测只复用这些用户
3. 正式压测脚本不负责持续造用户

### 4.2.3 各场景的数据准备建议

#### 登录压测

提前准备：

- `20 ~ 100` 个测试用户

压测时脚本动态生成：

- 无需生成新用户
- 可以按虚拟用户编号轮换使用已有用户名

#### 商品列表 / 商品详情压测

提前准备：

- `1` 个热点商品
- `5 ~ 20` 个普通商品
- 如果详情依赖 SKU，确保 SKU 数据完整

压测时脚本动态生成：

- 在普通商品池中随机选一个 `product_id`
- 热点场景则固定打同一个 `product_id`

#### 下单压测

提前准备：

- 可下单用户
- 可售商品
- 热点 `sku_id`
- 足够库存

压测时脚本动态生成：

- `request_id`
- 同一商品上的不同下单请求参数

#### 支付压测

提前准备：

- 已能成功创建订单的测试账户和商品数据

压测时脚本动态生成：

- `request_id`
- 订单创建后的支付请求参数

### 4.2.4 推荐的数据准备顺序

建议正式压测前按这个顺序准备：

1. 创建压测用户
2. 准备普通商品和热点商品
3. 确认热点 SKU 和库存
4. 验证登录、查商品、下单、支付链路都能走通
5. 再开始 `k6` 压测

### 4.2.5 后续协作方式

后面进入压测阶段时，我会按计划协助你做两类事情：

1. 正式压测前，帮你生成测试数据或补数据准备脚本
2. 每轮压测结束后，基于你发来的结果做统计、对比和问题分析

默认原则：

- 数据准备和正式压测分开
- 正式压测脚本尽量只做压测，不做大规模造数
- 如果某个场景确实需要动态补少量数据，我会单独说明并控制范围

### 4.3 环境冻结

每轮压测前记录：

- Git commit
- 配置版本
- 限流开关和阈值
- MySQL / Redis / Consul / Prometheus / Grafana 是否同一套环境

否则不同批次之间不可比。

## 5. 压测原则

### 5.1 单次只改一个变量

例如每轮只变更其中一个：

- 并发数
- 持续时间
- 是否开启限流
- 是否改动缓存
- 是否改动订单 / 支付幂等逻辑

避免多个变量同时变化，导致结果无法解释。

### 5.2 先小流量试跑，再正式升压

建议顺序：

1. 冒烟压测
2. 基线压测
3. 阶梯升压
4. 定点复测

### 5.3 每次压测都保留原始结果

至少保留：

- `k6` 控制台摘要
- 如有条件，导出 JSON 或文本摘要
- 对应时段的 Prometheus / Grafana 关键指标截图或数值

## 6. 推荐的批次计划

建议按以下批次执行。

### 6.1 批次 A：冒烟压测

目的：

- 验证脚本可运行
- 确认 token、参数、数据依赖没有问题

建议强度：

- `vus=1~5`
- `duration=30s~1m`

通过标准：

- 请求流程正确
- 错误率接近 `0`
- 没有明显的鉴权、参数、库存不足、幂等冲突等非性能错误

### 6.2 批次 B：基线压测

目的：

- 建立当前版本的单接口基线

建议强度：

- `vus=10`
- `duration=3m`

记录重点：

- `avg`
- `median`
- `p90`
- `p95`
- `p99`
- `http_req_failed`
- `iterations`
- `req/s`

### 6.3 批次 C：阶梯升压

目的：

- 找到开始抖动、报错或超时的拐点

建议强度：

1. `vus=10 duration=3m`
2. `vus=20 duration=3m`
3. `vus=50 duration=3m`
4. `vus=100 duration=3m`

如果系统稳定，再继续加压。

观察点：

- RT 是否阶跃上升
- 错误率是否突然放大
- 限流是否开始明显命中
- 下游 RPC 是否超时
- MySQL / Redis 是否出现明显等待

### 6.4 批次 D：热点专项

目标场景：

1. 固定 `product_id` 做热点详情读
2. 固定 `sku_id` 做热点下单
3. 固定 `order_id` 或同订单反复创建支付，观察幂等与唯一性保护

记录重点：

- 热点读和普通读 RT 差异
- 热点写时错误码分布
- 库存相关失败比例
- 支付重复创建是否被复用或拦截

### 6.5 批次 E：链路压测

链路建议：

1. 登录
2. 查询商品详情
3. 创建订单
4. 创建支付单

如后续脚本成熟，再加入：

5. 模拟支付成功

说明：

- 链路压测一定要在单接口基线稳定后再做
- 否则看到链路失败，也很难知道是哪一跳先出问题

## 7. 每轮压测必须记录的指标

### 7.1 k6 输出指标

每轮都记录：

- 场景名
- 接口或链路名
- `vus`
- `duration`
- 总请求数
- `req/s`
- `http_req_duration avg`
- `http_req_duration median`
- `http_req_duration p90`
- `http_req_duration p95`
- `http_req_duration p99`
- `http_req_failed`
- `checks`

### 7.2 系统观测指标

每轮都尽量同步记录：

- `gateway` 请求量和 RT
- `gateway` 错误数
- `gateway` 限流命中情况
- `user-service` / `product-service` / `order-service` / `payment-service` / `inventory-service` RPC RT
- RPC 错误数
- MySQL CPU
- MySQL 慢查询
- MySQL 锁等待
- Redis RT
- Redis 超时或失败

### 7.3 现象记录

除了数字，也要记录现象：

- 是否出现大量 `timeout`
- 是否出现大量限流
- 是否出现库存不足误判
- 是否出现重复支付或重复订单
- 是否出现单个下游服务先抖动

## 8. 结果归档格式

建议在仓库外或你自己的记录区，按批次保存结果。

建议目录结构：

```text
loadtest-results/
  2026-03-24/
    A-login-smoke.txt
    B-product-detail-baseline.txt
    C-order-create-vus50.txt
    D-hot-sku-order-vus100.txt
```

如果你不想自己建目录，至少把每轮结果按下面模板发给我。

## 9. 发给我的数据模板

每次压测结束后，直接按这个模板把结果发我。

```text
【压测批次】
B

【时间】
2026-03-24 20:30:00 ~ 2026-03-24 20:33:00

【代码版本】
git commit: xxxxxxx

【场景名】
product-detail-baseline

【接口 / 链路】
GET /api/v1/products/detail/:product_id

【测试目标】
单接口基线

【压测参数】
vus=10
duration=3m
product_id=192000000000000001

【k6 摘要】
total requests=
req/s=
http_req_failed=
avg=
median=
p90=
p95=
p99=

【系统指标】
gateway RT=
gateway errors=
product-service RPC RT=
product-service RPC errors=
mysql cpu=
mysql lock wait=
redis rt=

【异常现象】
无 / 有，描述如下：

【你的判断】
可先留空
```

如果你能直接贴 `k6` 原始摘要，我会基于摘要帮你二次整理。

## 10. 我会如何帮你统计

你每次把数据发给我后，我会继续帮你做以下工作：

1. 生成该轮压测摘要
2. 和历史批次做横向对比
3. 标出性能拐点和异常批次
4. 区分是入口瓶颈、下游 RPC 瓶颈、DB 锁竞争还是限流生效
5. 给出下一轮该如何调参

我重点会看：

- 同一接口在不同 `vus` 下的 `p95`、`p99` 变化
- 错误率是否在某个并发点突然升高
- 是否存在“k6 看起来慢，但服务指标没慢”的客户端假象
- 是否存在“gateway 正常，但某个下游服务先抖动”的局部瓶颈
- 是否存在热点 SKU 竞争导致的库存写放大

## 11. 第一阶段推荐执行顺序

建议先按这个顺序跑，不要跳。

1. `POST /api/v1/user/login`
2. `GET /api/v1/products`
3. `GET /api/v1/products/detail/:product_id`
4. `POST /api/v1/orders`
5. `POST /api/v1/payments`
6. 热点商品详情读
7. 热点 SKU 下单
8. 登录 -> 商品详情 -> 创建订单 -> 创建支付

原因：

- 登录是入口热点
- 商品读接口最容易先建立基线
- 下单和支付是核心写链路
- 热点场景最容易暴露真实并发问题

### 11.1 第一阶段的明确范围

第一阶段先做“单接口基线压测”，分成两个小阶段。

#### 第一阶段 A：先做读链路和入口链路

先压这 3 个接口：

1. `POST /api/v1/user/login`
2. `GET /api/v1/products`
3. `GET /api/v1/products/detail/:product_id`

原因：

- 这 3 个接口最容易先跑通
- 能先建立 gateway、user-service、product-service 的基线
- 失败时更容易定位，不会把订单、库存、支付问题混进来

#### 第一阶段 B：在 A 稳定后，再补核心写接口

再压这 2 个接口：

1. `POST /api/v1/orders`
2. `POST /api/v1/payments`

原因：

- 下单会引入 product / inventory / order 多跳依赖
- 创建支付会引入 order / payment 的状态约束
- 这两个接口更适合在读链路和登录链路稳定后再单独建立基线

### 11.2 第一阶段需要准备的数据

为了跑完第一阶段，建议至少准备下面这批数据：

- `1` 个管理员账号，用于创建测试商品
- `20` 个压测用户，用于登录和下单
- `1` 个热点商品
- `5` 个普通商品
- 每个商品至少 `1` 个可售 SKU
- 热点 SKU 充足库存，例如 `5000`
- 普通 SKU 基线库存，例如每个 `200`

这样可以覆盖：

- 登录压测：复用压测用户
- 商品列表压测：读普通商品池
- 商品详情压测：普通详情和热点详情
- 下单压测：用压测用户对热点 SKU 或普通 SKU 下单
- 支付压测：先创建订单，再对订单创建支付单

### 11.3 第一阶段测试数据准备工具

仓库已新增数据准备工具：

- [prepare_phase1_data](/Users/ruitong/GolandProjects/MeshCart/loadtest/cmd/prepare_phase1_data/main.go)

用途：

- 创建或复用一个管理员账号
- 创建或复用一批压测用户
- 创建 `1` 个热点商品
- 创建若干普通商品
- 创建商品时自动初始化库存
- 输出一份 manifest，记录用户名、商品 ID、SKU ID、库存等信息

运行示例：

```bash
go run ./loadtest/cmd/prepare_phase1_data \
  -base-url http://127.0.0.1:8080 \
  -user-count 20 \
  -normal-product-count 5 \
  -hot-stock 5000 \
  -normal-stock 200 \
  -output loadtest/results/phase1-manifest.json
```

默认会准备：

- 管理员账号：`loadtest_superadmin / Loadtest123456`
- 压测用户前缀：`loadtest_user_01 ...`
- 压测用户密码：`Loadtest123456`

执行前提：

- `gateway` 已启动
- `user-service`、`product-service`、`inventory-service` 已启动
- 首次运行时，如果库里还没有用户，脚本创建的管理员会自动成为首个 `superadmin`

脚本输出结果建议保存下来，后续 `k6` 脚本会直接消费这份 manifest。

## 12. 当前阶段先不要追求的事

当前不建议一上来做这些事：

- 一次把所有接口都压一遍
- 一上来直接超大并发全链路压测
- 在没有稳定测试数据的前提下测库存写热点
- 还没固定环境就反复比较不同轮结果

先把基线、热点和记录模板跑顺，后面的分析才有意义。

## 13. 下一步

本文档先定义计划和记录方式。

下一步建议我继续补两部分：

1. `k6` 脚本目录和基础脚本
2. 每个核心接口的执行命令示例

如果你要继续，我下一步直接在仓库里补：

- `loadtest/k6/`
- 通用配置
- 登录、商品详情、下单、支付的基础压测脚本

## 14. 第一轮结果

时间：

- `2026-03-24`

本轮实际完成：

- 第一阶段 A：`login`
- 第一阶段 A：`products list`
- 第一阶段 A：`product detail`
- 第一阶段 B：`create order`
- 第一阶段 B：`create payment`

结果文件：

- [phase1-manifest.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/phase1-manifest.json)
- [phase1-login-summary.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/phase1-login-summary.json)
- [phase1-products-list-summary.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/phase1-products-list-summary.json)
- [phase1-product-detail-summary.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/phase1-product-detail-summary.json)
- [phase1-order-create-summary.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/phase1-order-create-summary.json)
- [phase1-payment-create-summary.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/phase1-payment-create-summary.json)

### 14.1 测试数据

本轮已准备：

- 管理员账号：`loadtest_superadmin`
- 压测用户：`20` 个
- 热点商品：`1` 个
- 普通商品：`5` 个
- 热点 SKU 初始库存：`5000`
- 普通 SKU 初始库存：`200`

### 14.2 第一轮基线结果

#### `POST /api/v1/user/login`

- 场景：第一阶段 A 登录基线
- 参数：`3 req/s`，`30s`
- 总请求数：`91`
- 错误率：`0%`
- `avg`：`103.55ms`
- `p95`：`118.38ms`

#### `GET /api/v1/products`

- 场景：第一阶段 A 商品列表基线
- 参数：`10 VUs`，`30s`
- 总请求数：`1440`
- 吞吐：`47.86 req/s`
- 错误率：`0%`
- `avg`：`7.59ms`
- `p95`：`23.19ms`

#### `GET /api/v1/products/detail/:product_id`

- 场景：第一阶段 A 商品详情基线
- 参数：`3 VUs`，`30s`，`sleep=0.7s`
- 总请求数：`129`
- 吞吐：`4.23 req/s`
- 错误率：`0%`
- `avg`：`7.82ms`
- `p95`：`18.58ms`

#### `POST /api/v1/orders`

- 场景：第一阶段 B 下单基线
- 参数：`1 VU`，`30s`，`sleep=1s`
- 迭代数：`27`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`38.6ms`
- 自定义指标 `order_create_duration p95`：`60.54ms`

#### `POST /api/v1/payments`

- 场景：第一阶段 B 创建支付基线
- 参数：`1 VU`，`30s`，`sleep=1s`
- 迭代数：`26`
- 错误率：`0%`
- 自定义指标 `payment_create_duration avg`：`18.18ms`
- 自定义指标 `payment_create_duration p95`：`25.11ms`

### 14.3 第一轮现象与结论

本轮确认：

- 第一阶段 A 和第一阶段 B 的最小基线都已经跑通
- 当前登录、商品读、下单、创建支付在低到中等强度下都没有业务错误
- 商品列表和商品详情的 RT 明显低于登录接口
- 写链路在当前低强度下也比较稳定

本轮还发现了几个重要事项：

- 网关默认全局 IP 限流会影响热点详情压测，商品详情升压时不能直接沿用商品列表的压测强度
- `k6` 脚本里如果直接把雪花 ID 当普通 JSON number 传递，会出现精度丢失，需要按字符串或原始 JSON 文本处理
- 登录返回的 `access_token` 已经自带 `Bearer ` 前缀，脚本不能重复拼接

### 14.4 下一轮建议

建议第二轮按下面方式继续：

1. `login` 做阶梯升压，但不要直接超过登录限流阈值
2. `products list` 从当前基线继续向上推，观察何时命中全局 IP 限流
3. `product detail` 单独做热点升压，记录限流开始出现的拐点
4. `create order` 从 `1 VU` 提升到 `2 VUs`、`5 VUs`
5. `create payment` 从 `1 VU` 提升到 `2 VUs`、`5 VUs`

## 15. 第二轮结果

时间：

- `2026-03-24`

第二轮目标：

- 做阶梯升压
- 区分“性能瓶颈”和“网关限流”

### 15.1 第二轮开始前的现象

在不改网关配置时，第二轮一开始就确认了限流已经显著影响结果：

- `login` 提升到 `7 req/s` 时开始出现业务拒绝
- `products list` 提升到 `12 VUs` 时，大量请求被瞬时拒绝

这说明当前默认限流更适合作为生产护栏，不适合继续做本地第二轮性能阶梯。

### 15.2 第二轮临时调整

为了继续拿到“去限流干扰”的性能结果，本轮临时调高了网关限流阈值并重启 `gateway`：

- `GATEWAY_GLOBAL_IP_RATE_LIMIT_RPS=300`
- `GATEWAY_GLOBAL_IP_RATE_LIMIT_BURST=600`
- `GATEWAY_LOGIN_IP_RATE_LIMIT_RPS=30`
- `GATEWAY_LOGIN_IP_RATE_LIMIT_BURST=60`
- `GATEWAY_REGISTER_IP_RATE_LIMIT_RPS=20`
- `GATEWAY_REGISTER_IP_RATE_LIMIT_BURST=40`

说明：

- 这些是压测期临时参数
- 如果压测结束后要恢复默认护栏，需要把网关按原阈值重启

### 15.3 第二轮结果文件

- [round2-login-rate5.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-login-rate5.json)
- [round2-login-rate7.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-login-rate7.json)
- [round2-login-rate10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-login-rate10.json)
- [round2-products-list-v12.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-products-list-v12.json)
- [round2-products-list-v12-relaxed.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-products-list-v12-relaxed.json)
- [round2-product-detail-v10-relaxed.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-product-detail-v10-relaxed.json)
- [round2-order-v5.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-order-v5.json)
- [round2-payment-v5.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round2-payment-v5.json)

### 15.4 第二轮关键结果

#### `POST /api/v1/user/login`

默认限流下：

- `5 req/s`：稳定
- `7 req/s`：开始出现业务拒绝

放宽限流后：

- 场景：`10 req/s`，`20s`
- 总请求数：`200`
- 错误率：`0%`
- `avg`：`101.08ms`
- `p95`：`109.94ms`

结论：

- 登录本身在 `10 req/s` 下依然稳定
- 第一批拐点来自登录限流，而不是登录链路性能先抖

#### `GET /api/v1/products`

默认限流下：

- `12 VUs` 已经出现大量业务拒绝，不适合作为真实性能结果

放宽限流后：

- 场景：`12 VUs`，`20s`，`sleep=0.2s`
- 总请求数：`1128`
- 吞吐：`55.85 req/s`
- 错误率：`0%`
- `avg`：`12.31ms`
- `p95`：`34.7ms`

结论：

- 商品列表在 `~56 req/s` 下仍然稳定
- 说明第一轮接近 `50 req/s` 时看到的主要不是服务瓶颈，而是全局 IP 限流

#### `GET /api/v1/products/detail/:product_id`

放宽限流后：

- 场景：`10 VUs`，`20s`，`sleep=0.2s`
- 总请求数：`960`
- 吞吐：`48.0 req/s`
- 错误率：`0%`
- `avg`：`7.07ms`
- `p95`：`10.63ms`

结论：

- 热点详情在接近 `50 req/s` 下仍然稳定
- 当前热点详情链路的纯读性能明显好于登录链路

#### `POST /api/v1/orders`

放宽限流后：

- 场景：`5 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`160`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`31.33ms`
- 自定义指标 `order_create_duration p95`：`63.27ms`

结论：

- 下单写链路在 `5 VUs` 下稳定
- 当前没有出现库存、幂等或写链路抖动的直接迹象

#### `POST /api/v1/payments`

放宽限流后：

- 场景：`5 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`149`
- 错误率：`0%`
- 自定义指标 `payment_create_duration avg`：`36.24ms`
- 自定义指标 `payment_create_duration p95`：`107.41ms`

结论：

- 创建支付在 `5 VUs` 下稳定
- 相比第一轮，支付 RT 有上升，但还远不到危险区间

### 15.5 第二轮结论

第二轮主要结论：

1. 默认网关限流会显著影响压测结论，尤其是登录和热点读接口
2. 放宽限流后，当前项目在第二轮强度下仍然整体稳定
3. 登录链路 RT 明显高于商品读链路
4. 商品列表和热点详情在 `~50 req/s` 量级下还没有出现明显性能问题
5. 订单创建和支付创建在 `5 VUs` 下仍然稳定，没有业务错误

### 15.6 第三轮建议

第三轮建议分两条线继续：

1. 读链路继续升压
   - `products list`：`20 VUs`
   - `product detail`：`15 VUs`、`20 VUs`
2. 写链路继续升压
   - `create order`：`10 VUs`
   - `create payment`：`10 VUs`

同时建议开始结合以下观测一起看：

- `gateway` metrics
- `order-service` / `payment-service` / `inventory-service` RPC RT
- MySQL 锁等待
- Redis RT

## 16. 第三轮结果

时间：

- `2026-03-24`

第三轮目标：

- 在第二轮临时放宽网关限流的基础上，继续提升读链路和写链路强度
- 观察真正的 RT 抬升趋势，而不是只看限流命中

### 16.1 第三轮结果文件

- [round3-products-list-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round3-products-list-v20.json)
- [round3-product-detail-v15.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round3-product-detail-v15.json)
- [round3-product-detail-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round3-product-detail-v20.json)
- [round3-order-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round3-order-v10.json)
- [round3-payment-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round3-payment-v10.json)

### 16.2 第三轮关键结果

#### `GET /api/v1/products`

- 场景：`20 VUs`，`20s`，`sleep=0.2s`
- 总请求数：`1920`
- 吞吐：`95.61 req/s`
- 错误率：`0%`
- `avg`：`7.92ms`
- `p95`：`19.52ms`

结论：

- 商品列表继续升压到 `~96 req/s` 后仍然稳定
- 相比第二轮 `12 VUs`，读链路没有出现明显恶化

#### `GET /api/v1/products/detail/:product_id`

第一档：

- 场景：`15 VUs`，`20s`，`sleep=0.2s`
- 吞吐：`71.77 req/s`
- 错误率：`0%`
- `avg`：`8.01ms`
- `p95`：`15.53ms`

第二档：

- 场景：`20 VUs`，`20s`，`sleep=0.2s`
- 吞吐：`95.47 req/s`
- 错误率：`0%`
- `avg`：`7.55ms`
- `p95`：`16.87ms`

结论：

- 热点详情在接近 `100 req/s` 时仍然稳定
- 当前阶段还没有看到热点读链路的明显瓶颈

#### `POST /api/v1/orders`

- 场景：`10 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`308`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`40.49ms`
- 自定义指标 `order_create_duration p95`：`84.4ms`

结论：

- 下单写链路提升到 `10 VUs` 后仍然稳定
- RT 比第二轮 `5 VUs` 有所上升，但幅度不大

#### `POST /api/v1/payments`

- 场景：`10 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`172`
- 错误率：`0%`
- 自定义指标 `payment_create_duration avg`：`152.5ms`
- 自定义指标 `payment_create_duration p95`：`360.25ms`

结论：

- 创建支付在 `10 VUs` 下仍然成功，但 RT 已经明显高于第二轮
- 支付创建是当前第三轮最值得继续盯的写链路

### 16.3 第三轮结论

第三轮主要结论：

1. 商品列表和热点详情继续升压到 `~95 req/s` 量级时仍然稳定
2. 当前项目的读链路表现明显强于写链路
3. 下单在 `10 VUs` 下仍然平稳
4. 支付创建在 `10 VUs` 下虽然没有错误，但 RT 增长已经比较明显
5. 当前最值得继续追踪的不是商品读，而是支付创建链路

### 16.4 第四轮建议

如果继续第四轮，我建议优先这样跑：

1. `create payment`：`15 VUs`
2. `create payment`：`20 VUs`
3. `create order`：`15 VUs`
4. `create order`：`20 VUs`
5. 同时抓 `payment-service` / `order-service` / `inventory-service` 的 metrics

重点关注：

- `payment_create_duration`
- `order_create_duration`
- RPC timeout
- MySQL 锁等待
- `gateway` 到 `payment-service`、`order-service` 的 RT 抬升

## 17. 第四轮结果

时间：

- `2026-03-24`

第四轮目标：

- 继续提升写链路强度，重点观察 `create payment` 和 `create order`
- 判断第三轮出现的 RT 抬升是偶发还是趋势

### 17.1 第四轮结果文件

- [round4-payment-v15.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round4-payment-v15.json)
- [round4-payment-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round4-payment-v20.json)
- [round4-order-v15.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round4-order-v15.json)
- [round4-order-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round4-order-v20.json)

### 17.2 第四轮关键结果

#### `POST /api/v1/payments`

第一档：

- 场景：`15 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`401`
- 错误率：`0%`
- 自定义指标 `payment_create_duration avg`：`41.24ms`
- 自定义指标 `payment_create_duration p95`：`104.43ms`

第二档：

- 场景：`20 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`493`
- 错误率：`0%`
- 自定义指标 `payment_create_duration avg`：`60.16ms`
- 自定义指标 `payment_create_duration p95`：`148.37ms`

结论：

- 创建支付在第四轮仍然没有业务错误
- RT 随并发继续上升，但增长还比较平滑
- 当前支付链路是“已明显抬升，但尚未进入失败区”

#### `POST /api/v1/orders`

第一档：

- 场景：`15 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`452`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`42.48ms`
- 自定义指标 `order_create_duration p95`：`84.44ms`

第二档：

- 场景：`20 VUs`，`20s`，`sleep=0.5s`
- 迭代数：`476`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`165.42ms`
- 自定义指标 `order_create_duration p95`：`692.82ms`

结论：

- `15 VUs` 时下单仍然比较稳
- 到 `20 VUs` 时，下单 RT 出现了明显台阶式抬升
- 虽然还没报错，但 `order_create_duration p95` 已接近 `700ms`，需要重点关注

### 17.3 第四轮结论

第四轮主要结论：

1. 支付创建链路继续升压时 RT 持续上升，但目前仍稳定且没有错误
2. 下单链路在 `20 VUs` 时出现更明显的 RT 抬升
3. 当前最值得优先排查的已不只是支付，而是订单创建链路在更高并发下的放大
4. 到第四轮为止，系统还没有出现业务错误，但写链路已经明显进入敏感区

### 17.4 下一步建议

如果继续第五轮，建议不要再只盲目升压，而是开始带观测一起做：

1. 继续压 `create order 20 VUs`，同时抓 `order-service` / `inventory-service` / `product-service` 指标
2. 继续压 `create payment 20 VUs`，同时抓 `payment-service` / `order-service` 指标
3. 检查 MySQL 锁等待、慢查询、事务耗时
4. 对比 `gateway`、RPC、DB 三层 RT，确认抬升主要发生在哪一层

如果现在要继续，我建议第五轮优先做“带指标观察的订单写链路复测”，而不是先把 `VUs` 再翻倍。
