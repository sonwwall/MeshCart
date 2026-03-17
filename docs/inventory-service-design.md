# Inventory Service 设计文档

## 1. 目的

本文档说明 `inventory-service` 当前已经落地的设计与实现基线，并整理后续仍值得继续推进的演进方向。

本文档遵循 [service-development-spec.md](./service-development-spec.md) 中的通用服务规范，作为库存服务继续演进时的设计依据和实现参照。

## 2. 设计依据

- [service-development-spec.md](./service-development-spec.md)
- [microservice-governance.md](./microservice-governance.md)
- [evolution-plan.md](./evolution-plan.md)
- [product-service-design.md](./product-service-design.md)
- [cart-service-design.md](./cart-service-design.md)
- [distributed-transaction-design.md](./distributed-transaction-design.md)
- [error-code.md](./error-code.md)

## 3. 当前定位

### 3.1 当前目标

`inventory-service` 当前目标是为 MeshCart 提供统一的库存域能力，覆盖以下三类场景：

- 商品创建或更新后的库存初始化
- 商品和购物车链路中的库存可售校验
- 订单语义下的库存预占、释放、确认扣减

### 3.2 当前负责

- 维护 SKU 维度库存主数据
- 提供单个 SKU 和批量 SKU 的库存查询
- 提供库存是否足够的只读校验
- 提供商品创建后的库存初始化
- 提供按业务单号的库存预占
- 提供按业务单号的库存释放
- 提供按业务单号的库存确认扣减
- 提供后台库存调整能力

### 3.3 当前不负责

- 商品是否在线、SKU 是否 active
- 多仓、多库位、仓间调拨
- 采购入库、人工盘点、售后返库
- 秒杀库存分桶
- Redis 热点库存主扣减
- 对外公网 HTTP 直出库存写接口

## 4. 服务边界与协作

### 4.1 服务边界

- `inventory-service`
  - 负责库存真相源
  - 负责库存可售判断
  - 负责库存预占、释放、确认扣减
- `product-service`
  - 负责商品主数据
  - 负责商品是否在线、SKU 是否 active
- `cart-service`
  - 负责购物车项持久化
  - 不直接判断库存
- `gateway`
  - 负责对外 HTTP
  - 负责编排商品校验和库存校验
- `order-service`
  - 当前还未正式接入
  - 后续会成为库存预占、释放、确认扣减的主要调用方

### 4.2 协作原则

- 商品域和库存域通过 `sku_id` 关联
- `product-service` 负责“能不能卖”
- `inventory-service` 负责“还有没有库存可卖”
- `gateway` 或后续 `order-service` 负责把多服务能力编排成完整业务链路

## 5. 已实现业务链路

### 5.1 购物车加购前库存校验

1. 客户端调用 `POST /api/v1/cart/items`
2. `gateway` 调用 `product-service.GetProductDetail`
3. 校验商品在线、目标 SKU 可售
4. `gateway` 调用 `inventory-service.CheckSaleableStock`
5. `inventory-service` 判断 `available_stock >= quantity`
6. 校验通过后，`gateway` 调用 `cart-service.AddCartItem`

### 5.2 商品创建后的库存初始化

1. 管理端创建商品，请求中的 SKU 可带 `initial_stock`
2. `gateway` 发起 `DTM workflow`
3. `gateway` 调用 `product-service.CreateProductSaga`
4. 商品和 SKU 创建成功后，`gateway` 调用 `inventory-service.InitSkuStocksSaga`
5. `inventory-service` 为每个新 `sku_id` 创建库存记录
6. 如果目标状态是 `online`，`gateway` 最后调用商品上架
7. 任一步失败时，由 `DTM` 驱动前序成功分支补偿

### 5.3 商品更新新增 SKU 后的库存初始化

1. 管理端更新商品并提交新增 SKU
2. `gateway` 调用 `product-service.UpdateProduct`
3. `gateway` 识别本次新增的 SKU
4. `gateway` 调用 `inventory-service.InitSkuStocks`
5. `inventory-service` 为新增 SKU 写入库存记录

### 5.4 库存预占、释放、确认扣减

1. 上游业务方按 `biz_type + biz_id` 调用库存服务
2. `inventory-service` 先查询 `inventory_reservations`
3. 对同一 `biz_type + biz_id + sku_id` 做幂等和状态冲突判定
4. 在本地事务中同时更新 `inventory_stocks`
5. 同时写入或更新 `inventory_reservations`

三类动作语义：

- 预占成功：`reserved_stock += quantity`，`available_stock -= quantity`
- 释放成功：`reserved_stock -= quantity`，`available_stock += quantity`
- 确认扣减成功：`total_stock -= quantity`，`reserved_stock -= quantity`

## 6. 业务规则

- 一个 `sku_id` 对应一条库存记录
- `total_stock`、`reserved_stock`、`available_stock` 均不能小于 `0`
- `reserved_stock` 不能大于 `total_stock`
- `available_stock` 不能大于 `total_stock`
- `saleable_stock` 当前等于 `available_stock`
- `CheckSaleableStock` 只判断是否足够，不修改库存
- `InitSkuStocks` 只用于首次初始化库存，不用于重复覆盖
- `AdjustStock` 当前按“设置总库存”语义执行，不做增量加减
- 库存记录不存在时，当前按不可售处理
- 库存服务不负责判断商品是否在线，也不判断 SKU 是否 active
- 同一 `biz_type + biz_id + sku_id` 只能有一条预占记录
- 预占记录当前支持 `reserved / released / confirmed` 三种状态
- `ReserveSkuStocks` 只做锁库存，不减少 `total_stock`
- `ReleaseReservedSkuStocks` 只释放之前已锁住的库存
- `ConfirmDeductReservedSkuStocks` 把已预占库存转成真实扣减
- 未预占直接释放时会写入 `released` 标记，防止后续同一业务单再悬挂预占

## 7. 数据模型

### 7.1 表结构

数据库：`meshcart_inventory`

主表：`inventory_stocks`

- `id`
  - 库存记录主键 ID
- `sku_id`
  - SKU ID
- `total_stock`
  - 当前账面总库存
- `reserved_stock`
  - 已经被业务单锁住、但还未真正卖出的库存
- `available_stock`
  - 当前仍可继续售卖的库存
- `status`
  - 库存状态，`0` 表示冻结，`1` 表示可用
- `version`
  - 乐观锁版本号预留字段
- `created_at`
  - 创建时间
- `updated_at`
  - 更新时间

预占记录表：`inventory_reservations`

- `id`
  - 预占记录主键 ID
- `biz_type`
  - 业务类型，例如 `order`
- `biz_id`
  - 业务单号，例如订单 ID
- `sku_id`
  - 本条记录关联的 SKU ID
- `quantity`
  - 本次预占对应数量
- `status`
  - 当前记录状态，支持 `reserved / released / confirmed`
- `payload_snapshot`
  - 请求快照，用于幂等排障和问题定位
- `created_at`
  - 创建时间
- `updated_at`
  - 更新时间

### 7.2 索引与约束

`inventory_stocks`

- 主键：`id`
- 唯一索引：`uk_sku_id (sku_id)`
- 普通索引：`idx_updated_at (updated_at)`

`inventory_reservations`

- 主键：`id`
- 唯一索引：`uk_inventory_reservation_biz_sku (biz_type, biz_id, sku_id)`
- 普通索引：`idx_inventory_reservation_sku_id (sku_id)`

### 7.3 设计理由

- 先以 SKU 维度库存服务交易主链路
- 使用 `sku_id` 唯一约束保证单 SKU 单库存记录
- `available_stock` 直接落库，简化读取和校验逻辑
- `reserved_stock` 和 `version` 支撑预占和并发更新
- `inventory_reservations` 把预占幂等、防悬挂和状态流转收口在库存域内部

### 7.4 字段关系说明

当前库存表遵循以下关系：

- `available_stock = total_stock - reserved_stock`

三类写操作对应的字段变化如下：

- 预占
  - `reserved_stock += quantity`
  - `available_stock -= quantity`
  - `total_stock` 不变
- 释放
  - `reserved_stock -= quantity`
  - `available_stock += quantity`
  - `total_stock` 不变
- 确认扣减
  - `total_stock -= quantity`
  - `reserved_stock -= quantity`
  - `available_stock` 不变

## 8. 已实现接口

IDL 定义见 [idl/inventory.thrift](/Users/ruitong/GolandProjects/MeshCart/idl/inventory.thrift)。

### 8.1 RPC 接口总览

- `GetSkuStock`
- `BatchGetSkuStock`
- `CheckSaleableStock`
- `InitSkuStocks`
- `InitSkuStocksSaga`
- `CompensateInitSkuStocksSaga`
- `FreezeSkuStocks`
- `AdjustStock`
- `ReserveSkuStocks`
- `ReleaseReservedSkuStocks`
- `ConfirmDeductReservedSkuStocks`

### 8.2 主要接口语义

#### `GetSkuStock`

- 作用：查询单个 SKU 当前库存快照
- 请求字段：
  - `sku_id`
- 返回字段：
  - `stock`
  - `base.code`
  - `base.message`

#### `BatchGetSkuStock`

- 作用：批量查询多个 SKU 当前库存快照
- 请求字段：
  - `sku_ids`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`

#### `CheckSaleableStock`

- 作用：判断指定 SKU 对指定数量是否可售
- 请求字段：
  - `sku_id`
  - `quantity`
- 返回字段：
  - `saleable`
  - `available_stock`
  - `base.code`
  - `base.message`

#### `InitSkuStocks`

- 作用：普通库存初始化
- 请求字段：
  - `stocks[].sku_id`
  - `stocks[].total_stock`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`

#### `InitSkuStocksSaga`

- 作用：商品创建分布式事务中的库存初始化
- 请求字段：
  - `global_tx_id`
  - `branch_id`
  - `stocks[].sku_id`
  - `stocks[].total_stock`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`

#### `CompensateInitSkuStocksSaga`

- 作用：回滚本次事务初始化的库存记录
- 请求字段：
  - `global_tx_id`
  - `branch_id`
  - `sku_ids`
- 返回字段：
  - `base.code`
  - `base.message`

#### `FreezeSkuStocks`

- 作用：批量冻结已失效 SKU 对应库存记录
- 请求字段：
  - `sku_ids`
  - `operator_id`
  - `reason`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`

#### `AdjustStock`

- 作用：后台按“设置总库存”语义调整库存
- 请求字段：
  - `sku_id`
  - `total_stock`
  - `reason`
- 返回字段：
  - `stock`
  - `base.code`
  - `base.message`

#### `ReserveSkuStocks`

- 作用：按业务单号预占库存
- 请求字段：
  - `biz_type`
  - `biz_id`
  - `items[].sku_id`
  - `items[].quantity`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`
- 语义：
  - 成功后：`reserved_stock += quantity`，`available_stock -= quantity`
  - `total_stock` 不变
  - 同一业务单重复请求按幂等成功处理

#### `ReleaseReservedSkuStocks`

- 作用：释放已预占库存
- 请求字段：
  - `biz_type`
  - `biz_id`
  - `items[].sku_id`
  - `items[].quantity`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`
- 语义：
  - 成功后：`reserved_stock -= quantity`，`available_stock += quantity`
  - 已释放重复调用按成功处理
  - 未预占直接释放时写 `released` 标记

#### `ConfirmDeductReservedSkuStocks`

- 作用：把已预占库存转成真实扣减
- 请求字段：
  - `biz_type`
  - `biz_id`
  - `items[].sku_id`
  - `items[].quantity`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`
- 语义：
  - 成功后：`total_stock -= quantity`，`reserved_stock -= quantity`
  - `available_stock` 不变
  - 没有预占记录时返回业务错误

### 8.3 已实现 HTTP 接口

当前对外开放的 HTTP 接口仍只有后台库存查询与调整能力，由 `gateway` 暴露：

- `GET /api/v1/admin/inventory/skus/:sku_id`
- `POST /api/v1/admin/inventory/skus/batch_get`
- `PUT /api/v1/admin/inventory/skus/:sku_id/stock`

库存预占、释放、确认扣减目前只提供 RPC，不提供对外 HTTP 接口。

#### `GET /api/v1/admin/inventory/skus/:sku_id`

- 作用：查询单个 SKU 库存
- 权限：`admin` / `superadmin`
- 路径参数：
  - `sku_id`
- 请求体：无
- 成功返回字段：
  - `code`
  - `message`
  - `data.sku_id`
  - `data.total_stock`
  - `data.reserved_stock`
  - `data.available_stock`
  - `data.saleable_stock`
  - `data.status`
- 说明：
  - `admin` 只能查询自己创建商品对应的库存
  - `superadmin` 可查询任意库存

#### `POST /api/v1/admin/inventory/skus/batch_get`

- 作用：批量查询多个 SKU 库存
- 权限：`admin` / `superadmin`
- 请求体字段：
  - `sku_ids`
- 成功返回字段：
  - `code`
  - `message`
  - `data.items[].sku_id`
  - `data.items[].total_stock`
  - `data.items[].reserved_stock`
  - `data.items[].available_stock`
  - `data.items[].saleable_stock`
  - `data.items[].status`
- 说明：
  - 当前会按请求中的 `sku_ids` 批量查询库存快照
  - `admin` 仍受商品归属约束

请求体示例：

```json
{
  "sku_ids": [3001, 3002]
}
```

#### `PUT /api/v1/admin/inventory/skus/:sku_id/stock`

- 作用：后台按“设置总库存”语义调整库存
- 权限：`admin` / `superadmin`
- 路径参数：
  - `sku_id`
- 请求体字段：
  - `total_stock`
  - `reason`
- 成功返回字段：
  - `code`
  - `message`
  - `data.sku_id`
  - `data.total_stock`
  - `data.reserved_stock`
  - `data.available_stock`
  - `data.saleable_stock`
  - `data.status`
- 说明：
  - 当前是“设置总库存”而不是“增量加减库存”
  - 如果目标总库存小于当前 `reserved_stock`，会返回业务错误
  - `admin` 只能调整自己创建商品对应的库存

请求体示例：

```json
{
  "total_stock": 80,
  "reason": "manual correction"
}
```

## 9. 授权与安全

- 库存服务不直接暴露公网接口
- 对外授权统一收口在 `gateway`
- `inventory-service` 不直接执行 Casbin 判定
- 当前后台库存查询与库存调整能力由 `gateway` 对管理员开放
- `admin` 不能修改其他管理员创建商品对应的库存
- `gateway` 会先通过商品服务查询 `sku_id` 对应商品归属，再决定是否放行库存读写

## 10. 错误码

- `2050001`：库存记录不存在
- `2050002`：库存不足
- `2050003`：库存记录已存在
- `2050004`：库存数量不合法
- `2050005`：库存已冻结
- `2050006`：库存预占状态冲突
- `2050007`：库存预占记录不存在

## 11. 治理与运行基线

### 11.1 目录结构

```text
services/inventory-service/
├── cmd/inventory/main.go
├── config/
│   ├── config.go
│   └── inventory-service.local.yaml
├── rpc/
│   ├── main.go
│   ├── bootstrap/
│   └── handler/
├── biz/
│   ├── errno/
│   ├── repository/
│   └── service/
├── dal/
│   ├── db/
│   └── model/
├── migrations/
└── script/
```

### 11.2 已接入治理能力

- `/healthz`
- `/readyz`
- `/metrics`
- preflight
- graceful shutdown
- Consul 注册与 `direct/consul` 双态发现
- MySQL query timeout
- 结构化日志
- OTel tracing
- RPC metrics

### 11.3 关键配置

- `INVENTORY_RPC_SERVICE`
- `INVENTORY_SERVICE_ADDR`
- `INVENTORY_SERVICE_REGISTRY`
- `INVENTORY_METRICS_ADDR`
- `INVENTORY_SERVICE_CONFIG`
- `INVENTORY_SERVICE_PREFLIGHT_TIMEOUT_MS`
- `INVENTORY_SERVICE_DRAIN_TIMEOUT_MS`
- `INVENTORY_SERVICE_SHUTDOWN_TIMEOUT_MS`

## 12. 已实现清单

- `idl/inventory.thrift` 定义与生成代码
- `inventory-service` 启动入口、bootstrap、配置、migration
- 库存表 `inventory_stocks`
- 预占记录表 `inventory_reservations`
- 库存查询、可售校验、初始化、冻结、调整能力
- 库存预占、释放、确认扣减能力
- 商品创建链路的库存初始化 Saga
- `gateway` 侧 inventory RPC client
- `gateway` 购物车加购前库存校验接入
- `gateway` 购物车更新数量前库存校验接入
- `gateway` 后台库存 HTTP 查询与调整入口

## 13. 测试情况

当前已补测试：

- repository 单测
- service 单测
- inventory gateway logic 单测
- product create logic 单测
- gateway cart add logic 单测
- gateway cart update logic 单测

已通过的针对性测试命令：

```bash
go test ./services/inventory-service/... ./gateway/internal/logic/inventory ./gateway/internal/logic/product ./gateway/internal/logic/cart ./gateway/rpc/inventory
```

## 14. 后续演进

### 14.1 仍未完成事项

- 真实环境 migration 联调验证
- `gateway + product-service + inventory-service + cart-service` 验收链路
- `order-service` 正式接入库存主链路
- 更细粒度库存流水与审计
- 多仓与热点库存治理

### 14.2 订单服务接入建议

推荐链路：

1. `order-service` 创建订单草稿
2. 调用 `inventory-service.ReserveSkuStocks`
3. 支付成功后调用 `ConfirmDeductReservedSkuStocks`
4. 支付失败、取消订单或超时关闭时调用 `ReleaseReservedSkuStocks`

### 14.3 高并发演进设计

当前实现默认以 MySQL 为库存真相源，适合普通商城主链路和后台管理场景。

当后续出现热点 SKU、秒杀、大促或多实例网关下的高并发抢购时，建议按以下顺序演进：

1. 继续保留 `inventory-service` 和 MySQL 作为最终账本
2. 在入口前增加 Redis 热点库存令牌或可售额度缓存
3. 请求先在 Redis 做原子扣减，快速拦截明显超卖请求
4. 成功后投递 MQ，异步驱动库存服务执行正式预占
5. 消费端仍调用库存域正式能力，而不是绕过 `inventory-service` 直接写库
6. Redis、MQ 只承担削峰和前置拦截，不替代库存服务的业务真相源

职责拆分建议：

- Redis
  - 热点库存缓存
  - 原子预减
  - 请求削峰入口
- MQ
  - 平滑瞬时洪峰
  - 异步串行化热点业务
- MySQL / `inventory-service`
  - 最终库存账本
  - 幂等与状态机
  - 释放与确认扣减

约束原则：

- 不能让 Redis 成为唯一库存事实来源
- 不能只做 Redis 扣减而没有 MySQL 账本落地
- 不能跳过 `inventory_reservations` 直接做异步总库存修改
- 高并发方案必须继续保证预占、释放、确认扣减的状态语义不变
