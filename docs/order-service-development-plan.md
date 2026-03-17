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
