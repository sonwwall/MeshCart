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

## 18. 第五轮结果

时间：

- `2026-03-24`

第五轮目标：

- 不再盲目升压
- 固定在敏感点 `create order 20 VUs`
- 同时采集 `gateway`、`order-service`、`inventory-service`、`product-service` 指标

### 18.1 第五轮结果文件

- [round5-order-v20-metrics.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round5-order-v20-metrics.json)

### 18.2 压测结果

场景：

- `POST /api/v1/orders`
- `20 VUs`
- `20s`
- `sleep=0.5s`

结果：

- 迭代数：`289`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`571.86ms`
- 自定义指标 `order_create_duration p95`：`1.03s`
- HTTP `avg`：`461.93ms`
- HTTP `p95`：`940.19ms`

结论：

- 第五轮复测再次确认：订单写链路在 `20 VUs` 下 RT 明显放大
- 这不是第四轮偶发抖动，而是可复现趋势

### 18.3 指标采样结果

第五轮压测前后，对关键指标做了快照对比。

#### Gateway

压测前：

- `POST /api/v1/orders count`：`2611`
- `POST /api/v1/orders sum`：`252741081 us`

压测后：

- `POST /api/v1/orders count`：`2900`
- `POST /api/v1/orders sum`：`417154227 us`

增量：

- 请求数增量：`289`
- 耗时总和增量：`164413146 us`
- 该轮 gateway 侧平均单请求耗时约：`568.9ms`

#### Order Service

压测前：

- `create_order count`：`2664`
- `create_order sum`：`250.602931333 s`

压测后：

- `create_order count`：`2953`
- `create_order sum`：`413.551995584 s`

增量：

- 请求数增量：`289`
- 耗时总和增量：`162.949064251 s`
- 该轮 order-service 平均单请求耗时约：`563.8ms`

#### Inventory Service

压测前：

- `reserve_sku_stocks count`：`2664`
- `reserve_sku_stocks sum`：`82.396639821 s`

压测后：

- `reserve_sku_stocks count`：`2953`
- `reserve_sku_stocks sum`：`155.267083959 s`

增量：

- 请求数增量：`289`
- 耗时总和增量：`72.870444138 s`
- 该轮 inventory-service 平均单请求耗时约：`252.1ms`

#### Product Service

`batch_get_sku`：

- 增量请求数：`289`
- 增量耗时总和：`8.030671334 s`
- 平均单请求耗时约：`27.8ms`

`get_product_detail`：

- 增量请求数：`289`
- 增量耗时总和：`11.745840091 s`
- 平均单请求耗时约：`40.6ms`

### 18.4 第五轮结论

第五轮最重要的结论：

1. 订单链路在 `20 VUs` 下的 RT 放大是稳定复现的
2. `gateway` 与 `order-service` 的平均耗时增量几乎同步，说明抬升主要不是 gateway 自身逻辑，而是下游订单编排链路
3. 在订单编排链路中，`inventory-service reserve_sku_stocks` 占用了相当可观的耗时
4. `product-service` 的 `batch_get_sku` 和 `get_product_detail` 也有贡献，但量级明显低于库存预占
5. 当前最值得优先排查的是：
   - `order-service -> inventory-service reserve_sku_stocks`
   - 订单创建过程中的库存预占和相关数据库操作

### 18.5 下一步建议

如果继续第六轮，我建议改成“定位型压测”，而不是继续简单升 `VUs`：

1. 复测 `create order 20 VUs`
2. 同时抓：
   - `inventory-service` 更细粒度指标
   - MySQL 锁等待
   - MySQL 慢查询
   - `order-service` / `inventory-service` 日志
3. 如果有条件，再做热点 SKU 与非热点 SKU 对比

当前阶段，最有价值的工作已经从“继续抬压”切换到“确认库存预占与订单编排的耗时来源”。

如果现在要继续，我建议第五轮优先做“带指标观察的订单写链路复测”，而不是先把 `VUs` 再翻倍。

## 19. 第六轮结果

时间：

- `2026-03-24`

第六轮目标：

- 做“定位型压测”
- 对比热点 SKU 和普通 SKU 在相同 `20 VUs` 下的订单创建表现
- 判断第五轮的 RT 放大更像是热点竞争，还是更广义的订单链路波动

### 19.1 第六轮场景说明

本轮围绕 `POST /api/v1/orders` 做两组对照：

1. 热点 SKU：固定打热点商品对应的 `sku_id`
2. 普通 SKU：固定打普通商品对应的 `sku_id`

压测参数保持一致：

- `20 VUs`
- `20s`
- `sleep=0.5s`

### 19.2 第六轮结果文件

- [round6-order-hot-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round6-order-hot-v20.json)
- [round6-order-normal-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round6-order-normal-v20.json)
- [round6-order-normal-v20-restocked.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round6-order-normal-v20-restocked.json)

说明：

- [round6-order-normal-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round6-order-normal-v20.json) 是一次失效样本
- 这次普通 SKU 对照跑到中途时库存被耗尽，出现了大量业务失败，不能拿来做性能结论
- 为了保证对照有效，已把普通 SKU 总库存调高到 `5000`，并重新执行了 [round6-order-normal-v20-restocked.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round6-order-normal-v20-restocked.json)

### 19.3 热点 SKU 结果

场景：

- `POST /api/v1/orders`
- 固定热点 `sku_id`
- `20 VUs`
- `20s`

结果：

- 迭代数：`510`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`88.52ms`
- 自定义指标 `order_create_duration p95`：`175.74ms`
- HTTP `p95`：`302.77ms`

### 19.4 普通 SKU 失效样本

场景：

- `POST /api/v1/orders`
- 固定普通 `sku_id`
- `20 VUs`
- `20s`

失效样本结果：

- 迭代数：`608`
- 自定义指标 `order_create_failed`：`67.10%`
- 自定义指标 `order_create_duration p95`：`219.34ms`

失效原因：

- 普通 SKU 初始库存只有 `200`
- 本轮压测请求数明显超过库存承载能力
- 大量失败是库存耗尽导致的业务拒绝，不是性能瓶颈

因此：

- 这组数据仅用于说明“库存因素会污染对照组”
- 不纳入热点 / 非热点性能结论

### 19.5 普通 SKU 补库存后的有效结果

修正动作：

- 通过管理员接口将普通 SKU `2036361111363657729` 的总库存调整到 `5000`

场景：

- `POST /api/v1/orders`
- 固定普通 `sku_id`
- `20 VUs`
- `20s`

结果：

- 迭代数：`514`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`85.41ms`
- 自定义指标 `order_create_duration p95`：`183.97ms`
- HTTP `p95`：`323.59ms`

### 19.6 指标采样对比

为了对比热点和普通 SKU，本轮继续对关键 RPC 指标做了前后快照。

#### 热点 SKU

`order-service create_order`：

- 增量请求数：`510`
- 平均单请求耗时约：`85.16ms`

`inventory-service reserve_sku_stocks`：

- 增量请求数：`510`
- 平均单请求耗时约：`22.43ms`

#### 普通 SKU

`order-service create_order`：

- 增量请求数：`514`
- 平均单请求耗时约：`82.44ms`

`inventory-service reserve_sku_stocks`：

- 增量请求数：`514`
- 平均单请求耗时约：`23.50ms`

`product-service batch_get_sku`：

- 增量请求数：`514`
- 平均单请求耗时约：`4.82ms`

`product-service get_product_detail`：

- 增量请求数：`514`
- 平均单请求耗时约：`6.93ms`

### 19.7 第六轮结论

第六轮最重要的结论：

1. 第五轮里 `20 VUs` 订单 RT 抬升到 `p95 1s` 以上的现象，这一轮没有稳定复现
2. 在同样的 `20 VUs` 下，热点 SKU 和补库存后的普通 SKU 表现非常接近
3. 目前没有证据表明“热点 SKU 竞争”本身就是订单链路 RT 异常放大的主要原因
4. 上一轮的极高 RT 更像是阶段性波动、环境抖动，或者库存/状态因素叠加，而不是稳定的热点争用瓶颈
5. `inventory-service reserve_sku_stocks` 仍然是订单链路里需要持续观察的一段，但在本轮对照里没有出现热点显著劣化

### 19.8 下一步建议

如果继续第七轮，我建议不要立刻继续加 `VUs`，而是做下面两类验证：

1. 对 `create order 20 VUs` 做多次重复复测，确认第五轮的高 RT 是偶发还是周期性抖动
2. 开始补数据库层观测：
   - MySQL 锁等待
   - MySQL 慢查询
   - 订单表和库存表相关 SQL 耗时

当前阶段，更像是需要“稳定性复测 + 底层观测”，而不是继续单纯提高并发。

## 20. 第七轮结果

时间：

- `2026-03-24`

第七轮目标：

- 在相同压测参数下重复执行 `POST /api/v1/orders`
- 判断第五轮的高 RT 是否能稳定复现
- 识别“性能波动”和“测试数据耗尽”这两类不同问题

### 20.1 第七轮场景

固定场景：

- `POST /api/v1/orders`
- 热点 `sku_id`
- `20 VUs`
- `20s`
- `sleep=0.5s`

### 20.2 第七轮结果文件

- [round7-order-v20-run1.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round7-order-v20-run1.json)
- [round7-order-v20-run2-retry.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round7-order-v20-run2-retry.json)
- [round7-order-v20-run3.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round7-order-v20-run3.json)
- [round7-order-v20-run3-restocked.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round7-order-v20-run3-restocked.json)

说明：

- 第二次复测第一次尝试受到本地沙箱网络限制污染，没有纳入结果
- [round7-order-v20-run3.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round7-order-v20-run3.json) 是一组“库存耗尽样本”
- 为了得到干净的第三组数据，已补库存后重新执行 [round7-order-v20-run3-restocked.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round7-order-v20-run3-restocked.json)

### 20.3 有效复测结果

#### 第 1 次

- 迭代数：`499`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`106.84ms`
- 自定义指标 `order_create_duration p95`：`329.14ms`
- HTTP `p95`：`399.88ms`

#### 第 2 次

- 迭代数：`544`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`68.19ms`
- 自定义指标 `order_create_duration p95`：`142.64ms`
- HTTP `p95`：`256.88ms`

#### 第 3 次（补库存后）

- 迭代数：`552`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`63.87ms`
- 自定义指标 `order_create_duration p95`：`137.82ms`
- HTTP `p95`：`244.46ms`

### 20.4 失效样本

`[round7-order-v20-run3.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round7-order-v20-run3.json)` 的现象：

- 迭代数：`563`
- 自定义指标 `order_create_failed`：`12.25%`
- 自定义指标 `order_create_duration p95`：`184.07ms`

这组样本起初在 `k6` 侧表现为“business code is 0 失败”以及“response missing order_id”，但继续排查后确认不是接口格式问题，而是库存用尽。

证据：

1. `order-service` 暴露了业务错误码 `2040005`
2. `inventory-service` 暴露了业务错误码 `2050002`
3. 错误码定义分别对应“库存不足”
4. 管理员接口查询热点 SKU 现状时，返回：
   - `total_stock=5000`
   - `reserved_stock=5000`
   - `saleable_stock=0`

因此：

- 这组失败不是新的逻辑异常
- 而是连续多轮压测后，热点 SKU 被逐步打空

### 20.5 第七轮结论

第七轮最重要的结论：

1. 在补足库存的前提下，`create order 20 VUs` 的高 RT 没有稳定复现
2. 三组有效样本的 `order_create_duration p95` 分别是：
   - `329.14ms`
   - `142.64ms`
   - `137.82ms`
3. 这说明第五轮 `p95 1.03s` 更像是阶段性波动，而不是当前环境下稳定可复现的瓶颈
4. 连续压测时，测试数据本身会成为强干扰因素，尤其是热点 SKU 库存
5. 当前最需要控制的是：
   - 每轮前确认热点 SKU 可售库存
   - 区分“性能退化”和“库存耗尽”两类现象

### 20.6 当前限制

本轮原本计划补数据库层观测，但当前环境里没有可直接调用的 `mysql` 客户端，因此未能抓到：

- MySQL 锁等待
- MySQL 慢查询
- InnoDB 行锁细节

目前仍然可以依赖：

- `k6` 摘要
- `gateway` metrics
- `order-service` metrics
- `inventory-service` metrics

### 20.7 下一步建议

如果继续第八轮，我建议优先做两件事：

1. 给压测脚本增加“压测前库存检查”或“自动补库存”步骤，避免样本再次被库存耗尽污染
2. 开始引入数据库层观测工具，再做一次 `create order 20 VUs` 复测

当前阶段，系统表现更像是“整体可用但对测试数据状态敏感”，而不是已经稳定暴露出单一性能瓶颈。

## 21. 第八轮结果

时间：

- `2026-03-24`

第八轮目标：

- 做一轮“控制变量复测”
- 在固定库存前提下比较热点 SKU 与普通 SKU 的订单创建表现
- 再次确认 `create order 20 VUs` 是否存在稳定高延迟

### 21.1 第八轮准备动作

本轮开始前，先统一补库存，避免样本再次被库存耗尽污染：

- 热点 SKU `2036361111032307713`
  - 调整后：`total_stock=10000`
  - 当时库存状态：`reserved_stock=5552`，`saleable_stock=4448`

- 普通 SKU `2036361111363657729`
  - 调整后：`total_stock=10000`
  - 当时库存状态：`reserved_stock=714`，`saleable_stock=9286`

说明：

- 当前环境里的“补库存”是调高 `total_stock`
- 历史预占记录不会自动清空，因此准备动作的重点是保证 `saleable_stock` 足够支撑本轮样本

### 21.2 第八轮场景

统一参数：

- `POST /api/v1/orders`
- `20 VUs`
- `20s`
- `sleep=0.5s`

执行顺序：

1. 热点 SKU，第 1 次
2. 热点 SKU，第 2 次
3. 普通 SKU，对照组

### 21.3 第八轮结果文件

- [round8-order-hot-v20-run1.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round8-order-hot-v20-run1.json)
- [round8-order-hot-v20-run2.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round8-order-hot-v20-run2.json)
- [round8-order-normal-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round8-order-normal-v20.json)

### 21.4 热点 SKU，第 1 次

- 迭代数：`548`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`69.42ms`
- 自定义指标 `order_create_duration p95`：`154.02ms`
- HTTP `p95`：`236.39ms`

### 21.5 热点 SKU，第 2 次

- 迭代数：`561`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`59.05ms`
- 自定义指标 `order_create_duration p95`：`128.94ms`
- HTTP `p95`：`239.76ms`

### 21.6 普通 SKU，对照组

- 迭代数：`550`
- 错误率：`0%`
- 自定义指标 `order_create_duration avg`：`71.39ms`
- 自定义指标 `order_create_duration p95`：`190.29ms`
- HTTP `p95`：`260.67ms`

### 21.7 第八轮结论

第八轮最重要的结论：

1. 在统一补库存之后，三组样本全部稳定通过，没有出现业务失败
2. `create order 20 VUs` 在本轮没有复现第五轮的高 RT
3. 两次热点 SKU 结果分别为：
   - `p95=154.02ms`
   - `p95=128.94ms`
4. 普通 SKU 对照组结果为：
   - `p95=190.29ms`
5. 本轮没有出现“热点 SKU 明显慢于普通 SKU”的现象，反而热点组略快于普通组
6. 这进一步说明：
   - 第五轮 `p95 1.03s` 不是当前环境下稳定可复现的常态
   - 热点争用目前仍然不是已被证明的主瓶颈
   - 库存状态控制对压测结论影响非常大

### 21.8 当前判断更新

经过第八轮之后，可以进一步收敛为：

1. `20 VUs` 下的订单创建链路整体是可用且相对稳定的
2. 订单链路依然是当前最敏感的写路径，但还没有被证明在该强度下存在稳定的性能崩点
3. 当前更像是：
   - 订单链路对环境波动敏感
   - 压测结果对库存状态敏感
4. 如果后续要继续找瓶颈，重点不应该只是继续堆 `VUs`，而应该补：
   - 数据库层证据
   - 更严格的压测前数据重置

## 22. 第九轮结果

时间：

- `2026-03-24`

第九轮目标：

- 提升到更高强度，逼近系统失稳点
- 找出首先出现业务错误或明显 RT 放大的链路
- 观察是网关、登录、订单、库存还是支付服务先进入异常区

### 22.1 第九轮准备动作

本轮开始前，先把压测 SKU 库存抬高，避免样本过早耗尽：

- 热点 SKU `2036361111032307713`
  - `total_stock=50000`
  - `reserved_stock=6661`
  - `saleable_stock=43339`

- 普通 SKU `2036361111363657729`
  - `total_stock=30000`
  - `reserved_stock=1264`
  - `saleable_stock=28736`

### 22.2 第九轮结果文件

- [round9-products-list-v50.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round9-products-list-v50.json)
- [round9-product-detail-v50.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round9-product-detail-v50.json)
- [round9-login-rate30.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round9-login-rate30.json)
- [round9-order-v50.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round9-order-v50.json)
- [round9-payment-v20.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round9-payment-v20.json)

### 22.3 读链路极限压测

#### `GET /api/v1/products`

场景：

- `50 VUs`
- `20s`
- `sleep=0.1s`

结果：

- 迭代数：`17095`
- 吞吐：约 `842.56 req/s`
- HTTP `avg`：`22.98ms`
- HTTP `p95`：`104.89ms`
- HTTP 失败率：`0%`
- checks 失败：`11099 / 51285`

现象：

- 大量请求仍然返回 `200`
- 但业务校验失败，占比约 `21.64%`
- 表现为“response body 是 JSON，但 products 数据缺失”

判断：

- 读链路在 `50 VUs` 下已出现明显业务拒绝
- 失稳形式不是 HTTP 层失败，而是业务层先返回非成功结果

#### `GET /api/v1/products/detail/:product_id`

场景：

- `50 VUs`
- `20s`
- `sleep=0.1s`

结果：

- 迭代数：`9742`
- 吞吐：约 `474.55 req/s`
- HTTP `avg`：`73.80ms`
- HTTP `p95`：`192.76ms`
- HTTP 失败率：`0%`
- checks 失败：`6733 / 29226`

现象：

- 同样出现大量 `200` 响应但业务失败
- 表现为“product detail response missing product”
- 业务失败占比约 `23.03%`

判断：

- 商品详情链路在第九轮强度下也进入业务级失稳区
- 商品详情比商品列表更早出现更高 RT，但两者都还没表现成 TCP/HTTP 层崩溃

### 22.4 登录链路极限压测

#### `POST /api/v1/user/login`

场景：

- `30 req/s`
- `20s`
- `preAllocatedVUs=30`
- `maxVUs=60`

结果：

- 完成请求数：`567`
- 丢弃迭代：`34`
- HTTP `avg`：`1396.62ms`
- HTTP `p95`：`2009.84ms`
- HTTP 失败率：`0%`
- checks 失败：`345 / 1701`

现象：

- 业务失败占比约 `20.28%`
- 开始出现 dropped iterations
- 虽然 HTTP 层仍为 `200`，但大量登录响应不再返回 token

判断：

- 登录链路在 `30 req/s` 已明显到达压力拐点
- 当前系统入口的首个清晰瓶颈之一就是登录链路

### 22.5 订单链路极限压测

#### `POST /api/v1/orders`

场景：

- `50 VUs`
- `20s`
- `sleep=0.2s`

结果：

- 迭代数：`3603`
- HTTP 请求数：`4243`
- 自定义指标 `order_create_duration avg`：`804.25ms`
- 自定义指标 `order_create_duration p95`：`1352.64ms`
- HTTP `avg`：`209.82ms`
- HTTP `p95`：`1035.99ms`
- 自定义指标 `order_create_failed`：`1.25%`
- checks 失败：`2971 / 12729`

现象：

- 业务失败占比约 `23.34%`
- 出现两类失败：
  - `create order response missing order_id`
  - `login failed`

服务侧指标：

- `order-service create_order`
  - 成功：`9154`
  - 错误码 `2040005`：`476`
  - 错误码 `2040006`：`1`
  - 错误码 `1009999`：`1`

- `inventory-service reserve_sku_stocks`
  - 成功：`9154`
  - 错误码 `2050002`：`476`
  - 错误码 `2050006`：`1`
  - 错误码 `1009999`：`1`

判断：

- 订单链路在 `50 VUs` 已明显进入高延迟区
- 服务侧错误码显示，订单失败主要映射到了库存不足链路：
  - `2040005` = 订单侧库存不足
  - `2050002` = 库存侧库存不足
- 这说明极限压测下，订单链路的首个明确出错点是库存预占相关业务路径

### 22.6 支付链路极限压测

说明：

- 当前支付压测脚本会串行执行：
  1. 登录
  2. 创建订单
  3. 创建支付单
- 因此它本质上是一个短交易链路压测，不只是单独压 `payment-service`

#### `POST /api/v1/payments`

场景：

- `20 VUs`
- `20s`
- `sleep=0.2s`

结果：

- 迭代数：`19276`
- HTTP 请求数：`20507`
- 自定义指标 `payment_create_duration avg`：`46.64ms`
- 自定义指标 `payment_create_duration p95`：`92.94ms`
- HTTP `avg`：`14.09ms`
- HTTP `p95`：`73.44ms`
- 自定义指标 `payment_create_failed`：`5.37%`
- checks 失败：`18712 / 61521`

现象：

- checks 失败占比约 `30.41%`
- 主要失败表现仍然是：
  - 登录失败
  - 上游订单创建阶段失败

服务侧指标：

- `payment-service create_payment`
  - 成功：`1805`
  - 当前未观察到业务错误码累计增长

判断：

- 在第九轮强度下，支付服务本身还没有先成为首个失稳点
- 支付链路的失败主要发生在登录和订单前置阶段
- 也就是说，当前“更复杂链路先崩”的责任主要仍在入口和订单前置写链路，而不是支付服务自身

### 22.7 Gateway 指标补充

第九轮结束后，gateway 指标累计如下：

- `POST /api/v1/user/login`
  - count：`31999`
  - sum：`3003494980 us`
  - 平均约：`93.86ms`

- `GET /api/v1/products`
  - count：`20143`
  - sum：`401268564 us`
  - 平均约：`19.92ms`

- `GET /api/v1/products/detail/:product_id`
  - count：`14071`
  - sum：`738531070 us`
  - 平均约：`52.48ms`

- `POST /api/v1/orders`
  - count：`9624`
  - sum：`1410294992 us`
  - 平均约：`146.54ms`

- `POST /api/v1/payments`
  - count：`1811`
  - sum：`104421962 us`
  - 平均约：`57.66ms`

说明：

- gateway 侧累计平均耗时依然显示订单写链路明显重于读链路和支付接口
- 登录累计成本也明显高于读链路

### 22.8 第九轮结论

第九轮最重要的结论：

1. 在更高强度下，首先明显失稳的不是 HTTP 层，而是业务层
2. 读链路在 `50 VUs` 就出现约 `20%+` 的业务失败
3. 登录链路在 `30 req/s` 已出现：
   - `p95` 接近 `2s`
   - dropped iterations
   - 约 `20%` 的业务失败
4. 订单链路在 `50 VUs` 已进入明显高延迟区：
   - `order_create_duration p95=1352.64ms`
   - 业务失败约 `23%`
5. 服务侧最明确的错误归因是：
   - `order-service 2040005`
   - `inventory-service 2050002`
   - 即库存不足相关业务失败
6. 支付服务本身在本轮没有先成为首个失稳点
7. 当前系统在高强度压测下，最先暴露的薄弱点是：
   - 入口登录链路
   - 订单创建前置链路
   - 库存预占相关业务路径

### 22.9 本轮停止原因

第九轮没有继续把并发继续翻到更高档位，原因是当前结果已经足够明确：

1. 读链路在 `50 VUs` 已经进入业务级失稳区
2. 登录在 `30 req/s` 已经明显掉队
3. 订单在 `50 VUs` 已经进入高延迟区
4. 支付链路失败主要是被前置链路拖累，不需要再继续单纯提高并发才能得出结论

继续粗暴升压的收益已经不高，下一步更有价值的是：

1. 复核网关业务拒绝来源
2. 补数据库层观测
3. 进一步区分：
   - 限流导致的业务失败
   - 真实服务处理不过来的业务失败

## 23. 第十轮结果

时间：

- `2026-03-25`

第十轮目标：

- 排除用户接口干扰，聚焦电商核心交易链路
- 在关闭网关限流干扰后，确认真实瓶颈位于哪一层
- 基于结果判断哪里适合加缓存、哪里适合引入消息队列、哪里需要放大连接池

### 23.1 第十轮开始前的调整

为了避免继续把网关限流误判为系统容量上限，本轮压测前关闭了 `gateway` 限流开关后再启动服务。

说明：

- 这只是压测期配置
- 目的不是放松生产护栏，而是排除限流策略干扰，观察真实处理能力

本轮新增脚本：

- [round10_order_core.js](/Users/ruitong/GolandProjects/MeshCart/loadtest/k6/round10_order_core.js)
- [round10_checkout_core.js](/Users/ruitong/GolandProjects/MeshCart/loadtest/k6/round10_checkout_core.js)

本轮补充数据准备：

- 热点 SKU `2036361111032307713` 补库存到 `50000`
- 普通 SKU 池每个补库存到 `10000`

### 23.2 第十轮结果文件

- [round10-order-core-smoke.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round10-order-core-smoke.json)
- [round10-checkout-core-smoke.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round10-checkout-core-smoke.json)
- [round10-order-hot-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round10-order-hot-v10.json)
- [round10-order-normal-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round10-order-normal-v10.json)
- [round10-order-hot-v40.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round10-order-hot-v40.json)
- [round10-order-normal-v40.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round10-order-normal-v40.json)
- [round10-checkout-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round10-checkout-v10.json)

### 23.3 冒烟结果

#### `POST /api/v1/orders`

- 场景：纯下单冒烟
- 结果：业务失败率 `0%`
- `order_create_duration p95`：约 `29.61ms`

#### `创建订单 -> 创建支付 -> 模拟支付确认`

- 场景：核心结算链路冒烟
- 结果：业务失败率 `0%`
- `checkout_duration p95`：约 `86.69ms`
- `checkout_order_create_duration avg`：约 `20.76ms`
- `checkout_payment_create_duration avg`：约 `12.05ms`
- `checkout_payment_confirm_duration avg`：约 `26.02ms`

结论：

- 第十轮脚本和测试数据在小流量下是正确的
- 本轮正式压测中的大量失败，不是脚本错误或基础数据错误导致

### 23.4 第十轮关键结果

#### `POST /api/v1/orders` 热点 SKU

`10 VUs`：

- 总请求数：`16839`
- 吞吐：`830.20 req/s`
- 业务失败率：`98.16%`
- `order_create_duration avg`：`10.13ms`
- `order_create_duration p95`：`11.30ms`
- 最大耗时：`1092.45ms`

`40 VUs`：

- 总请求数：`45884`
- 吞吐：`2236.67 req/s`
- 业务失败率：`99.43%`
- `order_create_duration avg`：`16.72ms`
- `order_create_duration p95`：`5.13ms`
- 最大耗时：`2036.91ms`

#### `POST /api/v1/orders` 普通 SKU 池

`10 VUs`：

- 总请求数：`26166`
- 吞吐：`1286.78 req/s`
- 业务失败率：`98.41%`
- `order_create_duration avg`：`6.13ms`
- `order_create_duration p95`：`9.78ms`
- 最大耗时：`834.76ms`

`40 VUs`：

- 总请求数：`64058`
- 吞吐：`3126.81 req/s`
- 业务失败率：`99.09%`
- `order_create_duration avg`：`11.45ms`
- `order_create_duration p95`：`5.55ms`
- 最大耗时：`2025.78ms`

#### `创建订单 -> 创建支付 -> 模拟支付确认`

`10 VUs`：

- 总请求数：`16615`
- 吞吐：`792.12 req/s`
- 业务失败率：`99.74%`
- `checkout_duration avg`：`559.33ms`
- `checkout_duration p95`：`1404ms`
- `checkout_order_create_duration avg`：`11.03ms`
- `checkout_payment_create_duration avg`：`18.95ms`
- `checkout_payment_confirm_duration avg`：`117.34ms`

### 23.5 第十轮现象

本轮最关键的现象不是“热点 SKU 比普通 SKU 明显更差”，而是：

- 热点和普通商品两组下单场景都在很低并发下出现接近 `100%` 的业务失败
- 大部分失败是快速返回，只有一小部分请求拖长到秒级
- checkout 失败主要由前置下单失败传导而来，不是支付服务单独先崩

同时，`gateway` 日志中大量出现：

- `order rpc create returned business error`
- `rpc timeout: timeout=2s, to=meshcart.order, method=createOrder`

这说明：

- 第十轮的第一失稳点已经不是网关限流
- 真实瓶颈已经前移到 `gateway -> order-service` 这一段同步下单路径

### 23.6 第十轮结论

第十轮最重要的结论：

1. 当前系统的首要瓶颈已经明确落在订单创建主链路，而不是用户接口、库存热点或支付服务
2. 热点 SKU 和普通 SKU 的失败率差异不大，说明系统在进入“库存热点竞争”之前，就先被订单主链路打满了
3. checkout 场景的高失败率主要继承自订单创建失败，不是支付服务本身先成为首个失稳点
4. 大量请求快速失败，说明当前系统里存在明显的同步拒绝或下游不可用返回；少量请求拖到秒级，则说明已经伴随真实 RPC 超时

换句话说，第十轮确认了：

- 当前不是“库存先扛不住”
- 也不是“支付先扛不住”
- 而是“订单前置编排和其依赖访问成本太高，导致订单服务先成为系统闸口”

### 23.7 本轮对缓存、消息队列、连接池的判断

#### 更适合优先加缓存的位置

第一优先级是订单创建前置依赖里的商品和 SKU 数据。

原因：

- 下单链路里商品和 SKU 信息是高频重复读取的数据
- 这些数据变化频率相对低，但读取频率远高于修改频率
- 如果每次下单都同步查询商品信息，再逐商品补详情，会把订单服务和商品服务一起拖慢

建议优先考虑：

1. 商品快照缓存
2. SKU 基础信息缓存
3. 批量商品 / SKU 查询结果缓存

目标：

- 减少 `order-service -> product-service` 的同步访问次数
- 把订单主链路从“多跳串行查数”收敛成“少量必要校验”

#### 更适合引入消息队列的位置

消息队列不是解决第十轮首要瓶颈的第一手段，但适合承接交易成功后的后置动作。

更适合异步化的位置：

1. 支付成功后的订单状态推进通知
2. 库存确认扣减后的后续事件
3. 营销、消息、积分、审计日志等非主交易关键步骤

不建议把当前下单主路径里最核心的商品校验、库存预占、订单落库简单甩到 MQ 后面，否则会引入更复杂的一致性问题。

#### 更应该先调大的连接池

当前多个服务的 MySQL 连接池默认都只有 `20` 个连接：

- [services/order-service/dal/db/db.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/dal/db/db.go#L21)
- [services/product-service/dal/db/db.go](/Users/ruitong/GolandProjects/MeshCart/services/product-service/dal/db/db.go#L21)
- [services/inventory-service/dal/db/db.go](/Users/ruitong/GolandProjects/MeshCart/services/inventory-service/dal/db/db.go#L21)
- [services/payment-service/dal/db/db.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/dal/db/db.go#L34)

结合第十轮现象，优先级建议如下：

1. 先调大 `order-service` 数据库连接池
2. 再调大 `product-service` 和 `inventory-service` 数据库连接池
3. 补连接池等待、慢 SQL、事务时长、锁等待指标后再继续升压

说明：

- 连接池调大只能缓解“排队等待”问题
- 如果订单链路自身同步步骤太多，单纯放大连接池不会从根本上解决问题

### 23.8 第十轮后的优化优先级

建议下一步按这个顺序推进：

1. 先瘦身订单创建主链路，减少同步 RPC 和逐商品详情查询
2. 优先给商品 / SKU 读路径加缓存，尤其是订单创建依赖的快照数据
3. 调大 `order-service`、`product-service`、`inventory-service` 的数据库连接池
4. 补齐数据库层和 RPC 层观测指标，再复跑一轮核心链路压测
5. 最后再单独做库存热点专项，判断是否需要库存分片、排队或更激进的热点治理

### 23.9 本轮停止原因

第十轮没有继续把并发继续抬更高，原因是当前结果已经足够明确：

1. 在关闭网关限流干扰后，系统仍然在核心交易链路上快速失稳
2. 失稳点已经明确落在 `order-service` 主路径
3. checkout 失败的根因主要来自订单创建前置失败
4. 继续盲目升压只会重复确认同一个结论，暂时不会提供更高价值的信息

## 24. 第十一轮结果

时间：

- `2026-03-25`

第十一轮目标：

- 在已完成一部分 P0 优化后，复跑核心订单与 checkout 场景
- 与第十轮做直接对比，判断订单主链路瘦身是否已经产生收益
- 区分“吞吐和时延改善”与“整体业务成功率仍然偏低”这两件事

### 24.1 第十一轮开始前的改动

本轮是在以下优化落地后进行复测：

1. `order-service` 下单校验不再逐个调用商品详情接口，而是改成批量商品读取
2. `product-service` 内部给 `BatchGetProducts` 和 `BatchGetSKU` 加了 Redis cache-aside
3. `order-service` 的 `CreateOrder` 增加了 `validation / reserve / persist / total` 分阶段耗时日志
4. `order-service`、`product-service`、`inventory-service` 增加了可配置数据库连接池与 DB pool metrics

说明：

- 本轮仍然保持关闭 `gateway` 限流，避免策略性拒绝继续干扰判断
- 本轮重点不是再找“是否会失败”，而是判断“第十轮的首瓶颈是否已经被削弱”

### 24.2 第十一轮结果文件

- [round11-order-core-smoke.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round11-order-core-smoke.json)
- [round11-checkout-core-smoke.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round11-checkout-core-smoke.json)
- [round11-order-hot-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round11-order-hot-v10.json)
- [round11-order-normal-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round11-order-normal-v10.json)
- [round11-order-hot-v40.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round11-order-hot-v40.json)
- [round11-order-normal-v40.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round11-order-normal-v40.json)
- [round11-checkout-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round11-checkout-v10.json)

### 24.3 冒烟结果

#### `POST /api/v1/orders`

- 场景：纯下单冒烟
- 结果：业务失败率 `0%`
- `order_create_duration avg`：约 `23.22ms`
- `order_create_duration p95`：约 `42.82ms`

#### `创建订单 -> 创建支付 -> 模拟支付确认`

- 场景：核心结算链路冒烟
- 结果：业务失败率 `0%`
- `checkout_duration avg`：约 `68.22ms`
- `checkout_duration p95`：约 `103.35ms`
- `checkout_order_create_duration avg`：约 `24.85ms`
- `checkout_payment_create_duration avg`：约 `13.35ms`
- `checkout_payment_confirm_duration avg`：约 `28.92ms`

结论：

- 第十一轮脚本与测试数据在低压下仍然可用
- 优化改动没有引入新的功能性回归

### 24.4 第十一轮关键结果

#### `POST /api/v1/orders` 热点 SKU

`10 VUs`：

- 总请求数：`81003`
- 吞吐：`4014.19 req/s`
- 业务失败率：`98.64%`
- 成功请求数：`1099`
- `order_create_duration avg`：`1.51ms`
- `order_create_duration p95`：`1.32ms`
- 最大耗时：`191.44ms`

`40 VUs`：

- 总请求数：`90985`
- 吞吐：`4353.73 req/s`
- 业务失败率：`98.85%`
- 成功请求数：`1044`
- `order_create_duration avg`：`7.59ms`
- `order_create_duration p95`：`7.37ms`
- 最大耗时：`1730ms`

#### `POST /api/v1/orders` 普通 SKU 池

`10 VUs`：

- 总请求数：`25749`
- 吞吐：`1270.96 req/s`
- 业务失败率：`95.73%`
- 成功请求数：`1100`
- `order_create_duration avg`：`5.26ms`
- `order_create_duration p95`：`10.01ms`
- 最大耗时：`433.3ms`

`40 VUs`：

- 总请求数：`138673`
- 吞吐：`6847.73 req/s`
- 业务失败率：`99.21%`
- 成功请求数：`1099`
- `order_create_duration avg`：`4.53ms`
- `order_create_duration p95`：`6.13ms`
- 最大耗时：`513.15ms`

#### `创建订单 -> 创建支付 -> 模拟支付确认`

`10 VUs`：

- 总请求数：`89193`
- 吞吐：`4320.95 req/s`
- 业务失败率：`99.92%`
- 成功请求数：`71`
- `checkout_duration avg`：`301.46ms`
- `checkout_duration p95`：`703.05ms`
- `checkout_order_create_duration avg`：`1.78ms`
- `checkout_payment_create_duration avg`：`7.22ms`
- `checkout_payment_confirm_duration avg`：`91.47ms`

### 24.5 第十一轮与第十轮对比

#### 订单热点场景

`order-hot-v10` 对比第十轮：

- 吞吐：`830.20 -> 4014.19 req/s`
- `order_create_duration p95`：`11.30ms -> 1.32ms`
- `order_create_duration avg`：`10.13ms -> 1.51ms`
- 成功请求数：`309 -> 1099`
- 业务失败率：`98.16% -> 98.64%`

`order-hot-v40` 对比第十轮：

- 吞吐：`2236.67 -> 4353.73 req/s`
- `order_create_duration avg`：`16.72ms -> 7.59ms`
- `order_create_duration p95`：`5.13ms -> 7.37ms`
- 成功请求数：`263 -> 1044`
- 业务失败率：`99.43% -> 98.85%`

#### 订单普通场景

`order-normal-v10` 对比第十轮：

- 吞吐：`1286.78 -> 1270.96 req/s`
- `order_create_duration avg`：`6.13ms -> 5.26ms`
- `order_create_duration p95`：`9.78ms -> 10.01ms`
- 成功请求数：`415 -> 1100`
- 业务失败率：`98.41% -> 95.73%`

`order-normal-v40` 对比第十轮：

- 吞吐：`3126.81 -> 6847.73 req/s`
- `order_create_duration avg`：`11.45ms -> 4.53ms`
- `order_create_duration p95`：`5.55ms -> 6.13ms`
- 成功请求数：`580 -> 1099`
- 业务失败率：`99.09% -> 99.21%`

#### checkout 场景

`checkout-v10` 对比第十轮：

- 吞吐：`792.12 -> 4320.95 req/s`
- `checkout_duration avg`：`559.33ms -> 301.46ms`
- `checkout_duration p95`：`1404ms -> 703.05ms`
- `checkout_order_create_duration p95`：`11.21ms -> 1.72ms`
- `checkout_payment_create_duration p95`：`101.26ms -> 54.47ms`
- `checkout_payment_confirm_duration p95`：`362.7ms -> 297.17ms`
- 成功请求数：`43 -> 71`
- 业务失败率：`99.74% -> 99.92%`

### 24.6 第十一轮现象

本轮最重要的现象，不是“系统已经稳定”，而是“第十轮识别出的第一瓶颈被明显削弱了”。

从结果看：

1. 订单热点场景和 checkout 场景的吞吐都有明显提升
2. 下单阶段时延显著下降，尤其是热点 `v10` 和 checkout 里的下单子阶段
3. 成功请求数在所有核心场景里都比第十轮更多
4. 但大多数正式场景的业务失败率依然维持在 `95% ~ 99%`

这说明：

- 本轮对订单主链路的瘦身是有效的
- 之前 `order-service -> product-service` 那段同步读取放大效应已经被压下去一部分
- 当前系统已经能更快地走到业务判定点
- 但新的主要问题已经不是“商品读取太慢”，而是“请求更快地失败在后续业务阶段”

### 24.7 本轮对数据库连接池与观测的判断

第十一轮压测后抽查了 DB pool metrics：

- `product-service`：`meshcart_db_wait_count_total = 0`
- `inventory-service`：`meshcart_db_wait_count_total = 0`
- `order-service`：`meshcart_db_wait_count_total = 0`

当前结论是：

1. 这轮看到的收益，主要来自订单主链路瘦身和商品 / SKU 读缓存
2. 还没有观察到明显的数据库连接池等待信号
3. 连接池调优仍然是必要动作，但不是第十一轮结果改善的主要解释变量

### 24.8 第十一轮结论

第十一轮最重要的结论：

1. 第十轮识别出的首瓶颈，也就是订单创建前置读取链路，已经被明显削弱
2. 订单创建主链路瘦身后，系统吞吐和阶段时延都有实质改善
3. 当前业务高失败率仍然存在，但主因很可能已经转移到库存、幂等、状态校验或其他业务拒绝，而不是单纯的商品读取编排
4. 现在继续盲目加缓存或继续盲目升压，价值都不高；更有价值的是先把失败原因分桶

### 24.9 下一轮前的必做项

在继续第十二轮之前，建议先补两类能力：

1. 在 `k6` 脚本里统计非 `code=0` 的业务错误码和错误消息
2. 在 `gateway` 或 `order-service` 增加 `CreateOrder` 按错误码分桶的日志或 metrics

目标：

- 区分库存不足、幂等冲突、状态非法、下游超时、服务不可用等不同失败来源
- 避免下一轮仍然只能看到“失败率很高”，却看不出真正失败原因

### 24.10 本轮停止原因

第十一轮没有继续往更高并发推进，原因是当前信息已经足够支撑下一步优化判断：

1. 第十轮确认的首瓶颈已经被优化并复测验证
2. 本轮已经明确看到吞吐和下单阶段时延改善
3. 当前最缺的不是更高并发样本，而是失败原因拆解能力
4. 继续升压只能重复放大同一类“业务失败率很高”的现象，暂时无法提供更高价值的信息

## 25. 第十二轮结果

时间：

- `2026-03-25`

第十二轮目标：

- 使用新增的失败原因分桶能力，继续复测核心订单场景
- 确认第十一轮之后的高失败率主要集中在哪些业务阶段
- 判断下一步该优先优化幂等路径、库存预占还是订单持久化

### 25.1 第十二轮开始前的改动

本轮是在以下观测增强落地后进行：

1. `order-service` 的 `CreateOrder` 失败路径增加按阶段、按错误码统计的业务错误 metrics
2. `gateway` 的下单入口增加 RPC 错误与 RPC 业务错误的业务分桶 metrics
3. `k6` 订单脚本和 checkout 脚本增加失败分桶计数与错误码 / 原因输出

本轮新增观测目标：

- `request`
- `idempotency_lookup`
- `idempotency_create`
- `validation`
- `reserve_rpc`
- `reserve_biz`
- `persist`

### 25.2 第十二轮结果文件

有效结果：

- [round12-order-core-smoke.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round12-order-core-smoke.json)
- [round12-order-hot-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round12-order-hot-v10.json)
- [round12-order-normal-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round12-order-normal-v10.json)
- [round12-order-hot-v40.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round12-order-hot-v40.json)
- [round12-order-normal-v40.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round12-order-normal-v40.json)

无效样本：

- [round12-checkout-core-smoke.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round12-checkout-core-smoke.json)
- [round12-checkout-v10.json](/Users/ruitong/GolandProjects/MeshCart/loadtest/results/round12-checkout-v10.json)

说明：

- `checkout-v10` 在 setup 登录阶段就被 `user-service` 异常打断，不能作为有效交易链路样本参与结论统计

### 25.3 冒烟结果

#### `POST /api/v1/orders`

- 场景：纯下单冒烟
- 结果：业务失败率 `0%`
- `order_create_duration avg`：约 `45.03ms`
- `order_create_duration p95`：约 `101.01ms`

#### `创建订单 -> 创建支付 -> 模拟支付确认`

- 场景：核心结算链路冒烟
- 结果：业务失败率 `0%`
- `checkout_duration avg`：约 `145.31ms`
- `checkout_duration p95`：约 `310ms`

结论：

- 低压下订单与 checkout 脚本仍然可运行
- 第十二轮新增的失败分桶逻辑本身没有引入功能性错误

### 25.4 第十二轮关键结果

#### `POST /api/v1/orders` 热点 SKU

`10 VUs`：

- 总请求数：`34255`
- 吞吐：`1690.75 req/s`
- 业务失败率：`98.47%`
- 成功请求数：`525`
- `order_create_duration avg`：`4.73ms`
- `order_create_duration p95`：`2.62ms`
- 最大耗时：`777.25ms`

`40 VUs`：

- 总请求数：`27462`
- 吞吐：`1274.02 req/s`
- 业务失败率：`99.69%`
- 成功请求数：`85`
- `order_create_duration avg`：`29.79ms`
- `order_create_duration p95`：`4.97ms`
- 最大耗时：`2019.59ms`

#### `POST /api/v1/orders` 普通 SKU 池

`10 VUs`：

- 总请求数：`37562`
- 吞吐：`1853.76 req/s`
- 业务失败率：`98.36%`
- 成功请求数：`616`
- `order_create_duration avg`：`4.11ms`
- `order_create_duration p95`：`2.95ms`
- 最大耗时：`794.28ms`

`40 VUs`：

- 总请求数：`40127`
- 吞吐：`1876.81 req/s`
- 业务失败率：`99.31%`
- 成功请求数：`275`
- `order_create_duration avg`：`19.45ms`
- `order_create_duration p95`：`6.04ms`
- 最大耗时：`2024.15ms`

### 25.5 第十二轮失败分桶结果

本轮最关键的新信息，不是单纯的吞吐和时延，而是失败开始具备可拆解的阶段信息。

`order-service` 当前观测到的 `CreateOrder` 失败分布：

- `idempotency_lookup`：`554`
- `reserve_biz`：`185`
- `persist`：`9`
- `reserve_rpc`：`6`
- `validation`：`6`
- `idempotency_create`：`3`

同时，`order-service` 的 RPC 业务错误里：

- `meshcart_rpc_errors_total{method="create_order", code="1009999"}`：`763`

这说明当前最主要的失败并不是均匀分布在各阶段，而是首先集中在：

1. 订单幂等记录查询阶段
2. 库存预占业务阶段

### 25.6 第十二轮现象

从日志和 metrics 结合看，本轮已经能把 `1009999` 进一步拆开成更具体的问题：

1. `order_action_records` 相关路径出现超时，导致 `idempotency_lookup` 成为失败最多的阶段
2. `CreateWithItems` 落库出现长时间阻塞，`persist_duration` 出现 `31s+`
3. `product-service` 的 `batchGetSku` 出现超时，最终在订单侧表现为“下游服务暂不可用”
4. 库存预占阶段既有业务失败，也有 RPC 失败

日志里可见的典型问题包括：

- `batch get sku failed`
- `create order validation failed`，消息为 `下游服务暂不可用，请稍后重试`
- `create order persist failed`，并伴随 `context deadline exceeded`
- `mark order action failed failed`

这意味着：

- 第十二轮的失败不再只是“高失败率”这个泛化现象
- 当前已经能确认幂等记录路径和订单持久化路径是非常可疑的热点

### 25.7 checkout 样本失效说明

本轮 `checkout-v10` 未形成有效样本，原因不是 checkout 脚本本身错误，而是依赖服务异常：

1. `user-service` 在登录阶段开始返回 `1009999`
2. `gateway` 日志显示 `user rpc login failed`
3. `user-service` 日志显示 `get user by username failed: context deadline exceeded`
4. 重新拉起 `user-service` 时又出现 `run migrations failed: can't acquire lock`

因此：

- 第十二轮不能基于 `checkout-v10` 得出支付链路层面的新结论
- 后续如果要继续交易链路压测，必须先清理 `user-service` 的迁移锁和登录超时问题

### 25.8 第十二轮结论

第十二轮最重要的结论：

1. 第十一轮之后剩下的高失败率，已经不再是“看不清原因”的黑盒现象
2. 当前订单失败首先集中在幂等记录查询路径，其次是库存预占业务失败
3. 订单持久化路径也存在严重长尾，部分请求在 `persist` 阶段阻塞到 `30s+`
4. `product-service` 的 `batchGetSku` 仍会在高压下超时，说明商品读链路虽然被削弱，但还没有完全稳定

### 25.9 下一步优化优先级

基于第十二轮结果，建议下一步按这个顺序推进：

1. 优先优化 `order_action_records` 幂等路径，减少 `lookup / create / mark failed` 的超时与额外往返
2. 排查 `CreateWithItems` 持久化长尾，重点看事务时长、慢 SQL、锁等待
3. 继续排查 `product-service` 的 `batchGetSku` 超时来源
4. 在库存侧补更细的失败原因和热点观测，再决定是否需要更激进的库存治理
5. 单独修复 `user-service` 的迁移锁与登录超时问题，再恢复 checkout 压测

### 25.10 本轮停止原因

第十二轮没有继续扩展更多链路场景，原因是当前结果已经给出足够明确的下一步方向：

1. 订单场景已经拿到了有效的失败分桶
2. 当前最有价值的工作是直接针对幂等与持久化路径做代码优化
3. checkout 依赖 `user-service` 异常，继续强行补跑不会形成高质量样本
4. 在修复这些问题之前，继续升压不会比现有结果提供更多高价值信息

## 26. 第十三轮结果

### 26.1 第十三轮前已落地优化

在第十三轮之前，已经针对第十二轮暴露出来的几个热点做了连续优化：

1. 收缩 `order-service` 的幂等路径，不再默认先查 `order_action_records`，而是优先尝试创建 `pending` 记录，仅在唯一键冲突时回查已有记录
2. 将幂等状态回写从按 `(action_type, action_key)` 更新，收缩为优先按 `order_action_records.id` 更新，减少一次唯一索引查找
3. 优化订单持久化路径，`CreateWithItems` 改为批量插入订单项，并去掉提交后的额外回库查询
4. 优化 `product-service` 的 `BatchGetSKU`，先对 `sku_ids` 去重，再用轻量查询替代带 `Attrs` 预加载的重查询

这一轮的目标，不再是验证商品读取链路，而是确认：

1. 幂等路径优化是否能降低失败率和平均时延
2. `persist` 长尾是否已经明显收敛
3. 剩余主瓶颈是否已经转移到库存预占链路

### 26.2 第十三轮结果文件

有效样本：

- `loadtest/results/round13-order-core-smoke.json`
- `loadtest/results/round13-order-hot-v10.json`
- `loadtest/results/round13-order-normal-v10.json`
- `loadtest/results/round13-order-hot-v40.json`
- `loadtest/results/round13-order-normal-v40.json`

无效样本：

- `loadtest/results/round13-checkout-core-smoke.json`

说明：

1. 本轮 `checkout` 没有继续形成正式对比样本
2. 原因不是订单链路本身，而是支付创建阶段在 `1 VU` 的 smoke 下就返回 `1009999 / service_unavailable`
3. 因此第十三轮仍然以订单核心链路为主

### 26.3 第十三轮冒烟结果

#### order smoke

- `order_create_failed`：`0`
- `order_create_duration avg`：`30.73ms`
- `order_create_duration p95`：`71.01ms`

这说明新一轮代码优化没有破坏下单基本功能，订单核心场景可以继续正式压测。

#### checkout smoke

- `checkout_failed`：`100%`
- `checkout create payment failed code=1009999`
- `reason=service_unavailable`

这说明当前 `checkout` 的首个阻塞点已经不是下单，而是支付创建依赖不可用，因此本轮没有继续执行正式 `checkout-v10` 样本。

### 26.4 第十三轮正式结果

#### `POST /api/v1/orders` 热门 SKU 池

`10 VUs`：

- 总请求数：`52295`
- 吞吐：`2594.50 req/s`
- 业务失败率：`98.91%`
- 成功请求数：`572`
- `order_create_duration avg`：`2.65ms`
- `order_create_duration p95`：`4.96ms`

`40 VUs`：

- 总请求数：`38427`
- 吞吐：`1888.35 req/s`
- 业务失败率：`99.01%`
- 成功请求数：`380`
- `order_create_duration avg`：`17.58ms`
- `order_create_duration p95`：`28.91ms`

#### `POST /api/v1/orders` 普通 SKU 池

`10 VUs`：

- 总请求数：`49473`
- 吞吐：`2459.78 req/s`
- 业务失败率：`98.86%`
- 成功请求数：`566`
- `order_create_duration avg`：`2.76ms`
- `order_create_duration p95`：`5.46ms`

`40 VUs`：

- 总请求数：`46045`
- 吞吐：`2250.80 req/s`
- 业务失败率：`98.64%`
- 成功请求数：`626`
- `order_create_duration avg`：`13.52ms`
- `order_create_duration p95`：`28.08ms`

### 26.5 第十二轮 vs 第十三轮对比

#### 热门 SKU 池

`10 VUs`：

- 吞吐：`1690.75 req/s -> 2594.50 req/s`
- 业务失败率：`98.47% -> 98.91%`
- 成功请求数：`525 -> 572`
- `avg`：`4.73ms -> 2.65ms`
- `p95`：`2.62ms -> 4.96ms`

`40 VUs`：

- 吞吐：`1274.02 req/s -> 1888.35 req/s`
- 业务失败率：`99.69% -> 99.01%`
- 成功请求数：`85 -> 380`
- `avg`：`29.79ms -> 17.58ms`
- `p95`：`4.97ms -> 28.91ms`

#### 普通 SKU 池

`10 VUs`：

- 吞吐：`1853.76 req/s -> 2459.78 req/s`
- 业务失败率：`98.36% -> 98.86%`
- 成功请求数：`616 -> 566`
- `avg`：`4.11ms -> 2.76ms`
- `p95`：`2.95ms -> 5.46ms`

`40 VUs`：

- 吞吐：`1876.81 req/s -> 2250.80 req/s`
- 业务失败率：`99.31% -> 98.64%`
- 成功请求数：`275 -> 626`
- `avg`：`19.45ms -> 13.52ms`
- `p95`：`6.04ms -> 28.08ms`

如何解读：

1. 在高并发场景下，第十三轮明显优于第十二轮，尤其是 `40 VUs` 成功数提升很明显
2. 平均时延整体继续下降，说明幂等路径、订单持久化路径和 SKU 读取路径优化是有效的
3. `p95` 在部分场景上升，说明主瓶颈虽然转移了，但高并发长尾还没有被清掉

### 26.6 第十三轮现象

这一轮最关键的变化，不是简单的吞吐上升，而是失败热点已经发生转移。

从 `order-service` 日志看：

1. `validation_duration` 普遍已经下降到几毫秒到十几毫秒
2. `persist_duration` 也不再出现第十二轮那种 `31s+` 的极端长尾，常见值已经落在几十毫秒量级
3. `batchGetSku` 在本轮采样日志里没有再出现明显超时
4. 当前最重的一段已经变成 `reserve_duration`

本轮日志里，`reserve_duration` 典型分布大致是：

- 常见：`200ms ~ 350ms`
- 热点压力下部分请求：`1s+`

这意味着：

1. 前面针对订单幂等、持久化、SKU 查询的优化已经起效
2. 当前订单主链路的主瓶颈已经基本转移到库存预占阶段

### 26.7 checkout 样本失效说明

第十三轮没有继续给出 `checkout-v10` 正式对比结果，原因很明确：

1. `checkout` smoke 在支付创建阶段就已经失败
2. 错误表现为 `create payment failed code=1009999`
3. 失败原因标签为 `service_unavailable`

因此：

1. 第十三轮不适合基于 checkout 样本对支付链路做进一步结论
2. 在恢复有效 checkout 压测之前，需要先单独修复 `payment-service` 或其依赖问题

### 26.8 第十三轮结论

第十三轮最重要的结论：

1. 第十二轮后实施的几项优化是有效的，订单链路整体吞吐和成功数都有改善
2. 订单幂等路径已经不再像第十二轮那样成为最突出的首瓶颈
3. 订单持久化长尾明显收敛，`persist` 已不再是最显眼的问题
4. 当前新的首瓶颈已经转移到库存预占链路，尤其是在高并发和热点 SKU 场景下

### 26.9 下一步优化优先级

基于第十三轮结果，建议下一步按这个顺序推进：

1. 优先优化 `inventory-service` 的库存预占路径，补齐成功、业务失败、超时三类分桶
2. 检查库存预占 SQL、事务范围、索引和锁等待，确认是否存在热点行竞争
3. 针对热点 SKU 评估更激进的治理手段，例如限并发、分片或排队
4. 单独修复 `payment-service` 的 `service_unavailable` 问题，恢复有效 checkout 压测

### 26.10 本轮停止原因

第十三轮没有继续扩展更多订单压力和 checkout 正式样本，原因是：

1. 订单链路已经给出了足够清晰的新瓶颈转移信号
2. 当前最值得做的不是继续盲目升压，而是直接进入库存预占链路优化
3. checkout 在支付创建阶段就失效，继续强行补跑不会形成高质量结论

## 27. 第十四轮结果

### 27.1 第十四轮前已落地优化

在第十四轮之前，已经完成了两类直接面向第十三轮瓶颈的改动：

1. `inventory-service` 预占路径补齐成功、业务失败、超时三类分桶，并增加热点 SKU 本地限并发
2. `payment-service -> order-service` 增加服务发现失败时的直连回退，恢复 checkout 的可测性

第十四轮的目标，不再是继续验证订单幂等或商品查询优化，而是确认：

1. 库存预占是否已经从新的首瓶颈回落
2. `checkout` 是否已经恢复为有效正式样本
3. 本轮是否还能继续沿用第十三轮的“高失败率”判断

### 27.2 第十四轮正式样本说明

第十四轮开始后，首次正式样本并不干净。

原因是：

1. 网关限流仍然在生效
2. 单纯设置 `GATEWAY_RATE_LIMIT_ENABLED=false` 并不能真正使所有规则失效
3. 第一次正式 `order-hot-v10` 样本里仍大量出现 `1000005 / rate_limited`

因此本轮正式结果采用的是“关闭所有网关限流规则之后重新跑”的第二次样本。

本轮有效结果文件：

- `loadtest/results/round14-order-hot-v10-clean.json`
- `loadtest/results/round14-order-normal-v10.json`
- `loadtest/results/round14-order-hot-v40.json`
- `loadtest/results/round14-order-normal-v40.json`
- `loadtest/results/round14-checkout-v10.json`

补充文件：

- `loadtest/results/round14-order-core-smoke.json`
- `loadtest/results/round14-checkout-core-smoke.json`
- `loadtest/results/round14-order-core-ratelimit-check.json`

### 27.3 第十四轮冒烟结果

#### order smoke

第一次 smoke 通过，说明订单链路在当前版本下基本功能正常。

重新关闭限流规则后的短 smoke 也通过：

- `order_create_failed`：`0`
- `order_create_duration avg`：`32.41ms`
- `order_create_duration p95`：`60.60ms`

#### checkout smoke

第十四轮最重要的新变化之一，是 checkout smoke 恢复为有效样本：

- `checkout_failed`：`0`
- `checkout_duration avg`：`83.33ms`
- `checkout_duration p95`：`151.90ms`
- `checkout_payment_create_duration p95`：`31.52ms`
- `checkout_payment_confirm_duration p95`：`79.49ms`

这说明：

1. `payment-service` 的 `service_unavailable` 已不再是 smoke 阶段阻塞点
2. 第十四轮可以继续执行正式 `checkout-v10`

### 27.4 第十四轮正式结果

#### `POST /api/v1/orders` 热门 SKU 池

`10 VUs`：

- 吞吐：`42.40 req/s`
- 成功请求数：`859`
- 业务失败率：`0`
- `order_create_duration avg`：`182.19ms`
- `order_create_duration p95`：`514.50ms`

`40 VUs`：

- 吞吐：`154.74 req/s`
- 成功请求数：`3134`
- 业务失败率：`0`
- `order_create_duration avg`：`205.46ms`
- `order_create_duration p95`：`402.50ms`

#### `POST /api/v1/orders` 普通 SKU 池

`10 VUs`：

- 吞吐：`87.99 req/s`
- 成功请求数：`1771`
- 业务失败率：`0`
- `order_create_duration avg`：`61.64ms`
- `order_create_duration p95`：`170.03ms`

`40 VUs`：

- 吞吐：`165.84 req/s`
- 成功请求数：`3352`
- 业务失败率：`0`
- `order_create_duration avg`：`188.37ms`
- `order_create_duration p95`：`388.98ms`

#### checkout 核心链路

`10 VUs`：

- 吞吐：`81.64 req/s`
- 成功请求数：`1659`
- 业务失败率：`0`
- `checkout_duration avg`：`313.92ms`
- `checkout_duration p95`：`666.60ms`
- `checkout_order_create_duration p95`：`193.05ms`
- `checkout_payment_create_duration p95`：`136.02ms`
- `checkout_payment_confirm_duration p95`：`365.43ms`

### 27.5 第十三轮 vs 第十四轮对比

#### 订单链路

和第十三轮相比，第十四轮最显著的变化，不是吞吐更高，而是业务失败率从 `98%+` 直接下降到 `0`。

热门 SKU 池：

`10 VUs`：

- 成功请求数：`572 -> 859`
- 业务失败率：`98.91% -> 0`
- `avg`：`2.65ms -> 182.19ms`
- `p95`：`4.96ms -> 514.50ms`

`40 VUs`：

- 成功请求数：`380 -> 3134`
- 业务失败率：`99.01% -> 0`
- `avg`：`17.58ms -> 205.46ms`
- `p95`：`28.91ms -> 402.50ms`

普通 SKU 池：

`10 VUs`：

- 成功请求数：`566 -> 1771`
- 业务失败率：`98.86% -> 0`
- `avg`：`2.76ms -> 61.64ms`
- `p95`：`5.46ms -> 170.03ms`

`40 VUs`：

- 成功请求数：`626 -> 3352`
- 业务失败率：`98.64% -> 0`
- `avg`：`13.52ms -> 188.37ms`
- `p95`：`28.08ms -> 388.98ms`

如何理解这一变化：

1. 第十三轮的高吞吐，很大一部分来自“请求快速失败”
2. 第十四轮开始，大量请求真正走完了商品校验、库存预占、落库，所以耗时明显上升
3. 从压测结论角度，第十四轮比第十三轮更接近真实容量表现，因为它不再被高业务失败率扭曲

#### checkout 链路

第十三轮 checkout 只拿到了 smoke 且样本失效，第十四轮已经恢复为有效正式样本：

1. `checkout-v10` 可以完整执行
2. 支付创建和支付确认都不再被 `service_unavailable` 阻断
3. 当前 `checkout` 全链路 `p95` 为 `666.60ms`，仍有继续优化空间，但已经能作为正式对比样本使用

### 27.6 第十四轮库存观测

第十四轮新补上的库存预占分桶也给出了非常明确的信号。

当前 `inventory-service` 暴露的指标中：

- `meshcart_inventory_reservation_requests_total{action="reserve", outcome="success", reason="ok"}`：`11925`

同时，本轮采样时未观察到：

1. `biz_failed` 分桶计数
2. `timeout` 分桶计数

从 `meshcart_inventory_reservation_duration_seconds` 直方图看：

1. 大部分成功预占已经落在 `50ms` 到 `250ms` 以内
2. 没有出现上一轮那种明显的超时型失败信号

这说明第十四轮里：

1. 库存预占已经不再表现为“显式业务失败或超时”的主要来源
2. 之前针对库存预占链路的优化在这一轮是有效的

### 27.7 第十四轮现象

从 `order-service` 日志采样看，第十四轮的订单创建链路分段耗时已经比较稳定：

1. `validation_duration` 仍然在几毫秒量级
2. `reserve_duration` 多数落在十几毫秒到二十几毫秒
3. `persist_duration` 也多数稳定在个位到十几毫秒

同时，`checkout-v10` 的分段结果说明：

1. 当前 `payment_confirm` 比 `payment_create` 更重
2. 全链路最大的耗时段已经不是支付创建异常，而是成功链路本身的串行累计

### 27.8 第十四轮结论

第十四轮最重要的结论：

1. 第十三轮后针对库存预占和支付服务发现的优化已经生效
2. 订单四组正式样本的业务失败率已经从 `98%+` 降到 `0`
3. `checkout-v10` 已恢复为有效正式样本，支付链路不再被 `service_unavailable` 阻断
4. 当前系统已经从“高失败率阶段”进入“成功率恢复后重新评估真实容量”的阶段

### 27.9 下一步建议

基于第十四轮结果，建议下一步按这个顺序推进：

1. 先把第十四轮结果同步到高并发优化优先级文档，调整问题判断
2. 不要立刻用第十三轮那套失败导向结论继续优化，而是重新识别第十四轮下的真实容量瓶颈
3. 第十五轮如果继续压测，应在当前干净配置下逐步升压，确认成功率恢复后的拐点在哪里
4. 后续可专门增加更极端的热点 SKU 场景，验证库存热点治理是否仍有边界问题

### 27.10 本轮停止原因

第十四轮没有继续直接升到更高压力，原因是：

1. 本轮已经确认订单和 checkout 链路都恢复成有效样本
2. 当前最重要的不是继续堆更多样本，而是先重写对系统容量阶段的判断
3. 第十四轮和第十三轮的性质已经不同，需要先在文档和优化优先级上完成结论切换

## 28. 第十五轮结果

### 28.1 第十五轮目标

第十五轮不再以“修复是否生效”为唯一目标，而是开始在第十四轮干净样本的基础上，识别成功率恢复后的真实容量拐点。

本轮主要回答三个问题：

1. 普通 SKU 和热点 SKU 的稳定容量是否已经出现明显分化
2. checkout 在更高并发下是否仍能维持有效成功样本
3. 当前真实拐点更接近热点库存场景，还是已经扩散到普通交易链路

### 28.2 第十五轮样本范围

本轮沿用第十四轮的干净配置继续执行：

1. 网关所有限流规则继续关闭，避免再次污染正式样本
2. 保持现有代码版本不变，不引入新的实现变量
3. 保留已有的业务失败分桶和库存预占分桶指标

本轮结果文件：

- `loadtest/results/round15-order-core-smoke.json`
- `loadtest/results/round15-checkout-core-smoke.json`
- `loadtest/results/round15-order-hot-v20.json`
- `loadtest/results/round15-order-normal-v20.json`
- `loadtest/results/round15-order-hot-v40.json`
- `loadtest/results/round15-order-normal-v40.json`
- `loadtest/results/round15-checkout-v10.json`
- `loadtest/results/round15-checkout-v20.json`

说明：

1. `order-hot-v20` 和 `order-hot-v40` 的 `k6` 退出码为 `99`
2. 原因是阈值 `order_create_duration p(95)<1500` 被触发
3. 结果文件本身有效，仍可用于正式分析

### 28.3 第十五轮冒烟结果

#### order smoke

订单 smoke 通过，说明第十五轮开始时订单链路仍可正常执行：

1. 可以继续执行正式订单阶梯升压
2. 没有回退到第十三轮那种高比例业务快速失败状态

#### checkout smoke

checkout smoke 也通过，说明：

1. `payment-service` 的可测性仍然保持正常
2. checkout 可以继续执行正式 `v10 / v20` 样本

### 28.4 第十五轮正式结果

#### `POST /api/v1/orders` 热门 SKU 池

`20 VUs`：

- 吞吐：`18.52 req/s`
- 成功请求数：`565`
- 业务失败率：`0.35%`
- `order_create_duration avg`：`1022.15ms`
- `order_create_duration p95`：`1589.97ms`

`40 VUs`：

- 吞吐：`36.71 req/s`
- 成功请求数：`1085`
- 业务失败率：`3.13%`
- `order_create_duration avg`：`1032.31ms`
- `order_create_duration p95`：`1547.65ms`

#### `POST /api/v1/orders` 普通 SKU 池

`20 VUs`：

- 吞吐：`88.17 req/s`
- 成功请求数：`2694`
- 业务失败率：`0`
- `order_create_duration avg`：`174.26ms`
- `order_create_duration p95`：`334.39ms`

`40 VUs`：

- 吞吐：`177.45 req/s`
- 成功请求数：`5398`
- 业务失败率：`0`
- `order_create_duration avg`：`172.28ms`
- `order_create_duration p95`：`317.07ms`

#### checkout 核心链路

`10 VUs`：

- 吞吐：`48.96 req/s`
- 成功请求数：`499`
- 业务失败率：`0`
- `checkout_duration avg`：`557.03ms`
- `checkout_duration p95`：`779.20ms`

`20 VUs`：

- 吞吐：`97.77 req/s`
- 成功请求数：`993`
- 业务失败率：`0.20%`
- `checkout_duration avg`：`557.48ms`
- `checkout_duration p95`：`764.90ms`

### 28.5 第十四轮 vs 第十五轮对比

#### 热门 SKU 池

和第十四轮相比，热门 SKU 已经出现非常明确的容量拐点。

`40 VUs`：

- 吞吐：`154.74 req/s -> 36.71 req/s`
- 成功请求数：`3134 -> 1085`
- 业务失败率：`0 -> 3.13%`
- `avg`：`205.46ms -> 1032.31ms`
- `p95`：`402.50ms -> 1547.65ms`

这说明：

1. 热点 SKU 场景已经不再维持第十四轮的稳定成功状态
2. 当前热点容量瓶颈不是轻微退化，而是已经进入明显长尾放大区间
3. 第十五轮不再继续升到 `hot-v60 / hot-v80` 是合理停止，而不是样本不足

#### 普通 SKU 池

普通 SKU 仍然保持稳定，且没有出现明显业务失败：

`40 VUs`：

- 吞吐：`165.84 req/s -> 177.45 req/s`
- 成功请求数：`3352 -> 5398`
- 业务失败率：`0 -> 0`
- `avg`：`188.37ms -> 172.28ms`
- `p95`：`388.98ms -> 317.07ms`

这说明：

1. 当前普通 SKU 下单链路还没有到明显失稳点
2. 第十五轮暴露的问题更偏向热点竞争，而不是系统整体已经退化

#### checkout 链路

checkout 也保持了有效样本，但在更高档位已经开始出现轻微退化：

`10 VUs -> 20 VUs`：

- 吞吐：`48.96 req/s -> 97.77 req/s`
- 业务失败率：`0 -> 0.20%`
- `avg`：`557.03ms -> 557.48ms`
- `p95`：`779.20ms -> 764.90ms`

同时，相比第十四轮 `checkout-v10`：

- 吞吐：`81.64 req/s -> 48.96 req/s`
- `avg`：`313.92ms -> 557.03ms`
- `p95`：`666.60ms -> 779.20ms`

这说明 checkout 仍可用，但成功链路本身的真实耗时已经比第十四轮更重，不能再简单按“完全恢复”来判断。

### 28.6 第十五轮服务侧观测

本轮结束后，`inventory-service` 的预占分桶仍主要表现为成功：

- `meshcart_inventory_reservation_requests_total{action="reserve", outcome="success", reason="ok"}`：`23217`

本轮采样时，未观察到明显的：

1. `biz_failed` 分桶计数增长
2. `timeout` 分桶计数增长

但 `order-service` 指标中仍出现少量：

- `meshcart_biz_errors_total{service="order-service", module="order", action="create", stage="reserve_rpc", code="1009999"}`：`16`

同时：

- `meshcart_db_wait_count_total{service="order-service"}`：`2`

这说明第十五轮里：

1. 普通订单链路还没有被数据库连接池明显卡住
2. 热点 SKU 的退化更像预占链路长尾或热点竞争放大
3. 当前观测还不足以证明系统级数据库等待是主因

### 28.7 第十五轮结论

第十五轮最重要的结论：

1. 第十四轮恢复成功率后，系统的真实容量边界已经开始显现
2. 普通 SKU 下单链路和 checkout 仍然基本稳定
3. 热点 SKU 场景已经在 `40 VUs` 左右出现新的明显拐点
4. 当前最值得继续优化的，不是普通链路，而是热点库存场景下的预占与竞争治理

### 28.8 下一步建议

基于第十五轮结果，建议下一步按这个顺序推进：

1. 先把第十五轮结果同步到高并发优化优先级文档，调整热点治理优先级
2. 针对热点 SKU 单独做更细粒度观测，补充预占阶段长尾和失败原因
3. 评估更激进的热点治理手段，例如更严格的限并发、分片或排队
4. 普通 SKU 和 checkout 暂不需要盲目继续升压，应先稳定热点场景判断

### 28.9 本轮停止原因

第十五轮没有继续直接升到更高热点压力，原因是：

1. `hot-v40` 已经出现明确业务失败和 `p95` 超过 `1.5s`
2. 继续升到 `hot-v60 / hot-v80` 的价值已经不高，更多会重复确认同一类退化
3. 本轮已经足够证明“热点 SKU 是当前新拐点”的判断

## 29. 第十六轮结果

### 29.1 第十六轮前已落地优化

在第十六轮之前，已经针对热点库存预占路径继续做了两类优化：

1. `inventory-service` 的 `reserve/release/confirm` 路径进一步收缩 SQL 往返，减少事务外额外回库
2. 热点 SKU 本地预占并发阈值继续收紧，从偏宽松值降到更保守的 `2`

因此第十六轮不再跑全量场景，而是专门验证：

1. 热点 SKU 的失败率是否下降
2. 热点 SKU 的 `p95` 是否明显回落
3. 第十五轮暴露出来的热点拐点是否已被推后

### 29.2 第十六轮样本范围

本轮采用热点专项最小方案：

1. `order-hot-smoke`
2. `order-hot-v20`
3. `order-hot-v40`

本轮结果文件：

- `loadtest/results/round16-order-hot-smoke.json`
- `loadtest/results/round16-order-hot-v20.json`
- `loadtest/results/round16-order-hot-v40.json`

说明：

1. 第十六轮前先重启了 `inventory-service`，确保热点优化代码和更严格的并发阈值已实际生效
2. `gateway` 继续沿用无网关限流的干净配置，避免再次污染正式样本

### 29.3 第十六轮冒烟结果

#### order hot smoke

热点订单 smoke 通过：

- 业务失败率：`0`
- `order_create_duration avg`：`292.52ms`
- `order_create_duration p95`：`357.54ms`

这说明：

1. 第十六轮开始时热点订单链路基本功能正常
2. 可以继续执行热点正式样本

### 29.4 第十六轮正式结果

#### `POST /api/v1/orders` 热门 SKU 池

`20 VUs`：

- 吞吐：`63.34 req/s`
- 成功请求数：`1911`
- 业务失败率：`0`
- `order_create_duration avg`：`263.40ms`
- `order_create_duration p95`：`393.03ms`

`40 VUs`：

- 吞吐：`115.45 req/s`
- 成功请求数：`3514`
- 业务失败率：`0`
- `order_create_duration avg`：`292.57ms`
- `order_create_duration p95`：`397.38ms`

### 29.5 第十五轮 vs 第十六轮对比

第十六轮最重要的结果，是第十五轮暴露出来的热点退化已经被明显压回去了。

#### 热门 SKU 池

`20 VUs`：

- 吞吐：`18.52 req/s -> 63.34 req/s`
- 成功请求数：`565 -> 1911`
- 业务失败率：`0.35% -> 0`
- `avg`：`1022.15ms -> 263.40ms`
- `p95`：`1589.97ms -> 393.03ms`

`40 VUs`：

- 吞吐：`36.71 req/s -> 115.45 req/s`
- 成功请求数：`1085 -> 3514`
- 业务失败率：`3.13% -> 0`
- `avg`：`1032.31ms -> 292.57ms`
- `p95`：`1547.65ms -> 397.38ms`

这说明：

1. 第十五轮在热点 SKU 上观察到的明显拐点已经被推后
2. 当前 `hot-v40` 已经重新回到可接受范围，不再出现前一轮那种长尾放大
3. 本轮优化对热点库存预占路径是有效的

### 29.6 第十六轮服务侧观测

本轮采样时，`inventory-service` 的预占分桶仍然比较干净：

- `meshcart_inventory_reservation_requests_total{action="reserve", outcome="success", reason="ok"}`：`5432`

同时未观察到：

1. 明显的 `biz_failed` 分桶增长
2. 明显的 `timeout` 分桶增长

数据库侧当前观测也比较稳定：

- `meshcart_db_wait_count_total{service="inventory-service"}`：`0`
- `meshcart_db_wait_count_total{service="order-service"}`：`2`

另外，`order-service` 中：

- `meshcart_biz_errors_total{service="order-service", module="order", action="create", stage="reserve_rpc", code="1009999"}`：`16`

本轮未观察到它继续明显抬升。

这说明第十六轮里：

1. 热点库存预占没有再表现出上一轮那种明显失败放大
2. 当前热点链路改善并不是由数据库连接池等待消失之外的新异常掩盖出来的假象
3. 这轮结果可以作为热点优化有效的正式证据

### 29.7 第十六轮结论

第十六轮最重要的结论：

1. 针对热点库存预占路径的进一步优化已经生效
2. 第十五轮暴露出来的热点 SKU 明显退化已被压回去
3. 热点 `v20 / v40` 当前都恢复成 `0` 业务失败的有效样本
4. 说明现阶段系统的热点容量边界已经再次后移

### 29.8 下一步建议

基于第十六轮结果，建议下一步按这个顺序推进：

1. 先把第十六轮结果同步到高并发优化优先级文档，更新热点治理判断
2. 在热点专项继续升压，补跑 `hot-v60`，必要时再看 `hot-v80`
3. 如果热点更高档位仍稳定，再回头补普通 SKU 和 checkout 的对照复测
4. 在确认新拐点之前，不要贸然把当前热点治理结论当成最终容量上限

### 29.9 本轮停止原因

第十六轮没有继续直接跑 `hot-v60 / hot-v80`，原因是：

1. 本轮核心目标是验证热点库存优化是否有效
2. `hot-v20 / hot-v40` 已经足够证明这次优化把上一轮的明显退化压回去了
3. 在结论落档之前，继续升压的优先级低于先固化本轮观察结果

## 30. 第十七轮结果

### 30.1 第十七轮目标

第十七轮继续沿着热点专项路线推进，目标不再是验证热点库存优化是否有效，而是继续寻找新的热点容量边界。

本轮主要回答两个问题：

1. 在第十六轮恢复 `hot-v40` 稳定后，热点 SKU 是否还能继续支撑 `v60 / v80`
2. 如果热点库存链路继续稳定，新的压力会不会转移到 `order-service` 或数据库侧

### 30.2 第十七轮样本范围

本轮延续第十六轮的干净配置，只做热点升压：

1. `order-hot-v60`
2. `order-hot-v80`

本轮结果文件：

- `loadtest/results/round17-order-hot-v60.json`
- `loadtest/results/round17-order-hot-v80.json`

说明：

1. `gateway` 继续保持无网关限流的干净配置
2. `inventory-service` 继续沿用第十六轮已生效的热点预占优化版本

### 30.3 第十七轮正式结果

#### `POST /api/v1/orders` 热门 SKU 池

`60 VUs`：

- 吞吐：`71.34 req/s`
- 成功请求数：`2171`
- 业务失败率：`0`
- `order_create_duration avg`：`782.57ms`
- `order_create_duration p95`：`871.66ms`

`80 VUs`：

- 吞吐：`94.60 req/s`
- 成功请求数：`2925`
- 业务失败率：`0`
- `order_create_duration avg`：`782.15ms`
- `order_create_duration p95`：`890.45ms`

### 30.4 第十六轮 vs 第十七轮对比

第十七轮最重要的新信息，不是“热点重新退化”，而是“热点库存链路继续稳定，但时延平台已经抬高”。

#### 热门 SKU 池

相对第十六轮 `hot-v40`：

`60 VUs`：

- 吞吐：`115.45 req/s -> 71.34 req/s`
- 成功请求数：`3514 -> 2171`
- 业务失败率：`0 -> 0`
- `avg`：`292.57ms -> 782.57ms`
- `p95`：`397.38ms -> 871.66ms`

`80 VUs`：

- 吞吐：`115.45 req/s -> 94.60 req/s`
- 成功请求数：`3514 -> 2925`
- 业务失败率：`0 -> 0`
- `avg`：`292.57ms -> 782.15ms`
- `p95`：`397.38ms -> 890.45ms`

这说明：

1. 热点 SKU 在 `v60 / v80` 仍然没有重新出现业务失败
2. 第十六轮后的热点优化不只是把 `v40` 修好，而是把热点容量边界继续向后推了
3. 但成功链路本身的耗时已经进入新的更高平台，说明系统正在承受新的阶段性压力

### 30.5 第十七轮服务侧观测

本轮结束后，`inventory-service` 指标仍然比较干净：

- `meshcart_inventory_reservation_requests_total{action="reserve", outcome="success", reason="ok"}`：`10528`

同时：

- `meshcart_db_wait_count_total{service="inventory-service"}`：`0`

说明热点库存预占在 `v60 / v80` 下没有重新暴露出明显的连接池等待或显式失败。

但 `order-service` 侧出现了新的信号：

- `meshcart_db_wait_count_total{service="order-service"}`：`27`

而：

- `meshcart_biz_errors_total{service="order-service", module="order", action="create", stage="reserve_rpc", code="1009999"}`：`16`

没有继续明显抬升。

这说明第十七轮里：

1. 热点库存预占本身不再是最先失稳的位置
2. 新的压力更像已经开始向 `order-service` 或其数据库侧传递
3. 当前如果继续只盯库存链路，已经不一定能解释后续更高压下的主要瓶颈

### 30.6 第十七轮结论

第十七轮最重要的结论：

1. 热点 SKU 在 `v60 / v80` 下仍保持 `0` 业务失败
2. 第十六轮后的热点优化继续有效，新拐点至少已经高于 `v80`
3. 当前热点链路的主要风险不再是显式库存失败，而是成功请求时延明显抬高
4. 系统的新压力点，已经开始从 `inventory-service` 向 `order-service` / 数据库侧转移

### 30.7 下一步建议

基于第十七轮结果，建议下一步按这个顺序推进：

1. 先补普通 SKU 的 `normal-v60 / normal-v80` 对照样本
2. 同时重点检查 `order-service` 的数据库等待、慢 SQL 和事务时长
3. 如果普通 SKU 也出现类似时延平台抬高，再把下一轮优化重点从库存切到订单持久化和 DB 侧
4. 在没有拿到普通 SKU 对照之前，不要把第十七轮的时延抬高简单归因到热点库存链路

### 30.8 本轮停止原因

第十七轮没有继续直接跑更高热点压力，原因是：

1. 本轮已经证明热点 SKU 在 `v80` 下仍能保持 `0` 业务失败
2. 当前最需要补的是普通 SKU 对照和 `order-service` DB 观测，而不是继续单边堆热点压力
3. 在瓶颈疑似开始转移后，继续只打热点链路的收益已经下降

## 31. 第十八轮结果

### 31.1 第十八轮目标

第十八轮的目标，是验证第十七轮看到的“时延平台抬高”是否已经扩散到普通 SKU 场景。

本轮主要回答两个问题：

1. 普通 SKU 在 `v60 / v80` 下是否也会出现明显的时延抬高
2. 如果普通 SKU 也出现类似现象，新的瓶颈是否已经从库存链路切换到 `order-service` / 数据库侧

### 31.2 第十八轮样本范围

本轮不再继续升压热点 SKU，而是补普通 SKU 对照样本：

1. `order-normal-v60`
2. `order-normal-v80`

本轮结果文件：

- `loadtest/results/round18-order-normal-v60.json`
- `loadtest/results/round18-order-normal-v80.json`

说明：

1. `gateway` 继续保持无网关限流的干净配置
2. `inventory-service` 继续沿用第十六轮后已生效的热点预占优化版本

### 31.3 第十八轮正式结果

#### `POST /api/v1/orders` 普通 SKU 池

`60 VUs`：

- 吞吐：`110.42 req/s`
- 成功请求数：`3350`
- 业务失败率：`0`
- `order_create_duration avg`：`486.71ms`
- `order_create_duration p95`：`742.27ms`

`80 VUs`：

- 吞吐：`144.36 req/s`
- 成功请求数：`4449`
- 业务失败率：`0`
- `order_create_duration avg`：`492.84ms`
- `order_create_duration p95`：`762.10ms`

### 31.4 第十七轮热点 vs 第十八轮普通 SKU 对照

第十八轮最重要的意义，不是普通 SKU 出现失败，而是它们在高并发下也进入了更高时延平台。

#### 普通 SKU 池

普通 SKU 在 `v60 / v80` 下仍保持 `0` 业务失败，但时延已经明显高于第十五轮早期档位：

`60 VUs`：

- 吞吐：`110.42 req/s`
- 业务失败率：`0`
- `avg`：`486.71ms`
- `p95`：`742.27ms`

`80 VUs`：

- 吞吐：`144.36 req/s`
- 业务失败率：`0`
- `avg`：`492.84ms`
- `p95`：`762.10ms`

与第十七轮热点场景相比：

1. 热点和普通 SKU 在高并发下都已经进入更高时延平台
2. 但两者都没有重新出现业务失败
3. 这说明当前主要问题已经不再是热点库存独有瓶颈，而是成功链路整体变重

### 31.5 第十八轮服务侧观测

本轮结束后，`inventory-service` 指标依然比较干净：

- `meshcart_inventory_reservation_requests_total{action="reserve", outcome="success", reason="ok"}`：`18327`
- `meshcart_db_wait_count_total{service="inventory-service"}`：`0`

说明普通 SKU 对照场景下，库存侧依旧没有重新暴露出明显的等待或失败信号。

但 `order-service` 侧数据库等待继续抬升：

- `meshcart_db_wait_count_total{service="order-service"}`：`51`

而：

- `meshcart_biz_errors_total{service="order-service", module="order", action="create", stage="reserve_rpc", code="1009999"}`：`16`

并没有继续明显增长。

这说明第十八轮里：

1. 新压力并没有主要表现为库存 RPC 失败
2. 更像是订单服务数据库连接等待、事务或持久化成本在上升
3. 当前瓶颈已经基本可以从库存链路切换到 `order-service` / DB 侧来判断

### 31.6 第十八轮结论

第十八轮最重要的结论：

1. 普通 SKU 在 `v60 / v80` 下也进入了更高时延平台
2. 普通 SKU 和热点 SKU 都仍保持 `0` 业务失败
3. 当前首要矛盾已经不再是库存预占，而是 `order-service` 及其数据库侧压力上升
4. 下一轮不应继续盲目升压，而应该开始针对订单持久化和 DB 路径优化

### 31.7 下一步建议

基于第十八轮结果，建议下一步按这个顺序推进：

1. 把优化重点从库存链路切换到 `order-service` 的 DB / 持久化侧
2. 重点检查订单创建事务时长、慢 SQL、连接池等待和持久化路径
3. 在优化 `order-service` 之后，再做下一轮对照压测验证时延是否回落
4. 当前阶段继续盲目升压的收益，已经低于先做订单侧优化

### 31.8 本轮停止原因

第十八轮没有继续直接跑更高普通 SKU 压力，原因是：

1. 本轮已经足够证明普通 SKU 也开始进入更高时延平台
2. 当前需要优先处理的是 `order-service` / DB 侧新瓶颈，而不是继续扩样本
3. 在结论已经收敛到订单侧之后，继续升压对后续优化方向的帮助有限
