# MeshCart 日志打印规范

## 1. 目的

本文档用于约束 MeshCart 后续日志改造与新增日志的打印方式，重点解决以下问题：

- 只打印业务错误码，看不到底层真实原因
- 同一类错误在不同服务中的日志风格不一致
- 日志字段不统一，跨服务排障检索成本高
- 技术错误、业务拒绝、正常成功混在同一日志级别

目标：

- 明确什么场景必须打日志
- 明确日志应该打到哪一层
- 明确不同级别日志的使用边界
- 明确标准字段与命名约定
- 让后续 `gateway`、`order-service`、`payment-service`、`inventory-service` 的日志改造有统一参照

## 2. 适用范围

当前适用于：

- `gateway/`
- `services/*-service/`
- `app/log`
- `docs/`

默认要求：

- 新增服务遵守本规范
- 旧服务后续重构日志时逐步对齐本规范

## 3. 总体原则

### 3.1 对外错误与内部日志分离

对外返回给前端或调用方的错误信息，继续保持业务友好、稳定、可控。

日志则必须保留：

- 原始技术错误
- 业务判定上下文
- 关键状态字段

禁止用“系统内部错误”“下游服务不可用”这类对外文案替代内部排障日志。

### 3.2 结构化优先

统一使用 `app/log` 输出结构化日志。

要求：

- 不新增 `fmt.Println`
- 不新增零散文本拼接日志
- 优先使用 `zap` 字段

### 3.3 业务可预期失败不等于技术异常

例如：

- 库存不足
- 商品下架
- 订单已过期
- 支付单状态不允许关闭

这类属于业务拒绝，不应一律打印为 `error`。

### 3.4 技术错误必须带原始 error

例如：

- 数据库写入失败
- RPC timeout
- 连接拒绝
- 唯一键冲突
- 事务提交失败

这类日志必须包含 `zap.Error(err)` 或等价原始错误信息。

### 3.5 关键状态迁移必须可追踪

涉及状态机的核心动作必须有日志可追溯：

- 创建订单
- 取消订单
- 超时关闭订单
- 创建支付单
- 支付成功确认
- 关闭支付单
- 库存预占 / 释放 / 确认扣减

## 4. 日志分层要求

### 4.1 Handler / RPC Handler 层

允许打印：

- 请求绑定失败
- 鉴权失败
- 调用 logic / service 后的最终业务结果

要求：

- 不重复打印和下层完全相同的技术错误细节
- 重点记录接口入口、资源 ID、最终业务码

适合的日志：

- `request bind failed`
- `unauthorized request`
- `create payment failed`

不适合的日志：

- repository SQL 原始错误
- 事务内部细节

### 4.2 Logic / Service 层

这是业务日志的核心层。

必须打印：

- 业务关键动作开始 / 成功 / 拒绝
- 状态机拒绝原因
- 下游 RPC 失败及其映射结果

要求：

- 明确“为什么拒绝”
- 明确“调用了哪个下游”
- 明确“把下游错误映射成了什么业务错误”

例如：

- `confirm payment success rejected by payment expiry`
- `close payment rejected by payment status`
- `confirm order paid blocked by inventory rpc business error`

### 4.3 Repository 层

这是技术错误日志的核心层。

必须打印：

- DB 查询失败
- DB 更新失败
- 事务执行失败
- 唯一键冲突、状态冲突对应的上下文

要求：

- 必须带原始 `err`
- 必须带主业务键
- 不只返回 `ErrInternalError` 而不留底层痕迹

### 4.4 RPC Client 层

允许打印：

- 调用下游服务失败
- nil response
- transport error

要求：

- 不写业务规则
- 不吞掉原始 RPC error

## 5. 日志级别规范

### 5.1 `Debug`

用于：

- 本地开发辅助排查
- 高频但低价值的临时上下文

要求：

- 默认生产可关闭
- 不打印大对象全文

### 5.2 `Info`

用于：

- 关键业务动作成功
- 状态迁移完成
- 幂等命中且语义正常

典型场景：

- 创建订单成功
- 创建支付单成功
- 支付成功确认完成
- 库存预占成功

### 5.3 `Warn`

用于：

- 可预期业务拒绝
- 幂等处理中
- 状态冲突
- 资源不存在
- 下游业务码返回非成功但不属于系统异常

典型场景：

- 库存不足
- 订单已过期
- 支付单已关闭
- 请求正在处理中

### 5.4 `Error`

用于：

- 技术失败
- DB / RPC / Redis / MQ 等底层异常
- 状态迁移落库失败
- 事务失败
- 无法归类为正常业务拒绝的异常

要求：

- 必须带 `err`
- 必须带业务主键

### 5.5 `Fatal`

仅用于启动失败或服务无法继续运行的场景：

- 配置加载失败
- preflight 失败
- MySQL / Consul / Snowflake 初始化失败

禁止在普通请求链路里使用 `Fatal`。

## 6. 标准字段规范

### 6.1 通用字段

所有关键日志建议尽量包含：

- `trace_id`
- `span_id`
- `service`
- `env`
- `module`
- `action`
- `code`

说明：

- `trace_id / span_id` 优先从 `ctx` 自动注入
- `service / env` 由统一 logger 注入
- `module / action` 在业务层按需补充

### 6.2 用户与请求字段

根据场景补充：

- `user_id`
- `request_id`
- `http_path`
- `rpc_method`

### 6.3 订单域字段

- `order_id`
- `status`
- `from_status`
- `to_status`
- `cancel_reason`
- `payment_id`
- `payment_method`
- `payment_trade_no`
- `expire_at`

### 6.4 支付域字段

- `payment_id`
- `order_id`
- `payment_method`
- `payment_trade_no`
- `status`
- `from_status`
- `to_status`
- `expire_at`
- `closed_at`
- `fail_reason`

### 6.5 库存域字段

- `sku_id`
- `product_id`
- `biz_type`
- `biz_id`
- `quantity`
- `reserved_stock`
- `available_stock`

### 6.6 商品域字段

- `product_id`
- `sku_id`
- `creator_id`
- `status`

## 7. 必打日志场景

以下场景默认必须打日志。

### 7.1 请求入口失败

例如：

- 请求参数绑定失败
- 路径参数非法
- JWT 无效

建议级别：

- `warn`

### 7.2 业务状态拒绝

例如：

- 订单不是 `reserved` 却请求支付
- 支付单不是 `pending` 却请求确认成功
- 库存不足

建议级别：

- `warn`

### 7.3 下游 RPC 失败

例如：

- `order-service` 调 `inventory-service` timeout
- `payment-service` 调 `order-service` unavailable

建议级别：

- 技术错误：`error`
- 下游业务拒绝：`warn`

### 7.4 数据库失败

例如：

- `INSERT/UPDATE/SELECT` 报错
- 事务提交失败
- 唯一键冲突

建议级别：

- 技术失败：`error`
- 预期状态冲突：`warn`

### 7.5 关键状态迁移成功

例如：

- `pending -> succeeded`
- `reserved -> paid`
- `pending -> closed`

建议级别：

- `info`

## 8. 禁止事项

禁止：

- 只打印业务错误文案，不打印底层真实错误
- 在多个层重复打印完全相同的错误细节
- 打印无业务键的“系统内部错误”
- 打印整包请求 / 响应大对象
- 打印密码、token、完整支付回调原文、敏感配置
- 在高频热路径无节制打印 `info`

## 9. 推荐日志模板

### 9.1 业务拒绝

```go
logx.L(ctx).Warn("confirm payment success rejected by payment expiry",
    zap.Int64("payment_id", payment.PaymentID),
    zap.Int64("order_id", payment.OrderID),
    zap.Time("expire_at", payment.ExpireAt),
    zap.Time("now", s.now()),
    zap.Int32("code", errno.ErrPaymentExpired.Code),
)
```

### 9.2 下游 RPC 失败

```go
logx.L(ctx).Error("confirm order paid rpc failed",
    zap.Error(err),
    zap.String("downstream", "order-service"),
    zap.String("rpc_method", "ConfirmOrderPaid"),
    zap.Int64("payment_id", payment.PaymentID),
    zap.Int64("order_id", payment.OrderID),
)
```

### 9.3 Repository 技术错误

```go
logx.L(ctx).Error("update payment status failed",
    zap.Error(err),
    zap.Int64("payment_id", transition.PaymentID),
    zap.Int32("from_status", oldStatus),
    zap.Int32("to_status", transition.ToStatus),
)
```

### 9.4 状态迁移成功

```go
logx.L(ctx).Info("payment status transitioned",
    zap.Int64("payment_id", updated.PaymentID),
    zap.Int64("order_id", updated.OrderID),
    zap.Int32("from_status", PaymentStatusPending),
    zap.Int32("to_status", PaymentStatusSucceeded),
    zap.String("action", "confirm_success"),
)
```

## 10. 错误映射约定

统一要求：

- `BizError` 只负责对外业务码与文案
- 原始技术错误只进日志，不直接透出给前端
- 技术错误映射成统一业务错误前，必须先记录日志

建议模式：

1. repository / rpc client 打原始技术错误
2. service 层记录业务上下文与拒绝原因
3. handler 层只记录最终业务码和资源主键

## 11. 改造优先级建议

建议按以下顺序推进现有项目日志改造：

1. `payment-service`
   - 支付创建、支付确认、支付关闭、repository 状态迁移
2. `order-service`
   - 下单、取消、支付确认、超时关闭
3. `inventory-service`
   - 预占、释放、确认扣减
4. `gateway`
   - 入口 bind 失败、下游 RPC 失败、最终业务返回口径

## 12. 文档联动

后续如果日志规范发生变化，需要同步检查：

- [docs/service-development-spec.md](/Users/ruitong/GolandProjects/MeshCart/docs/service-development-spec.md)
- [docs/logging-tracing.md](/Users/ruitong/GolandProjects/MeshCart/docs/logging-tracing.md)
- [docs/microservice-governance.md](/Users/ruitong/GolandProjects/MeshCart/docs/microservice-governance.md)
- 各服务设计文档中的“运行与治理基线”部分
