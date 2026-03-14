# Inventory Service 设计文档

## 1. 目的

本文档用于说明 `inventory-service` 当前已经完成的设计与实现基线，并整理库存域后续尚未完成的开发计划。

本文档遵循 [service-development-spec.md](./service-development-spec.md) 中的通用服务规范，作为库存服务继续演进时的设计依据和实现参照。

## 2. 设计依据

- [service-development-spec.md](./service-development-spec.md)
- [microservice-governance.md](./microservice-governance.md)
- [evolution-plan.md](./evolution-plan.md)
- [product-service-design.md](./product-service-design.md)
- [cart-service-design.md](./cart-service-design.md)
- [error-code.md](./error-code.md)

---

## 第一部分：当前已完成的设计与实现

## 3. 当前目标与边界

### 3.1 当前目标

`inventory-service` 当前阶段的目标是补齐“商品可售校验 -> 库存可售校验 -> 购物车加购”的最小库存闭环，让购物车加购在商品状态正确之外，还能完成库存是否足够的判断。

### 3.2 当前负责

- 维护 SKU 维度库存主数据
- 提供单个 SKU 库存查询能力
- 提供批量 SKU 库存查询能力
- 提供库存是否足够的只读校验能力
- 为后续订单预占、扣减保留统一库存服务边界

### 3.3 当前不负责

- 多仓、多库位、仓间调拨
- 库存流水明细
- 正式库存预占与释放
- 支付成功后的最终扣减
- 采购入库、人工盘点、售后返库
- Redis 热点库存扣减
- 秒杀库存分桶

## 4. 当前服务边界与协作关系

### 4.1 服务边界

- `inventory-service`
  - 负责库存主数据读能力
  - 负责库存是否足够的判断
  - 当前不做商品状态判断
  - 当前不做订单状态驱动的库存变更
- `product-service`
  - 负责商品是否在线
  - 负责 SKU 是否 active
  - 不负责库存数量
- `cart-service`
  - 负责购物车项持久化
  - 不直接判断库存
- `gateway`
  - 负责在加购链路中编排商品校验与库存校验
  - 负责统一鉴权、参数校验和错误映射

### 4.2 多服务协作关系

当前库存域和其他服务的协作关系如下：

- `gateway`
  - 对外承接 HTTP 请求
  - 不保存商品主数据和库存主数据
  - 负责把多个下游服务编排成一条完整业务链路
- `product-service`
  - 保存商品和 SKU 主数据
  - 负责商品是否在线、SKU 是否 active
  - 不负责库存数量
- `inventory-service`
  - 保存 SKU 对应库存数据
  - 负责库存是否足够
  - 不负责商品标题、价格、图片、上下架状态
- `cart-service`
  - 保存购物车项和商品快照
  - 不负责判断商品是否可售，也不负责判断库存是否足够
- `order-service`
  - 当前阶段还未正式接入库存主链路
  - 后续会成为库存预占、释放、扣减的核心调用方

当前协作原则：

- 商品域和库存域通过 `sku_id` 关联
- 商品服务负责“能不能卖”
- 库存服务负责“还有没有库存可卖”
- 网关或后续订单服务负责把两类判断编排起来

换句话说：

- `product-service` 管商品主数据
- `inventory-service` 管交易库存数据
- 两者不是冲突关系，而是职责拆分关系

### 4.3 当前核心链路

#### 购物车加购前库存校验

1. 客户端调用 `POST /api/v1/cart/items`
2. `gateway` 从 JWT 解析当前用户身份
3. `gateway` 调用 `product-service.GetProductDetail`
4. 校验商品在线、目标 SKU 可售
5. `gateway` 调用 `inventory-service.CheckSaleableStock`
6. `inventory-service` 按 `sku_id` 查询库存记录
7. 判断 `available_stock >= quantity`
8. 校验通过后，`gateway` 调用 `cart-service.AddCartItem`

#### 库存读取链路

1. 内部调用方发起 `GetSkuStock` 或 `BatchGetSkuStock`
2. `inventory-service` 查询 `inventory_stocks`
3. 返回库存快照

### 4.4 当前调用方视角

从“谁会调用库存服务”这个问题看，当前和后续可分为两层：

- 当前已接入调用方
  - `gateway`
    - 在购物车加购链路里调用 `CheckSaleableStock`
- 当前可直接接入但尚未开放 HTTP 的内部调用方
  - 其他服务或后台逻辑
    - 调用 `GetSkuStock`
    - 调用 `BatchGetSkuStock`
- 后续核心调用方
  - `order-service`
    - 预占库存
    - 释放库存
    - 扣减库存

因此，当前库存服务不是给商品服务“替代商品数据”的，而是给 `gateway` 和后续 `order-service` 提供库存侧能力。

## 5. 当前业务规则

- 一个 `sku_id` 对应一条库存记录
- `total_stock`、`reserved_stock`、`available_stock` 均不能小于 `0`
- `reserved_stock` 不能大于 `total_stock`
- `available_stock` 不能大于 `total_stock`
- `saleable_stock` 当前等于 `available_stock`
- `CheckSaleableStock` 只判断是否足够，不修改库存
- 库存记录不存在时，当前按不可售处理
- 库存服务不负责判断商品是否在线，也不判断 SKU 是否 active

## 6. 数据模型设计

### 6.1 库表

- 数据库：`meshcart_inventory`
- 表名：`inventory_stocks`

### 6.2 表结构

字段：

- `id`
  - 库存记录主键 ID
- `sku_id`
  - SKU ID
- `total_stock`
  - 总库存
- `reserved_stock`
  - 已预留给后续预占场景的库存
- `available_stock`
  - 当前可用库存
- `version`
  - 乐观锁版本号预留字段
- `created_at`
- `updated_at`

### 6.3 索引与约束

- 主键：`id`
- 唯一索引：`uk_sku_id (sku_id)`
- 普通索引：`idx_updated_at (updated_at)`

### 6.4 当前设计理由

- 当前只做 SKU 维度库存，先服务交易主链路
- 使用 `sku_id` 唯一约束保证单 SKU 单库存记录
- 预留 `reserved_stock` 和 `version`，避免后续做订单预占时大改表结构
- `available_stock` 直接落库，简化第一阶段读取和校验逻辑

## 7. 服务结构与治理接入

### 7.1 当前目录结构

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

### 7.2 当前治理接入

`inventory-service` 当前已经接入：

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

### 7.3 当前关键配置

- `INVENTORY_RPC_SERVICE`
- `INVENTORY_SERVICE_ADDR`
- `INVENTORY_SERVICE_REGISTRY`
- `INVENTORY_METRICS_ADDR`
- `INVENTORY_SERVICE_CONFIG`
- `INVENTORY_SERVICE_PREFLIGHT_TIMEOUT_MS`
- `INVENTORY_SERVICE_DRAIN_TIMEOUT_MS`
- `INVENTORY_SERVICE_SHUTDOWN_TIMEOUT_MS`

## 8. 授权与安全设计

### 8.1 当前授权原则

- 库存服务当前不直接暴露公网接口
- 对外授权统一收口在 `gateway`
- `inventory-service` 不直接执行 Casbin 判定
- 当前库存写能力未对外开放

### 8.2 当前角色语义

- `guest`
  - 不直接访问库存接口
- `user`
  - 不直接访问库存接口
- `admin`
  - 当前未开放库存管理写接口
- `superadmin`
  - 当前未开放库存管理写接口

## 9. 接口设计

### 9.1 RPC 接口总览

当前第一阶段已实现以下 RPC：

- `GetSkuStock`
- `BatchGetSkuStock`
- `CheckSaleableStock`

IDL 定义见 [idl/inventory.thrift](/Users/ruitong/GolandProjects/MeshCart/idl/inventory.thrift)。

### 9.2 数据结构

#### `SkuStock`

字段：

- `sku_id`
- `total_stock`
- `reserved_stock`
- `available_stock`
- `saleable_stock`

### 9.3 `GetSkuStock`

- 作用：查询单个 SKU 当前库存
- 请求字段：
  - `sku_id`
- 返回字段：
  - `stock`
  - `base.code`
  - `base.message`

成功返回示例：

```json
{
  "stock": {
    "sku_id": 3001,
    "total_stock": 100,
    "reserved_stock": 0,
    "available_stock": 100,
    "saleable_stock": 100
  },
  "base": {
    "code": 0,
    "message": "成功"
  }
}
```

### 9.4 `BatchGetSkuStock`

- 作用：批量查询多个 SKU 当前库存
- 请求字段：
  - `sku_ids`
- 返回字段：
  - `stocks`
  - `base.code`
  - `base.message`

成功返回示例：

```json
{
  "stocks": [
    {
      "sku_id": 3001,
      "total_stock": 100,
      "reserved_stock": 0,
      "available_stock": 100,
      "saleable_stock": 100
    }
  ],
  "base": {
    "code": 0,
    "message": "成功"
  }
}
```

### 9.5 `CheckSaleableStock`

- 作用：判断指定 SKU 对指定数量是否可售
- 请求字段：
  - `sku_id`
  - `quantity`
- 返回字段：
  - `saleable`
  - `available_stock`
  - `base.code`
  - `base.message`

成功返回示例：

```json
{
  "saleable": true,
  "available_stock": 100,
  "base": {
    "code": 0,
    "message": "成功"
  }
}
```

库存不足返回示例：

```json
{
  "saleable": false,
  "available_stock": 1,
  "base": {
    "code": 2050002,
    "message": "库存不足"
  }
}
```

### 9.6 当前 HTTP 接口状态

当前第一阶段未开放独立库存 HTTP 接口。

库存能力目前通过：

- `gateway` 内部编排调用
- 其他服务直接走 RPC

## 10. 错误码

当前第一阶段已实现：

- `2050001`
  - 库存记录不存在
- `2050002`
  - 库存不足

## 11. 当前已完成实现清单

当前已经完成：

- `idl/inventory.thrift` 定义
- `kitex_gen/meshcart/inventory` 生成
- `inventory-service` 启动入口、bootstrap、配置、migration
- 库存表 `inventory_stocks`
- repository 层查询能力
- service 层库存查询与可售校验能力
- RPC handler 层
- gateway 侧 inventory rpc client
- `gateway` 购物车加购前库存校验接入
- 最小单测

## 12. 当前测试情况

当前已补测试：

- repository 单测
- service 单测
- gateway cart add logic 单测

当前已通过的针对性测试命令：

```bash
go test ./services/inventory-service/... ./gateway/internal/logic/cart ./gateway/rpc/inventory
```

---

## 第二部分：未完成事项与后续开发计划

## 13. 当前未完成事项

当前仍未完成：

- 真实环境 migration 联调验证
- `gateway + product-service + inventory-service + cart-service` 验收链路
- 后台库存查询 HTTP 接口
- 库存写能力
- 订单预占、释放、扣减
- 库存流水、审计、幂等
- 多仓与热点库存治理

## 14. 后续设计方案

### 14.1 第二阶段目标

第二阶段建议先补“可管理、可联调、可接订单”的能力，不要直接跳到复杂库存系统。

建议目标：

- 开放后台只读库存查询能力
- 开放库存初始化和后台库存写能力
- 明确订单预占接口设计
- 补库存流水或预占记录模型设计

### 14.2 建议新增接口

优先建议新增：

- `GET /api/v1/admin/inventory/skus/:sku_id`
  - 查询单个 SKU 库存
- `POST /api/v1/admin/inventory/skus/batch_get`
  - 批量查询库存
- `InitSkuStocks`
  - 在商品创建后初始化 SKU 库存
- `AdjustStock`
  - 后台人工调整库存
- `ReserveStock`
  - 供订单服务预占库存
- `ReleaseReservedStock`
  - 供订单取消或超时释放库存
- `DeductStock`
  - 供支付成功后做最终扣减

### 14.3 商品创建时的库存初始化设计

当前建议支持“商品创建时一并提交初始库存”，但不建议让 `product-service` 自己保存库存字段。

推荐设计如下：

1. 管理端创建商品时，请求中的每个 SKU 可带 `initial_stock`
2. `gateway` 先调用 `product-service.CreateProduct`
3. 商品和 SKU 创建成功后，`gateway` 拿到新生成的 `sku_id`
4. `gateway` 再调用 `inventory-service.InitSkuStocks`
5. `inventory-service` 为每个 `sku_id` 创建对应库存记录

这个设计的核心原则是：

- 商品创建请求可以携带初始库存
- 但库存真实落库仍归 `inventory-service`
- `product-service` 不持有库存主数据

不建议的设计：

- 在 `product-service` 表里直接增加库存字段
- 让 `product-service` 直接写库存表
- 把商品创建和库存写入强行耦合成一个服务内职责

原因：

- `product-service` 负责商品主数据
- `inventory-service` 负责库存主数据
- 后续预占、释放、扣减都应围绕库存服务扩展
- 如果商品服务直接承接库存写，后面库存域会再次拆分，成本更高

### 14.4 商品与库存初始化的协作方式

推荐链路：

1. 后台请求到达 `gateway`
2. `gateway` 校验商品参数和 `initial_stock`
3. `gateway` 调用 `product-service.CreateProduct`
4. `product-service` 返回商品 ID 和 SKU ID
5. `gateway` 调用 `inventory-service.InitSkuStocks`
6. `inventory-service` 写入 `inventory_stocks`
7. 两边都成功后，再返回创建成功

### 14.5 初始化库存失败时的处理建议

当前阶段建议按简单补偿方案处理，不必一开始就引入分布式事务：

- 首选方案
  - 商品创建成功但库存初始化失败时，接口整体返回失败
  - 同时把商品保持在 `draft` 或 `offline` 状态，避免进入可售链路
- 次选方案
  - 记录失败日志，由管理员重试初始化库存
- 后续再考虑
  - 异步补偿任务
  - 事务消息
  - 更严格的一致性方案

### 14.6 建议新增模型

后续建议至少补其中一种：

- `inventory_reservations`
  - 记录订单维度预占
- `inventory_flow_logs`
  - 记录库存变更流水

建议字段方向：

- `reservation_id` / `flow_id`
- `sku_id`
- `biz_order_id`
- `quantity`
- `action`
- `status`
- `operator`
- `trace_id`
- `created_at`
- `updated_at`

### 14.7 后台库存写能力建议

在订单链路正式接入前，库存写能力建议先只开放两类：

- 初始化库存
  - 用于商品创建成功后的首次数量写入
- 调整库存
  - 用于后台人工修正库存

当前不建议先开放：

- 任意业务侧直接加减库存
- 多来源并发修改库存且没有流水记录

### 14.8 订单协作设计

后续推荐链路：

1. `order-service` 创建订单草稿
2. `order-service` 调用 `inventory-service.ReserveStock`
3. 支付成功后调用 `DeductStock`
4. 支付失败、取消订单或超时关闭时调用 `ReleaseReservedStock`

### 14.9 当前不建议过早做的事情

当前不建议直接推进：

- Redis 分布式热点扣减
- 多仓库存路由
- 秒杀专用库存模型
- 复杂补偿任务系统

这些能力应等订单链路和基础库存状态机稳定后再进入。

## 15. 后续开发顺序建议

建议按以下顺序继续推进：

1. 跑通真实数据库和 migration
2. 完成库存服务本地启动与联调
3. 补后台只读库存查询接口
4. 设计订单预占模型与接口
5. 补库存流水和幂等设计
6. 最后再评估热点库存和多仓

## 16. 当前结论

- `inventory-service` 第一阶段已经完成最小库存闭环
- 当前最核心的已落地能力是“购物车加购前库存可售校验”
- 下一阶段重点不应是做复杂库存系统，而应先补联调、后台查询和订单预占设计
