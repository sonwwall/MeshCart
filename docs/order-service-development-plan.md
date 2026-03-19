# Order Service 开发与设计文档

## 1. 目的

本文档用于说明 MeshCart `order-service` 当前已经实现的能力、核心业务设计、数据库模型、RPC / HTTP 接口，以及后续继续推进的方向。

本文档不再按历史开发顺序堆叠阶段内容，而是优先收口当前已完成基线；未来计划统一放在后半部分。

## 2. 当前定位

`order-service` 当前已经不是骨架服务，而是订单主链路的核心编排服务，负责：

- 接收下单请求
- 拉取商品与 SKU 信息生成订单快照
- 调用 `inventory-service` 做库存预占
- 订单取消时释放库存
- 支付确认时确认扣减库存
- 提供订单详情、订单列表、取消订单、支付确认、关闭超时订单等 RPC 能力

当前服务边界：

- `product-service`
  - 负责商品和 SKU 真相数据
- `inventory-service`
  - 负责库存真相账本、预占、释放、确认扣减
- `order-service`
  - 负责订单状态机、订单快照、订单幂等、订单状态流转日志
- `gateway`
  - 负责对外 HTTP 暴露、用户鉴权、轻编排和响应裁剪

## 3. 当前已实现能力

当前已经完成的订单能力包括：

- `order-service` RPC 服务骨架、bootstrap、配置、Consul 注册、healthz/readyz/metrics、优雅停机
- 订单主表 `orders`
- 订单项表 `order_items`
- 动作幂等表 `order_action_records`
- 状态流转日志表 `order_status_logs`
- `CreateOrder`
  - 商品/SKU 实时校验
  - 订单快照生成
  - 库存预占
  - 订单落库
  - 落库失败时库存释放补偿
- `GetOrder`
- `ListOrders`
- `CancelOrder`
  - 取消时释放库存
- `CloseExpiredOrders`
  - 关闭超时未支付订单并释放库存
- `ConfirmOrderPaid`
  - 确认扣减库存
  - 推进订单到 `paid`
  - 落支付方式、支付渠道流水号等支付关联字段
- 下单、取消、支付确认三类幂等控制
- 订单状态流转日志和动作记录
- `gateway` 侧用户订单 HTTP 接口

## 4. 核心业务设计

### 4.1 订单状态机

当前订单状态定义在 [service.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/service.go)：

- `1 = pending`
  - 预留状态，当前主链路正常下单不会停留在这里
- `2 = reserved`
  - 已完成库存预占，等待支付
- `3 = paid`
  - 已完成支付确认和库存确认扣减
- `4 = cancelled`
  - 用户主动取消
- `5 = closed`
  - 超时关闭

当前主要状态流转：

- `reserved -> paid`
- `pending/reserved -> cancelled`
- `pending/reserved -> closed`

当前明确禁止：

- `paid -> cancelled`
- `closed -> paid`
- `cancelled -> paid`

### 4.2 下单链路

当前下单主链路实现位于 [create_order.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/create_order.go)。

执行顺序：

1. 校验请求和幂等键
2. 调 `product-service.BatchGetSku`
   - 校验 `sku_id` 是否存在
   - 校验 `sku_id -> product_id` 归属关系
   - 校验 SKU 是否可售
3. 调 `product-service.GetProductDetail`
   - 校验商品是否在线
   - 获取商品标题
4. 生成订单项快照
   - `product_title_snapshot`
   - `sku_title_snapshot`
   - `sale_price_snapshot`
5. 汇总金额
   - `subtotal_amount`
   - `total_amount`
   - `pay_amount`
6. 调 `inventory-service.ReserveSkuStocks`
   - `biz_type = order`
   - `biz_id = order_id`
7. 订单主表和订单项表落库
8. 若订单落库失败，立即调 `ReleaseReservedSkuStocks` 做补偿释放
9. 订单成功创建后直接进入 `reserved`

这条链路当前采用的是“订单服务内同步编排 + 最小补偿”方案，还没有升级成独立 Saga/TCC。

### 4.3 取消订单与超时关闭

当前实现位于：

- [cancel_order.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/cancel_order.go)
- [close_expired_orders.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/close_expired_orders.go)

处理原则：

- 对 `pending/reserved` 订单允许取消或关闭
- 先释放库存，再条件更新订单状态
- 库存释放继续复用下单时的库存业务键：
  - `biz_type = order`
  - `biz_id = order_id`

当前语义：

- 用户取消：
  - 状态改成 `cancelled`
  - `cancel_reason` 默认为 `user_cancelled`
- 系统超时关闭：
  - 状态改成 `closed`
  - `cancel_reason` 为 `order_expired`
- `orders.expire_at` 是订单级过期时间
  - 它控制“这笔交易整体是否还允许成交”
  - 即使支付单已经创建，只要订单已经到或超过 `expire_at`，订单侧仍然必须拒绝支付成功
- 当前支付域已经有自己的支付单过期时间：
  - 订单过期负责关闭整笔交易
  - 支付单过期只负责关闭某一次支付尝试
  - 两者不互相替代

### 4.4 支付确认

当前实现位于 [confirm_order_paid.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/confirm_order_paid.go)。

执行顺序：

1. 校验请求与支付幂等键
2. 查询订单并校验状态必须为 `reserved`
3. 调 `inventory-service.ConfirmDeductReservedSkuStocks`
4. 条件更新订单状态：
   - `reserved -> paid`
5. 写入：
   - `payment_id`
   - `payment_method`
   - `payment_trade_no`
   - `paid_at`

当前语义：

- 同一个 `payment_id` 重复通知按幂等成功处理
- 若传入 `request_id`，支付确认优先按 `request_id` 做动作幂等
- 订单即使还没被后台任务关闭，只要当前时间已经到或超过 `expire_at`，也不允许支付成功
- 已支付订单要求 `payment_id` 一致；如果 `payment_method` / `payment_trade_no` 也已落库，则重复通知时也必须一致
- 已关闭或已取消订单不允许支付成功
- 已支付订单如果支付信息不一致，返回支付冲突

### 4.5 幂等与排障

当前实现位于：

- [helpers.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/service/helpers.go)
- [repository.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/biz/repository/repository.go)

当前幂等覆盖：

- 下单：
  - `CreateOrderRequest.request_id`
- 取消订单：
  - `CancelOrderRequest.request_id`
- 支付确认：
  - 默认使用 `payment_id`
  - 若传 `request_id`，优先使用 `request_id`

支付确认边界约定：

- `payment_id`
  - 订单域关联的内部支付单号
- `payment_method`
  - 支付方式标识，例如 `mock` / `alipay` / `wechat`
- `payment_trade_no`
  - 外部支付渠道流水号
- `paid_at`
  - 支付服务确认的支付成功时间；如果未传，则默认使用订单服务本地时间

订单与支付单超时关系：

- 订单过期时间由 `orders.expire_at` 表示
- 它是交易级总开关，不会因为支付单已创建就失效
- 即使 `payment-service` 已经有自己的支付单过期时间，订单侧仍继续保留订单过期校验
- 支付成功时，必须同时满足：
  - 订单未过期
  - 订单状态允许支付成功

动作状态：

- `pending`
- `succeeded`
- `failed`

当前排障入口：

- `order_action_records`
  - 看某个动作是否执行、是否失败、失败文案是什么
- `order_status_logs`
  - 看订单经历过哪些状态流转

当前日志入口：

- `gateway/internal/logic/order/`
  - `create`
  - `get`
  - `list`
  - `cancel`
- `services/order-service/biz/service/`
  - `create_order`
  - `cancel_order`
  - `confirm_order_paid`
  - `close_expired_orders`
- `services/order-service/biz/repository/repository.go`

当前日志约定：

- Gateway
  - 下游 transport error 记 `Error`
  - 下游业务错误记 `Warn`
  - nil order / nil response 记 `Error`
- order-service service
  - 记录 `start / reject / completed`
  - 会明确记录“库存预占失败”“订单已过期”“订单状态不允许支付”等原因
- repository
  - 记录原始 DB 错误
  - 记录状态迁移冲突、动作记录更新失败

## 5. 数据库设计

### 5.1 订单主表 `orders`

定义见：

- [model.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/dal/model/model.go)
- [000001_create_orders.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/order-service/migrations/000001_create_orders.up.sql)
- [000003_add_order_cancel_reason.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/order-service/migrations/000003_add_order_cancel_reason.up.sql)
- [000004_add_order_payment_fields.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/order-service/migrations/000004_add_order_payment_fields.up.sql)
- [000007_add_order_payment_detail_fields.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/order-service/migrations/000007_add_order_payment_detail_fields.up.sql)

字段说明：

- `order_id`
  - 订单主键，雪花 ID
- `user_id`
  - 订单所属用户
- `status`
  - 订单状态
- `total_amount`
  - 订单总金额
- `pay_amount`
  - 实际应付金额
- `expire_at`
  - 订单过期时间
- `cancel_reason`
  - 取消或关闭原因
- `payment_id`
  - 订单域关联的内部支付单号
- `payment_method`
  - 支付方式标识，例如 `mock` / `alipay` / `wechat`
- `payment_trade_no`
  - 外部支付渠道流水号
- `paid_at`
  - 支付成功时间
- `created_at`
  - 创建时间
- `updated_at`
  - 更新时间

索引说明：

- `idx_orders_user_id_status`
  - 支撑按用户分页查订单
- `idx_orders_status_expire_at`
  - 支撑扫描超时订单
- `idx_orders_updated_at`
  - 支撑更新时间维度排查和排序

### 5.2 订单项表 `order_items`

定义见：

- [model.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/dal/model/model.go)
- [000002_create_order_items.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/order-service/migrations/000002_create_order_items.up.sql)

字段说明：

- `id`
  - 订单项主键，雪花 ID
- `order_id`
  - 所属订单
- `product_id`
  - 商品 ID
- `sku_id`
  - SKU ID
- `product_title_snapshot`
  - 下单时商品标题快照
- `sku_title_snapshot`
  - 下单时 SKU 标题快照
- `sale_price_snapshot`
  - 下单时成交单价快照
- `quantity`
  - 购买数量
- `subtotal_amount`
  - 当前订单项小计金额
- `created_at`
  - 创建时间
- `updated_at`
  - 更新时间

设计意图：

- 订单项快照一旦写入，不再依赖商品域实时读取
- 订单详情始终以订单域自己的快照为准

### 5.3 动作幂等表 `order_action_records`

定义见：

- [model.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/dal/model/model.go)
- [000005_create_order_action_records.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/order-service/migrations/000005_create_order_action_records.up.sql)

字段说明：

- `id`
  - 记录主键，雪花 ID
- `action_type`
  - 动作类型，例如：
    - `create`
    - `cancel`
    - `pay_confirm`
- `action_key`
  - 幂等键
- `order_id`
  - 关联订单 ID
- `user_id`
  - 关联用户 ID
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

### 5.4 状态流转日志表 `order_status_logs`

定义见：

- [model.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/dal/model/model.go)
- [000006_create_order_status_logs.up.sql](/Users/ruitong/GolandProjects/MeshCart/services/order-service/migrations/000006_create_order_status_logs.up.sql)

字段说明：

- `id`
  - 日志主键，雪花 ID
- `order_id`
  - 关联订单 ID
- `from_status`
  - 变更前状态
- `to_status`
  - 变更后状态
- `action_type`
  - 触发动作，例如：
    - `create`
    - `cancel`
    - `close_expired`
    - `pay_confirm`
- `reason`
  - 变更原因
- `external_ref`
  - 外部引用，例如 `payment_trade_no` 或 `payment_id`
- `created_at`
  - 创建时间

设计意图：

- 状态变化和状态日志同事务落库
- 方便还原订单状态演进过程

## 6. RPC 设计

IDL 定义见 [order.thrift](/Users/ruitong/GolandProjects/MeshCart/idl/order.thrift)。

当前 `OrderService` 已提供这些 RPC：

- `CreateOrder`
- `CancelOrder`
- `ConfirmOrderPaid`
- `GetOrder`
- `ListOrders`
- `CloseExpiredOrders`

### 6.1 `CreateOrder`

作用：

- 为指定用户创建订单

请求字段：

- `user_id`
- `items[]`
  - `product_id`
  - `sku_id`
  - `quantity`
- `request_id`
  - 可选
  - 下单幂等键

返回字段：

- `order`
- `base`

关键语义：

- 商品快照由订单服务内部生成，不信任调用方传快照
- 创建成功后的订单当前直接进入 `reserved`

### 6.2 `CancelOrder`

作用：

- 取消指定订单

请求字段：

- `user_id`
- `order_id`
- `cancel_reason`
- `request_id`

关键语义：

- 只允许取消自己的订单
- 已支付订单不可取消
- 重复取消按幂等返回

### 6.3 `ConfirmOrderPaid`

作用：

- 内部支付确认入口

请求字段：

- `order_id`
- `payment_id`
- `request_id`
- `payment_method`
- `payment_trade_no`
- `paid_at`

关键语义：

- 当前不对外暴露给用户 HTTP
- 支付成功后会确认扣减库存并推进订单状态
- 当前把支付边界收口成一条内部 RPC：
  - `payment_id` 作为内部支付单关联号
  - `payment_trade_no` 作为外部渠道流水号
  - `payment_method` 作为支付方式标识
- 未来 `payment-service` 接入后，仍继续复用这条订单侧确认语义

### 6.4 `GetOrder`

作用：

- 查询单笔订单详情

请求字段：

- `user_id`
- `order_id`

关键语义：

- 当前按 `user_id + order_id` 做资源隔离

### 6.5 `ListOrders`

作用：

- 查询用户订单列表

请求字段：

- `user_id`
- `page`
- `page_size`

关键语义：

- RPC 当前仍返回完整 `Order` 列表
- HTTP 层会把它裁剪成摘要视图

### 6.6 `CloseExpiredOrders`

作用：

- 扫描并关闭一批超时未支付订单

请求字段：

- `limit`

关键语义：

- 当前作为内部扫描 RPC 使用
- 不对普通用户开放 HTTP

## 7. HTTP 接口设计

当前对外 HTTP 接口由 `gateway` 暴露，相关实现位于：

- [routes.go](/Users/ruitong/GolandProjects/MeshCart/gateway/internal/handler/order/routes.go)
- [create.go](/Users/ruitong/GolandProjects/MeshCart/gateway/internal/handler/order/create.go)
- [list.go](/Users/ruitong/GolandProjects/MeshCart/gateway/internal/handler/order/list.go)
- [get.go](/Users/ruitong/GolandProjects/MeshCart/gateway/internal/handler/order/get.go)
- [cancel.go](/Users/ruitong/GolandProjects/MeshCart/gateway/internal/handler/order/cancel.go)

当前已开放：

- `POST /api/v1/orders`
- `GET /api/v1/orders`
- `GET /api/v1/orders/:order_id`
- `POST /api/v1/orders/:order_id/cancel`

这些接口都要求 JWT。

### 7.1 `POST /api/v1/orders`

作用：

- 提交订单

请求体：

- `request_id`
- `items[]`
  - `product_id`
  - `sku_id`
  - `quantity`

成功响应 `data`：

- 完整订单对象
- 包含完整 `items`

### 7.2 `GET /api/v1/orders`

作用：

- 查询当前登录用户的订单列表

查询参数：

- `page`
- `page_size`

成功响应 `data`：

- `orders`
- `total`

`orders[]` 字段：

- `order_id`
- `user_id`
- `status`
- `total_amount`
- `pay_amount`
- `expire_at`
- `cancel_reason`
- `payment_id`
- `payment_method`
- `paid_at`
- `item_count`

关键语义：

- 列表接口当前只返回摘要
- 不返回完整 `items`

### 7.3 `GET /api/v1/orders/:order_id`

作用：

- 查询当前登录用户的单笔订单详情

成功响应 `data`：

- 完整订单对象
- 包含完整 `items`
- 包含支付关联字段：
  - `payment_id`
  - `payment_method`
  - `payment_trade_no`
  - `paid_at`

### 7.4 `POST /api/v1/orders/:order_id/cancel`

作用：

- 取消当前登录用户自己的订单

请求体：

- `request_id`
- `cancel_reason`

成功响应 `data`：

- 返回取消后的完整订单对象

### 7.5 当前未开放的 HTTP 能力

当前仍只保留为内部 RPC：

- `ConfirmOrderPaid`
- `CloseExpiredOrders`

## 8. 运行与治理基线

`order-service` 当前已经接入：

- `healthz / readyz / metrics`
- preflight
- graceful shutdown
- Consul 注册与发现
- tracing / metrics / logging
- 下游 RPC timeout
- 商品读 RPC 有限重试

关键落点：

- 启动与 bootstrap：
  - [main.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/rpc/main.go)
  - [bootstrap.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/rpc/bootstrap/bootstrap.go)
- 配置：
  - [config.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/config/config.go)
  - [order-service.local.yaml](/Users/ruitong/GolandProjects/MeshCart/services/order-service/config/order-service.local.yaml)
- 下游 client：
  - [client.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/rpcclient/product/client.go)
  - [client.go](/Users/ruitong/GolandProjects/MeshCart/services/order-service/rpcclient/inventory/client.go)

当前重试口径：

- `product-service` 读接口：
  - `GetProductDetail`
  - `BatchGetSKU`
  - 已启用一次有限重试
- `inventory-service` 的写接口：
  - `ReserveSkuStocks`
  - `ReleaseReservedSkuStocks`
  - `ConfirmDeductReservedSkuStocks`
  - 不自动重试

## 9. 测试情况

当前已补测试包括：

- repository 测试
  - 创建订单
  - 状态迁移
  - 动作记录
  - 状态日志
- service 测试
  - 下单成功
  - 下单失败
  - 库存不足
  - 下单失败后释放库存
  - 取消订单
  - 关闭过期订单
  - 支付确认成功
  - 支付确认幂等
  - 支付冲突
- RPC handler 测试
- `gateway` 侧订单 logic 测试
- `gateway` 侧订单 RPC client 测试

## 10. 当前边界与未完成部分

当前还没有完成的主要是：

- 独立 `payment-service` 正式接入
- 支付 HTTP 对外接口
- 支付单主表与支付流水表
- 常驻超时关闭调度器
- 自动补偿 / 自动重试 / 恢复任务
- 管理端订单查询接口
- 更完整的订单审计与操作后台
- 高并发异步下单架构

## 11. 后续推进计划

### 11.1 近期计划

优先顺序建议：

1. 接 `payment-service`
   - 让 `payment-service` 负责支付单创建、支付回调和结果确认
   - `order-service` 继续保留 `ConfirmOrderPaid` 作为订单状态推进边界
2. 增加常驻超时关闭调度
   - 让 `CloseExpiredOrders` 真正进入日常运行
3. 补管理端订单查询
   - 后台可查订单列表和详情
4. 补更完整的排障入口
   - 失败动作回查
   - 更清晰的状态演进可视化

### 11.2 中期计划

- 更完整的事务恢复任务
- 更丰富的订单状态时间字段
- 支付单主表与支付流水表
- 管理端操作审计
- 与库存、支付的联调验收链路

### 11.3 高并发演进计划

当前订单服务的高并发演进方向保持不变：

- Redis
  - 热点库存前置拦截
  - 热点额度缓存
- MQ
  - 削峰和异步串行化
- `order-service`
  - 负责订单状态机、幂等、结果回写
- `inventory-service`
  - 继续作为库存真相账本

必须坚持的约束：

- Redis 不能成为最终订单和库存事实来源
- 不能跳过 `inventory-service`
- MQ 消费必须幂等
- 高并发优化不能破坏当前订单状态机和库存状态语义

## 12. 推荐阅读

- [docs/inventory-service-design.md](/Users/ruitong/GolandProjects/MeshCart/docs/inventory-service-design.md)
- [docs/product-service-design.md](/Users/ruitong/GolandProjects/MeshCart/docs/product-service-design.md)
- [docs/microservice-governance.md](/Users/ruitong/GolandProjects/MeshCart/docs/microservice-governance.md)
