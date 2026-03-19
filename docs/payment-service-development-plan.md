# Payment Service 开发与设计文档

## 1. 目的

本文档用于说明 MeshCart `payment-service` 当前已经实现的能力、核心业务设计、数据库模型、RPC 设计，以及后续继续推进的方向。

本文档不再按历史开发顺序堆叠阶段内容，而是优先收口当前已完成基线；未来计划统一放在后半部分。

## 2. 当前定位

`payment-service` 当前已经具备第一版内部支付能力，负责：

- 为订单创建支付单
- 保存支付单真相数据
- 查询订单关联支付单
- 模拟支付成功确认
- 调 `order-service.ConfirmOrderPaid`
- 记录支付动作幂等和支付状态流转

当前服务边界：

- `order-service`
  - 负责订单状态机和库存确认扣减
- `payment-service`
  - 负责支付单状态机、支付成功确认、支付动作幂等
- `gateway`
  - 当前还没有接入支付 HTTP
- 第三方支付渠道
  - 当前第一版只支持 `mock`
  - 还没有接真实支付宝、微信支付

## 3. 当前已实现能力

当前已经完成的支付能力包括：

- `payment-service` RPC 服务骨架、bootstrap、配置、Consul 注册、healthz/readyz/metrics、优雅停机
- 支付主表 `payments`
- 支付动作幂等表 `payment_action_records`
- 支付状态流转日志表 `payment_status_logs`
- `CreatePayment`
  - 订单状态校验
  - 同一订单有效支付单复用
  - 创建支付单幂等
- `GetPayment`
- `ListPaymentsByOrder`
- `ConfirmPaymentSuccess`
  - 当前只支持 `mock`
  - 调 `order-service.ConfirmOrderPaid`
  - 订单确认成功后推进支付单到 `succeeded`
- 创建支付单、支付成功确认两类幂等控制
- 支付状态流转日志和动作记录

## 4. 核心业务设计

### 4.1 支付状态机

当前支付状态定义在 [service.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/biz/service/service.go)：

- `1 = pending`
  - 支付单已创建，等待支付成功
- `2 = succeeded`
  - 支付成功
- `3 = failed`
  - 预留状态，当前第一版还没有启用
- `4 = closed`
  - 预留状态，当前第一版还没有启用

当前主要状态流转：

- `pending -> succeeded`

当前明确禁止：

- `succeeded -> failed`
- `succeeded -> closed`
- `closed -> succeeded`

### 4.2 创建支付单链路

当前实现位于 [create_payment.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/biz/service/create_payment.go)。

执行顺序：

1. 校验请求参数和支付方式
2. 若传 `request_id`，先检查创建支付单幂等记录
3. 查询是否已存在同一订单的有效支付单
   - 当前有效状态定义为 `pending/succeeded`
   - 如果已存在，则直接返回已有支付单
4. 调 `order-service.GetOrder`
5. 校验订单状态必须为 `reserved`
6. 读取订单应付金额 `pay_amount`
7. 创建支付单并写入：
   - `payment_id`
   - `order_id`
   - `user_id`
   - `payment_method`
   - `amount`
   - `currency`
   - `request_id`
8. 写支付状态流转日志：
   - `0 -> pending`

当前语义：

- 同一订单不会反复创建多笔有效支付单
- 如果用户重复点击支付，优先返回已有支付单
- 第一版只允许 `mock` 支付方式

### 4.3 支付成功确认链路

当前实现位于 [confirm_payment_success.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/biz/service/confirm_payment_success.go)。

执行顺序：

1. 校验 `payment_id` 和 `payment_method`
2. 若传 `request_id`，先检查支付成功确认幂等记录
3. 查询支付单并校验状态必须为 `pending`
4. 生成或读取 `payment_trade_no`
   - `mock` 场景默认生成 `mock-{payment_id}`
5. 调 `order-service.ConfirmOrderPaid`
   - `payment_id = payment_id` 的字符串形式
   - `payment_method`
   - `payment_trade_no`
   - `paid_at`
6. 若订单确认成功，再把支付单推进到 `succeeded`
7. 写支付状态流转日志：
   - `pending -> succeeded`

当前语义：

- 当前第一版采用“先确认订单成功，再确认支付成功”的顺序
- 这样可以避免出现“支付单已成功但订单未推进”的悬挂状态
- 同一个支付成功结果重复通知按幂等成功处理
- 已成功支付的支付单如果收到不同的 `payment_trade_no`，返回支付冲突

### 4.4 幂等与排障

当前实现位于：

- [helpers.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/biz/service/helpers.go)
- [repository.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/biz/repository/repository.go)

当前幂等覆盖：

- 创建支付单：
  - `CreatePaymentRequest.request_id`
- 支付成功确认：
  - 默认使用 `payment_id`
  - 若传 `request_id`，优先使用 `request_id`

动作状态：

- `pending`
- `succeeded`
- `failed`

当前排障入口：

- `payment_action_records`
  - 看某个支付动作是否执行、是否失败、失败文案是什么
- `payment_status_logs`
  - 看支付单经历过哪些状态流转

## 5. 数据库设计

### 5.1 支付主表 `payments`

定义见：

- [model.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/dal/model/model.go)
- [000001_create_payments.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/migrations/000001_create_payments.up.sql)

字段说明：

- `payment_id`
  - 支付单主键，雪花 ID
- `order_id`
  - 关联订单 ID
- `user_id`
  - 关联用户 ID
- `status`
  - 支付状态
- `payment_method`
  - 支付方式，当前只支持 `mock`
- `amount`
  - 支付金额，来自订单 `pay_amount`
- `currency`
  - 币种，当前固定为 `CNY`
- `payment_trade_no`
  - 外部渠道流水号；`mock` 场景在支付确认时生成
- `request_id`
  - 创建支付单请求幂等键
- `succeeded_at`
  - 支付成功时间
- `closed_at`
  - 关闭时间，当前第一版预留
- `fail_reason`
  - 支付失败原因，当前第一版预留
- `created_at`
  - 创建时间
- `updated_at`
  - 更新时间

索引说明：

- `idx_payments_order_id_status`
  - 支撑按订单查有效支付单
- `idx_payments_user_id`
  - 支撑按用户查支付单
- `idx_payments_status_updated_at`
  - 支撑按状态排障和后续恢复任务

### 5.2 支付动作幂等表 `payment_action_records`

定义见：

- [model.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/dal/model/model.go)
- [000002_create_payment_action_records.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/migrations/000002_create_payment_action_records.up.sql)

字段说明：

- `id`
  - 记录主键，雪花 ID
- `action_type`
  - 动作类型，例如：
    - `create`
    - `confirm_success`
- `action_key`
  - 幂等键
- `payment_id`
  - 关联支付单 ID
- `order_id`
  - 关联订单 ID
- `status`
  - 当前动作状态：
    - `pending`
    - `succeeded`
    - `failed`
- `error_message`
  - 失败时的错误文案
- `created_at`
  - 创建时间
- `updated_at`
  - 更新时间

约束说明：

- `action_type + action_key` 唯一

### 5.3 支付状态流转日志表 `payment_status_logs`

定义见：

- [model.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/dal/model/model.go)
- [000003_create_payment_status_logs.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/migrations/000003_create_payment_status_logs.up.sql)

字段说明：

- `id`
  - 日志主键
- `payment_id`
  - 关联支付单 ID
- `from_status`
  - 变更前状态
- `to_status`
  - 变更后状态
- `action_type`
  - 触发动作，例如：
    - `create`
    - `confirm_success`
- `reason`
  - 变更原因
- `external_ref`
  - 外部引用，例如 `payment_trade_no`
- `created_at`
  - 创建时间

设计意图：

- 支付状态变化和状态日志同事务落库
- 便于还原支付单状态演进过程

## 6. RPC 设计

IDL 定义见 [payment.thrift](/Users/ruitong/GolandProjects/MeshCart/idl/payment.thrift)。

当前 `PaymentService` 已提供这些 RPC：

- `CreatePayment`
- `GetPayment`
- `ListPaymentsByOrder`
- `ConfirmPaymentSuccess`

### 6.1 `CreatePayment`

作用：

- 为指定订单创建支付单

请求字段：

- `order_id`
- `user_id`
- `payment_method`
- `request_id`

关键语义：

- 当前只支持 `mock`
- 同一订单已有有效支付单时，直接返回已有支付单

### 6.2 `GetPayment`

作用：

- 查询单笔支付单详情

请求字段：

- `payment_id`
- `user_id`

关键语义：

- 当前按 `user_id + payment_id` 做资源隔离

### 6.3 `ListPaymentsByOrder`

作用：

- 查询某笔订单下的支付单列表

请求字段：

- `order_id`
- `user_id`

关键语义：

- 当前按 `order_id + user_id` 查全部支付单

### 6.4 `ConfirmPaymentSuccess`

作用：

- 确认支付成功

请求字段：

- `payment_id`
- `payment_method`
- `payment_trade_no`
- `paid_at`
- `request_id`

关键语义：

- 当前第一版既是支付成功确认入口，也是 `mock` 支付成功入口
- 当前会先调用 `order-service.ConfirmOrderPaid`
- 只有订单确认成功后，支付单才会推进到 `succeeded`

## 7. HTTP 接口现状

当前还没有对外开放支付 HTTP 接口。

也就是说，第一版已经具备支付域内部 RPC 能力，但还没有接入：

- `gateway/rpc/payment`
- 用户侧支付 HTTP handler / logic / types

这部分会放到下一阶段。

## 8. 运行与治理基线

`payment-service` 当前已经接入：

- `healthz / readyz / metrics`
- preflight
- graceful shutdown
- Consul 注册与发现
- tracing / metrics / logging
- 下游 `order-service` RPC timeout
- `GetOrder` 读 RPC 有限重试

关键落点：

- 启动与 bootstrap：
  - [main.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/rpc/main.go)
  - [bootstrap.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/rpc/bootstrap/bootstrap.go)
- 配置：
  - [config.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/config/config.go)
  - [payment-service.local.yaml](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/config/payment-service.local.yaml)
- 下游 client：
  - [client.go](/Users/ruitong/GolandProjects/MeshCart/services/payment-service/rpcclient/order/client.go)

当前重试口径：

- `order-service.GetOrder`
  - 已启用一次有限重试
- `order-service.ConfirmOrderPaid`
  - 写 RPC
  - 不自动重试

## 9. 测试情况

当前已补测试包括：

- service 测试
  - 创建支付单成功
  - 复用有效支付单
  - 支付成功确认成功
  - 支付成功确认时订单 RPC 异常
  - 查询支付单不存在
- RPC handler 测试
  - 创建支付单成功
  - 支付成功确认成功

## 10. 当前边界与未完成部分

当前还没有完成的主要是：

- `gateway` 对外支付 HTTP 接口
- `mock_success` 开发环境 HTTP 联调接口
- `ClosePayment`
- `failed / closed` 状态的完整流转
- 常驻补偿与恢复任务
- 管理端支付查询接口
- 真实支付渠道接入
- 第三方支付回调验签和原始报文留存

## 11. 后续推进计划

### 11.1 近期计划

优先顺序建议：

1. 接 `gateway`
   - 对外开放创建支付单、查询支付单接口
2. 增加开发环境 `mock_success` 联调接口
   - 让下单到支付成功的整条链路可从 HTTP 跑通
3. 补 `ClosePayment`
   - 为订单超时关闭后的支付单关闭做准备
4. 补管理端支付查询
   - 后台可查支付单和状态流转

### 11.2 中期计划

- 支付补偿和恢复任务
- 更完整的 `failed / closed` 状态流转
- 支付回调原始报文表
- 管理端操作审计
- 与订单、网关的联调验收链路

### 11.3 高并发演进计划

当前支付服务的高并发演进方向保持不变：

- `gateway`
  - 入口限流
- `payment-service`
  - 继续作为支付单真相账本和支付状态机
- MQ
  - 异步处理支付成功通知和恢复任务
- Redis
  - 做短期防重和热点读缓存

必须坚持的约束：

- 支付单真相仍在 MySQL
- 不能让 Redis 成为最终支付事实来源
- 支付成功后的订单确认不能绕过 `order-service`

## 12. 推荐阅读

- [docs/order-service-development-plan.md](/Users/ruitong/GolandProjects/MeshCart/docs/order-service-development-plan.md)
- [docs/service-development-spec.md](/Users/ruitong/GolandProjects/MeshCart/docs/service-development-spec.md)
- [docs/microservice-governance.md](/Users/ruitong/GolandProjects/MeshCart/docs/microservice-governance.md)
