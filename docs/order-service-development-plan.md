# Order Service 开发计划

## 1. 目的

本文档用于定义 MeshCart `order-service` 的开发顺序、阶段目标和最终演进方向。

这份计划不追求一次性把订单服务做成完整电商系统，而是按当前仓库基础能力逐步推进，最终支持高并发场景下稳定下单。

## 2. 当前基线

当前仓库已经具备以下前置条件：

- `gateway`、`product-service`、`inventory-service` 已接入主链路
- `inventory-service` 已支持：
  - 库存可售校验
  - 库存初始化
  - 库存预占
  - 库存释放
  - 库存确认扣减
- `product-service` 已支持商品详情和 SKU 读取
- 分布式事务规范已经存在
- 日志、指标、链路追踪、服务发现、生命周期治理已具备基础模板

当前 `order-service` 仍处于骨架状态：

- 目录已存在
- `cmd`、`biz`、`dal` 已有占位结构
- 尚无完整 IDL、RPC bootstrap、业务表、下单主链路

## 3. 总体原则

订单服务的建设按以下原则推进：

1. 先做可闭环的最小交易主链路，再补复杂运营能力
2. 先保证数据语义正确，再做高并发优化
3. 先让 `order-service` 复用 `product-service` 和 `inventory-service` 的现有能力，不重复造域能力
4. 高并发方案必须建立在订单状态机、库存预占模型和幂等机制已经稳定的前提上

## 4. 最终目标

最终希望得到的订单服务具备以下能力：

- 用户可创建订单
- 订单创建时完成商品快照和库存预占
- 订单可取消，并释放库存
- 支付成功后可确认扣减库存并推进订单状态
- 支持超时关闭、重复请求幂等、状态流转控制
- 可在热点 SKU、高并发下通过 Redis + MQ + 异步编排进一步提升吞吐

## 5. 分阶段计划

### 阶段一：订单服务最小骨架

目标：

- 把 `order-service` 从目录骨架补成可运行服务

本阶段要完成：

- 定义 [idl/order.thrift](/Users/ruitong/GolandProjects/MeshCart/idl/order.thrift)
- 生成 `kitex_gen` 代码
- 补 `rpc/main.go`、`rpc/bootstrap`、`rpc/handler`
- 补配置文件和启动脚本
- 接入：
  - healthz / readyz / metrics
  - graceful shutdown
  - preflight
  - Consul 注册与直连回退
  - tracing / metrics / logging
- 补最小单测和启动验证

阶段完成标准：

- `order-service` 能独立启动
- 可被 `gateway` 或测试调用
- 生命周期和治理接入方式与 `user/product/inventory-service` 保持一致

当前实现设计：

- IDL
  - 已定义 [idl/order.thrift](/Users/ruitong/GolandProjects/MeshCart/idl/order.thrift)
  - 当前提供三个最小 RPC：
    - `CreateOrder`
    - `GetOrder`
    - `ListOrders`
- 运行入口
  - `services/order-service/rpc/main.go`
  - `services/order-service/rpc/bootstrap/bootstrap.go`
  - `services/order-service/script/start.sh`
- 配置
  - `services/order-service/config/config.go`
  - `services/order-service/config/order-service.local.yaml`
  - 当前沿用与 `cart-service`、`inventory-service` 一致的配置结构：
    - `mysql`
    - `migration`
    - `snowflake`
    - `timeout`
- 服务治理
  - 已接入：
    - `healthz / readyz / metrics`
    - preflight
    - graceful shutdown
    - Consul 注册与直连回退
    - tracing / metrics / logging
- 启动方式
  - 默认 RPC 服务名：`meshcart.order`
  - 默认监听地址：`127.0.0.1:8892`
  - 默认 admin 地址：`:9096`

本阶段设计取舍：

- 直接复用现有服务模板，不单独发明新的启动和治理结构
- 先保证 `order-service` 可运行、可注册、可观测
- 暂不在阶段一接入任何订单业务下游依赖，先把运行骨架站稳

### 阶段二：订单主数据与最小下单能力

目标：

- 建立订单服务自己的数据模型和最小下单能力

本阶段要完成：

- 设计并落库订单主表、订单项表
- 明确订单状态机最小集合，例如：
  - `pending`
  - `reserved`
  - `paid`
  - `cancelled`
  - `closed`
- 定义最小 RPC：
  - 创建订单
  - 查询订单详情
  - 查询订单列表
- 下单时落订单草稿和订单项快照

建议数据结构：

- `orders`
  - `order_id`
  - `user_id`
  - `status`
  - `total_amount`
  - `pay_amount`
  - `expire_at`
  - `created_at`
  - `updated_at`
- `order_items`
  - `order_id`
  - `sku_id`
  - `product_id`
  - `sku_title_snapshot`
  - `product_title_snapshot`
  - `sale_price_snapshot`
  - `quantity`
  - `subtotal_amount`

阶段完成标准：

- 订单可落库
- 订单详情可查询
- 订单项快照模型明确

当前实现设计：

- 订单状态
  - 当前已在代码中预留：
    - `pending`
    - `reserved`
    - `paid`
    - `cancelled`
    - `closed`
  - 阶段二实际创建订单时默认进入 `pending`
- 数据表
  - `orders`
    - `order_id`
    - `user_id`
    - `status`
    - `total_amount`
    - `pay_amount`
    - `expire_at`
    - `created_at`
    - `updated_at`
  - `order_items`
    - `id`
    - `order_id`
    - `product_id`
    - `sku_id`
    - `product_title_snapshot`
    - `sku_title_snapshot`
    - `sale_price_snapshot`
    - `quantity`
    - `subtotal_amount`
    - `created_at`
    - `updated_at`
- migration
  - `services/order-service/migrations/000001_create_orders.up.sql`
  - `services/order-service/migrations/000002_create_order_items.up.sql`
- repository 设计
  - `CreateWithItems`
    - 在单库事务中同时写订单主表和订单项表
  - `GetByOrderID`
    - 按 `user_id + order_id` 查询，并 preload 订单项
  - `ListByUserID`
    - 支持分页查询，并 preload 订单项
- service 设计
  - `CreateOrder`
    - 当前要求调用方直接提交商品快照字段
    - 服务内负责：
      - 参数校验
      - 订单 ID / 订单项 ID 生成
      - `subtotal_amount` 计算
      - `total_amount / pay_amount` 汇总
      - 默认 `expire_at` 设为创建后 30 分钟
  - `GetOrder`
    - 当前按“用户只能查自己的订单”约束实现
  - `ListOrders`
    - 当前按 `user_id` 分页查询

本阶段设计取舍：

- 先做“内部草稿下单”版本，不在阶段二接 `product-service` 和 `inventory-service`
- 订单项快照暂时由调用方直接提供，阶段三再切到服务内生成
- 先让订单域自己的表结构、状态字段、查询模型稳定下来，再接入库存预占

当前阶段二交付边界：

- 已完成：
  - 订单表
  - 订单项表
  - 创建订单草稿 RPC
  - 查询订单详情 RPC
  - 查询订单列表 RPC
  - repository / service 单测
- 暂未完成：
  - 商品快照实时校验
  - 库存预占
  - 取消订单
  - 支付成功确认扣减

### 阶段三：商品快照校验 + 库存预占闭环

目标：

- 打通真正的“下单”主链路

本阶段要完成：

1. `order-service` 调 `product-service.BatchGetSku`
2. 校验：
   - 商品在线
   - SKU active
   - SKU 与请求匹配
3. 生成订单商品快照
4. 调用 `inventory-service.ReserveSkuStocks`
5. 在订单侧落单并进入“已预占库存”状态

核心问题：

- 下单和库存预占之间要有清晰的一致性策略
- 至少保证：
  - 库存预占失败时，订单不能伪成功
  - 订单落库失败时，不能留下无法解释的已预占库存

当前推荐做法：

- 先使用最小分布式一致性收口
- 优先考虑订单创建与库存预占的统一事务边界
- 若当前阶段不立即引入完整 Saga，也至少要有明确补偿口径

阶段完成标准：

- 用户成功下单时，库存已被预占
- 任一步失败时，不留下不可解释的半成品订单或库存

当前实现设计：

- 下游依赖装配
  - `order-service` 当前在启动期初始化两个下游 RPC client：
    - `product-service`
    - `inventory-service`
  - 配置收口在：
    - `services/order-service/config/config.go`
    - `services/order-service/config/order-service.local.yaml`
  - client 封装放在：
    - `services/order-service/rpcclient/product`
    - `services/order-service/rpcclient/inventory`
- 订单入参调整
  - `CreateOrderRequest.items` 现在只要求：
    - `product_id`
    - `sku_id`
    - `quantity`
  - 商品标题、SKU 标题、成交价快照不再由调用方传入，而是由订单服务在创建过程中实时拉取并生成
- 商品校验链路
  - 第一步调用 `product-service.BatchGetSku`
    - 作用：
      - 校验请求里的 `sku_id` 是否存在
      - 校验 `sku_id -> product_id` 归属关系是否匹配
      - 校验 SKU 是否 `active`
  - 第二步按商品维度调用 `product-service.GetProductDetail`
    - 作用：
      - 校验商品是否 `online`
      - 获取商品标题快照
  - 当前阶段没有直接信任调用方提供的任何快照字段
- 快照生成规则
  - `product_title_snapshot`
    - 来自商品详情的 `product.title`
  - `sku_title_snapshot`
    - 来自 SKU 详情的 `sku.title`
  - `sale_price_snapshot`
    - 来自 SKU 当前 `sale_price`
  - `subtotal_amount`
    - 由订单服务按 `sale_price_snapshot * quantity` 计算
  - `total_amount / pay_amount`
    - 由订单服务按所有订单项汇总
- 库存预占链路
  - 订单服务按 `sku_id` 聚合数量后，调用：
    - `inventory-service.ReserveSkuStocks`
  - 预占的业务幂等键约定为：
    - `biz_type = order`
    - `biz_id = order_id`
  - 当前创建成功后订单直接进入：
    - `reserved`
  - 也就是说，阶段三已经把“草稿单”推进为“已完成库存预占的待支付订单”
- 一致性口径
  - 当前没有把订单创建和库存预占升级成完整 Saga
  - 当前采用的是“库存先预占，订单后落库”的最小补偿方案：
    1. 先完成商品校验与快照生成
    2. 调用库存预占
    3. 订单主表和订单项表落库
    4. 如果订单落库失败，立即调用 `ReleaseReservedSkuStocks` 做补偿释放
  - 这套口径至少保证：
    - 库存预占失败时，订单不会伪成功
    - 订单落库失败时，会立即做库存释放补偿
- 状态设计
  - 阶段二创建订单默认是 `pending`
  - 阶段三落地后，新的正常下单链路会直接生成：
    - `reserved`
  - `pending` 状态仍然保留，目的是兼容历史草稿单和后续更复杂状态机扩展
- 代码落点
  - IDL：
    - `idl/order.thrift`
  - service：
    - `services/order-service/biz/service/create_order.go`
    - `services/order-service/biz/service/helpers.go`
  - repository：
    - `services/order-service/biz/repository/repository.go`
  - bootstrap：
    - `services/order-service/rpc/bootstrap/bootstrap.go`
  - 下游 client：
    - `services/order-service/rpcclient/product/client.go`
    - `services/order-service/rpcclient/inventory/client.go`

本阶段设计取舍：

- 先用订单服务内编排 + 补偿释放收口，不在阶段三直接引入完整分布式事务框架
- 先用同步 RPC 生成商品快照，保证订单快照和当前商品状态一致
- 当前商品详情是按商品维度逐个拉取，而不是批量商品详情接口，优先保证边界清晰
- 预占幂等依赖 `inventory-service` 已落地的 `biz_type + biz_id` 机制，订单服务自身暂未引入独立下单请求幂等键

当前阶段三交付边界：

- 已完成：
  - 商品/SKU 实时校验
  - 订单快照服务内生成
  - 库存预占
  - 创建失败后的库存释放补偿
  - 订单创建成功即进入 `reserved`
- 暂未完成：
  - 支付成功后的确认扣减
  - 下单请求幂等键
  - 更重的一致性框架，比如 Saga/TCC

### 阶段四：取消订单、超时关闭、库存释放

目标：

- 让订单生命周期能够自洽

本阶段要完成：

- 用户主动取消订单
- 订单超时关闭
- 关闭订单时调用 `inventory-service.ReleaseReservedSkuStocks`
- 增加状态流转约束：
  - `pending/reserved -> cancelled`
  - `pending/reserved -> closed`
  - 已支付订单不能再取消

建议同时补：

- 订单超时任务扫描
- 取消原因字段
- 幂等取消控制

阶段完成标准：

- 未支付订单可被关闭
- 库存会被正确释放
- 重复取消不会造成重复释放

当前实现设计：

- 新增 RPC 接口
  - `CancelOrder`
    - 用于用户主动取消自己的订单
  - `CloseExpiredOrders`
    - 用于内部扫描并关闭已超时未支付订单
  - 协议定义在：
    - `idl/order.thrift`
- 订单模型补充
  - `orders` 表新增：
    - `cancel_reason`
  - 对应 migration：
    - `services/order-service/migrations/000003_add_order_cancel_reason.up.sql`
  - 作用：
    - 记录用户取消原因或系统超时关闭原因
    - 当前约定：
      - 用户取消写 `user_cancelled`
      - 超时关闭写 `order_expired`
- repository 能力补充
  - `GetByID`
    - 供内部编排和状态回查使用
  - `UpdateStatus`
    - 按“允许的旧状态集合 -> 新状态”做条件更新
    - 当前用于控制：
      - `pending/reserved -> cancelled`
      - `pending/reserved -> closed`
  - `ListExpiredOrders`
    - 查询 `expire_at <= now` 且状态仍是 `pending/reserved` 的订单
- 主动取消链路
  - 先按 `user_id + order_id` 查询订单
  - 状态规则：
    - `cancelled / closed`
      - 直接按幂等成功返回
    - `paid`
      - 直接拒绝
    - `pending / reserved`
      - 允许进入取消流程
  - 取消流程：
    1. 调 `inventory-service.ReleaseReservedSkuStocks`
    2. 成功后再把订单状态更新成 `cancelled`
    3. 同时写入 `cancel_reason`
  - 如果并发取消导致状态更新失败，会回查当前订单状态：
    - 已经是 `cancelled / closed` 则按幂等成功处理
    - 已经变成 `paid` 则返回“已支付订单不可取消”
- 超时关闭链路
  - `CloseExpiredOrders(limit)` 会：
    1. 扫描一批已过期且仍未支付的订单
    2. 对每笔订单调用库存释放
    3. 状态更新成 `closed`
    4. 写入 `cancel_reason = order_expired`
  - 当前没有在服务内直接起常驻定时器，而是先暴露内部 RPC，后续由 cron / job / 调度器驱动
- 释放库存的业务键
  - 主动取消和超时关闭都会复用阶段三创建订单时的同一个预占业务键：
    - `biz_type = order`
    - `biz_id = order_id`
  - 因为 `inventory-service` 的释放是幂等的，所以重复取消、重复超时扫描不会导致重复释放库存
- 一致性口径
  - 当前阶段四优先保证“库存释放幂等 + 订单状态条件更新”
  - 取消和超时关闭都采用：
    - 先释放库存
    - 再条件更新订单状态
  - 这样做的直接收益是：
    - 释放动作天然复用库存域幂等能力
    - 并发重复取消不会造成多次释放
  - 当前仍未引入更重的订单域事务日志，因此极端异常下仍以“可重试 + 库存侧幂等”作为恢复策略
- 代码落点
  - service：
    - `services/order-service/biz/service/cancel_order.go`
    - `services/order-service/biz/service/close_expired_orders.go`
  - repository：
    - `services/order-service/biz/repository/repository.go`
  - handler：
    - `services/order-service/rpc/handler/cancel_order.go`
    - `services/order-service/rpc/handler/close_expired_orders.go`

本阶段设计取舍：

- 先提供内部关闭扫描 RPC，不在阶段四直接实现常驻调度器
- 先用 `cancel_reason` 承接审计语义，不额外引入 `cancelled_at / closed_at` 等更多状态时间字段
- 先依赖库存域幂等释放，不在订单侧重复造一套库存释放幂等表

当前阶段四交付边界：

- 已完成：
  - 主动取消订单
  - 已支付订单禁止取消
  - 超时订单扫描关闭
  - 关闭/取消时释放库存
  - 重复取消的幂等返回
  - `cancel_reason` 字段落库
- 暂未完成：
  - 服务内定时扫描任务常驻执行
  - 订单状态流转日志表
  - 更强的订单域恢复任务与死信排障机制

### 阶段五：支付成功联动与确认扣减

目标：

- 把订单从“预占库存”推进到“真实成交”

本阶段要完成：

- 支付成功回调或内部支付确认接口
- 订单状态推进到 `paid`
- 调用 `inventory-service.ConfirmDeductReservedSkuStocks`
- 保证：
  - 重复支付通知不重复扣减
  - 已关闭订单不能再支付成功

本阶段建议同步补：

- 支付流水 ID / 外部支付单号
- 支付成功幂等键
- 订单状态流转日志

阶段完成标准：

- 支付成功后订单和库存状态一致
- 重复回调不会重复确认扣减

当前实现设计：

- 新增 RPC 接口
  - `ConfirmOrderPaid`
    - 作为当前阶段的内部支付确认入口
    - 当前不要求先落完整 `payment-service`
    - 先由内部回调方或后续支付服务调用
  - 协议定义在：
    - `idl/order.thrift`
- 订单模型补充
  - `orders` 表新增：
    - `payment_id`
    - `paid_at`
  - 对应 migration：
    - `services/order-service/migrations/000004_add_order_payment_fields.up.sql`
  - 作用：
    - 持久化外部支付流水号
    - 记录订单正式成交时间
- 支付确认链路
  - 当前 `ConfirmOrderPaid` 的执行顺序是：
    1. 读取订单
    2. 校验订单状态
    3. 调 `inventory-service.ConfirmDeductReservedSkuStocks`
    4. 把订单状态从 `reserved` 推进到 `paid`
    5. 写入 `payment_id`、`paid_at`
  - 库存确认扣减仍复用订单创建时的库存业务键：
    - `biz_type = order`
    - `biz_id = order_id`
- 状态约束
  - 当前允许：
    - `reserved -> paid`
  - 当前拒绝：
    - `closed -> paid`
    - `cancelled -> paid`
    - 非 `reserved` 状态的直接支付确认
  - 如果订单已经是 `paid`：
    - 且 `payment_id` 与本次请求一致，则按幂等成功返回
    - 且 `payment_id` 不一致，则判定为支付信息冲突
- 一致性口径
  - 阶段五的目标不是先做支付域，而是先把订单和库存的“成交收口”打通
  - 当前采用：
    - 先确认库存扣减
    - 再推进订单状态
  - 这样做的含义是：
    - 只有库存真正从“预占”转成“已扣减”后，订单才会进入 `paid`
    - 订单侧不会出现“订单已支付，但库存仍停留在预占”的正常成功返回
- 幂等口径
  - 当前支付确认天然用 `payment_id` 作为幂等键
  - 如果请求显式传 `request_id`，则优先用 `request_id`
  - 也就是说：
    - 同一个支付流水重复通知，不会重复确认扣减
    - 重复回调会直接返回当前已支付订单
- 代码落点
  - service：
    - `services/order-service/biz/service/confirm_order_paid.go`
  - handler：
    - `services/order-service/rpc/handler/confirm_order_paid.go`
  - repository：
    - `services/order-service/biz/repository/repository.go`
  - model：
    - `services/order-service/dal/model/model.go`

本阶段设计取舍：

- 先提供订单域内部支付确认 RPC，不在阶段五直接引入 `payment-service`
- 先把 `payment_id` 作为外部支付幂等键，不引入更复杂的支付流水表
- 当前只允许 `reserved -> paid`，不尝试兼容未预占库存的草稿单直接支付

当前阶段五交付边界：

- 已完成：
  - `ConfirmOrderPaid` RPC
  - 库存确认扣减
  - 订单状态推进到 `paid`
  - `payment_id / paid_at` 持久化
  - 重复支付通知幂等返回
- 暂未完成：
  - 独立支付流水表
  - 支付回调签名验签
  - 与未来 `payment-service` 的正式联动契约

### 阶段六：订单域事务、幂等与排障能力

目标：

- 让订单服务从“能跑”进入“可维护、可恢复”

本阶段要完成：

- 下单请求幂等
- 取消订单幂等
- 支付确认幂等
- 订单状态流转日志或事务记录表
- 关键链路日志补齐：
  - `order_id`
  - `user_id`
  - `sku_ids`
  - `inventory reservation biz_id`
- 补验收测试：
  - 下单成功
  - 下单失败
  - 取消释放库存
  - 支付确认扣减

阶段完成标准：

- 订单链路具备回归测试
- 关键异常有排障入口
- 重复请求不会制造脏数据

当前实现设计：

- 幂等记录表
  - 新增：
    - `order_action_records`
  - 对应 migration：
    - `services/order-service/migrations/000005_create_order_action_records.up.sql`
  - 表的核心字段：
    - `action_type`
    - `action_key`
    - `order_id`
    - `user_id`
    - `status`
    - `error_message`
  - 约束：
    - `action_type + action_key` 唯一
- 状态流转日志表
  - 新增：
    - `order_status_logs`
  - 对应 migration：
    - `services/order-service/migrations/000006_create_order_status_logs.up.sql`
  - 记录字段包括：
    - `order_id`
    - `from_status`
    - `to_status`
    - `action_type`
    - `reason`
    - `external_ref`
- 当前幂等覆盖范围
  - 下单：
    - `CreateOrderRequest.request_id`
    - 若同一个 `request_id` 已成功，会直接返回已创建订单
  - 取消：
    - `CancelOrderRequest.request_id`
    - 若同一个 `request_id` 已成功，会直接返回最终订单状态
  - 支付确认：
    - 默认使用 `payment_id`
    - 如果显式传 `request_id`，则优先使用 `request_id`
- 动作状态
  - `order_action_records.status` 当前约定：
    - `pending`
    - `succeeded`
    - `failed`
  - 同一个幂等键如果仍处于 `pending`，服务会返回“请求正在处理中，请稍后重试”
- 状态日志写入策略
  - 创建订单时：
    - 在 `CreateWithItems` 的本地事务里写入初始状态日志
  - 取消 / 超时关闭 / 支付确认时：
    - 在订单状态迁移事务里同时写入状态日志
  - 这样至少能保证：
    - 订单状态变化和日志落库在同一个本地事务里
- repository 设计补充
  - `TransitionStatus`
    - 统一处理状态条件更新
    - 支持同时落 `order_status_logs`
  - `GetActionRecord / CreateActionRecord / MarkActionRecordSucceeded / MarkActionRecordFailed`
    - 用于动作级幂等控制
- 当前排障入口
  - `order_action_records`
    - 用于查看某个幂等键是否已执行、是否失败、失败文案是什么
  - `order_status_logs`
    - 用于还原订单经历过哪些状态变更
  - 结合 `orders.payment_id`、`orders.cancel_reason`
    - 已能满足当前阶段的基础回溯
- 测试补充
  - repository 层已覆盖：
    - 状态迁移并写日志
    - 动作记录创建与成功回写
  - service 层已覆盖：
    - 下单成功
    - 库存不足
    - 下单失败后释放库存
    - 取消订单
    - 关闭过期订单
    - 支付确认成功
    - 支付确认幂等
    - 支付流水冲突

本阶段设计取舍：

- 先用数据库表做动作幂等，不引入 Redis 分布式锁
- 先把状态日志压在订单域本库内，不引入单独审计服务
- 当前对 `failed` 动作记录不做自动恢复，只作为排障入口和幂等保护
- 当前没有做完整事务恢复任务，优先保证可观察、可重试、可回查

当前阶段六交付边界：

- 已完成：
  - 下单请求幂等
  - 取消请求幂等
  - 支付确认幂等
  - 动作记录表
  - 状态流转日志表
  - repository / service 测试补齐
- 暂未完成：
  - 自动重试或补偿任务
  - 失败动作重放控制台
  - 更完整的事务恢复扫描器

### 阶段七：网关接入与用户侧订单接口

目标：

- 对外提供真正可用的订单 HTTP 能力

本阶段要完成：

- `gateway` 接入订单 RPC client
- HTTP 接口设计：
  - 提交订单
  - 订单详情
  - 我的订单列表
  - 取消订单
- 用户鉴权与“仅可读写自己的订单”控制
- 管理端订单查询预留

阶段完成标准：

- 用户可通过 `gateway` 下单、查单、取消订单
- 权限边界清晰

当前实现设计：

- gateway 接入
  - 已新增 `gateway -> order-service` 的 RPC client：
    - `gateway/rpc/order/client.go`
  - 已在网关配置与依赖容器中完成装配：
    - `gateway/config/config.go`
    - `gateway/internal/svc/service_context.go`
  - 当前新增的配置组是：
    - `OrderRPC`
- 对外 HTTP 路由
  - 当前用户侧订单接口都挂在：
    - `/api/v1/orders`
  - 路由注册在：
    - `gateway/internal/handler/order/routes.go`
  - 已开放接口：
    - `POST /api/v1/orders`
    - `GET /api/v1/orders`
    - `GET /api/v1/orders/:order_id`
    - `POST /api/v1/orders/:order_id/cancel`
  - 这些接口都要求 JWT 登录态
- handler / logic / types 落点
  - handler：
    - `gateway/internal/handler/order/`
  - logic：
    - `gateway/internal/logic/order/`
  - HTTP 类型：
    - `gateway/internal/types/order.go`

接口文档：

- `POST /api/v1/orders`
  - 作用：
    - 提交订单
  - 认证：
    - 需要 JWT
  - 请求体：
    - `request_id: string`
      - 可选
      - 用于透传到订单服务做下单幂等
    - `items: []`
      - 至少一项
    - `items[].product_id: int64`
    - `items[].sku_id: int64`
    - `items[].quantity: int32`
  - 示例请求：
    ```json
    {
      "request_id": "req-order-1001",
      "items": [
        {
          "product_id": 2001,
          "sku_id": 3001,
          "quantity": 2
        }
      ]
    }
    ```
  - 成功响应 data：
    - `order_id`
    - `user_id`
    - `status`
    - `total_amount`
    - `pay_amount`
    - `expire_at`
    - `cancel_reason`
    - `payment_id`
    - `paid_at`
    - `items`
  - 关键语义：
    - 只允许为当前登录用户创建订单
    - 网关不会信任客户端传商品快照，实际快照由订单服务内部生成
    - 创建成功后的订单当前会直接进入 `reserved`

- `GET /api/v1/orders`
  - 作用：
    - 查询当前登录用户的订单列表
  - 认证：
    - 需要 JWT
  - 查询参数：
    - `page: int32`
      - 可选，默认 `1`
    - `page_size: int32`
      - 可选，默认 `20`
      - 当前最大会被网关裁剪到 `100`
  - 成功响应 data：
    - `orders`
    - `total`
  - `orders[]` 字段与单笔订单详情一致
  - 关键语义：
    - 只能查询自己的订单
    - 不支持跨用户查询

- `GET /api/v1/orders/:order_id`
  - 作用：
    - 查询当前登录用户的单笔订单详情
  - 认证：
    - 需要 JWT
  - 路径参数：
    - `order_id: int64`
  - 成功响应 data：
    - `order_id`
    - `user_id`
    - `status`
    - `total_amount`
    - `pay_amount`
    - `expire_at`
    - `cancel_reason`
    - `payment_id`
    - `paid_at`
    - `items`
  - `items[]` 字段：
    - `item_id`
    - `order_id`
    - `product_id`
    - `sku_id`
    - `product_title_snapshot`
    - `sku_title_snapshot`
    - `sale_price_snapshot`
    - `quantity`
    - `subtotal_amount`
  - 关键语义：
    - 当前依赖订单服务按 `user_id + order_id` 做资源隔离

- `POST /api/v1/orders/:order_id/cancel`
  - 作用：
    - 取消当前登录用户自己的订单
  - 认证：
    - 需要 JWT
  - 路径参数：
    - `order_id: int64`
  - 请求体：
    - `request_id: string`
      - 可选
      - 用于透传到订单服务做取消幂等
    - `cancel_reason: string`
      - 可选
  - 示例请求：
    ```json
    {
      "request_id": "req-cancel-2001",
      "cancel_reason": "changed_mind"
    }
    ```
  - 成功响应 data：
    - 返回取消后的订单对象
  - 关键语义：
    - 已支付订单不可取消
    - 重复取消会按订单服务当前状态做幂等返回

当前未对外暴露的订单能力：

- `ConfirmOrderPaid`
  - 当前仍只保留在 `order-service` 内部 RPC
  - 不通过 `gateway` 对外开放
- `CloseExpiredOrders`
  - 当前也只作为内部扫描 RPC 使用
  - 不对用户暴露 HTTP 接口

代码落点：

- 网关 RPC client：
  - `gateway/rpc/order/client.go`
- HTTP handler：
  - `gateway/internal/handler/order/create.go`
  - `gateway/internal/handler/order/list.go`
  - `gateway/internal/handler/order/get.go`
  - `gateway/internal/handler/order/cancel.go`
- 业务 logic：
  - `gateway/internal/logic/order/create.go`
  - `gateway/internal/logic/order/list.go`
  - `gateway/internal/logic/order/get.go`
  - `gateway/internal/logic/order/cancel.go`
- 类型定义：
  - `gateway/internal/types/order.go`

本阶段设计取舍：

- 先只开放用户侧最小闭环接口，不把支付确认和过期关闭暴露到 HTTP
- 先沿用订单服务的 `user_id` 资源隔离，不额外在网关重复做复杂资源查询
- 先让 `gateway` 只是代理与轻编排层，不在网关重写订单状态机

当前阶段七交付边界：

- 已完成：
  - 订单 RPC client
  - 提交订单 HTTP 接口
  - 我的订单列表 HTTP 接口
  - 订单详情 HTTP 接口
  - 取消订单 HTTP 接口
  - JWT 鉴权接入
  - gateway 订单逻辑测试
- 暂未完成：
  - 管理端订单查询
  - 支付确认 HTTP 接口
  - 内部订单关闭调度入口的网关暴露

### 阶段八：高并发增强版订单服务

目标：

- 支持热点商品、高并发下单场景

本阶段不是重写订单服务，而是在前面稳定模型基础上加一层高并发能力。

本阶段要完成：

- Redis 作为热点库存前置拦截层
- MQ 作为削峰与异步串行化层
- 订单服务按“同步受理 + 异步确认”或“令牌成功后快速落单”模式演进
- 建立异步状态回写机制
- 增加订单创建结果查询或回查机制

推荐职责拆分：

- Redis
  - 热点 SKU 额度缓存
  - 原子预减
  - 快速拒绝超卖请求
- MQ
  - 平滑流量洪峰
  - 异步驱动订单创建和库存正式预占
- `order-service`
  - 订单状态机
  - 幂等控制
  - 结果回写
- `inventory-service`
  - 最终库存账本
  - 预占、释放、确认扣减真相源

必须坚持的约束：

- Redis 不能成为订单和库存的唯一事实来源
- 不能跳过 `inventory-service` 直接把 Redis 结果视为最终扣减成功
- MQ 消费必须有幂等机制
- 高并发优化不能破坏订单状态机和库存状态语义

阶段完成标准：

- 热点订单流量不再直接打穿数据库
- 超卖风险由 Redis 前置拦截 + 库存真相源双重控制
- 异步链路具备可重试、可回查、可排障能力

## 6. 推荐推进顺序

建议严格按以下顺序推进：

1. 阶段一：先让 `order-service` 成为真正可运行服务
2. 阶段二：补订单主表和订单项表
3. 阶段三：打通商品快照校验 + 库存预占
4. 阶段四：补取消和超时关闭
5. 阶段五：补支付成功和确认扣减
6. 阶段六：补幂等、事务记录和测试体系
7. 阶段七：接 `gateway`，开放用户侧下单接口
8. 阶段八：最后做高并发增强

## 7. 当前建议

如果下一步立刻开工，推荐从“阶段一 + 阶段二”开始，不要直接进入高并发设计。

最先落地的内容应是：

- 订单 IDL
- 订单表结构
- 订单服务 bootstrap
- 创建订单草稿 RPC
- 查询订单详情 RPC

原因：

- 现在库存服务已经准备好了
- 但订单服务还没有承接这些能力的主体
- 高并发优化必须建立在订单状态机和库存协作已经稳定的前提上
