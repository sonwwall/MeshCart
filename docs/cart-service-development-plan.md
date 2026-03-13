# Cart Service 开发计划

## 1. 目的

本文档用于规划 `cart-service` 的最小可落地范围、实施顺序和配套治理要求。

本文档以 [service-development-spec.md](./service-development-spec.md) 为直接约束，不单独发明新的服务结构、治理方式或分层模型。

## 2. 规划依据

本计划主要遵循以下现有规范：

- [service-development-spec.md](./service-development-spec.md)
  - 服务目录结构
  - 分层职责
  - 配置规范
  - 服务发现规范
  - 可观测性规范
  - 微服务治理规范
  - 测试规范
- [microservice-governance.md](./microservice-governance.md)
  - 当前治理基线
  - 当前不优先做的治理项
- [evolution-plan.md](./evolution-plan.md)
  - 当前整体演进顺序

## 3. 目标定位

`cart-service` 作为下一条优先业务线，目标不是一次性做完整交易链路，而是作为“首个完整复用当前治理模板的新服务”落地。

本阶段希望同时验证两件事：

1. `cart-service` 业务本身能形成最小可用能力
2. 现有服务开发规范和治理模板可以被新服务低成本复用

## 4. 服务边界

`cart-service` 当前阶段负责：

- 用户购物车查询
- 加购
- 修改购物车商品数量
- 删除购物车商品
- 清空购物车
- 面向结算前提供购物车条目读取能力

`cart-service` 当前阶段不负责：

- 下单
- 库存预占与扣减
- 价格锁定
- 优惠券、促销、满减
- 购物车商品失效自动修复
- 跨端购物车合并
- 游客购物车

## 5. 协作关系

当前建议协作关系如下：

- `gateway`
  - 对外暴露购物车 HTTP 接口
  - 负责登录态、统一响应、错误映射、对外授权编排
- `cart-service`
  - 提供购物车读写 RPC
  - 持久化购物车主数据
- `product-service`
  - 提供商品基础只读信息
  - 用于加购前最小校验和购物车展示补充字段

当前阶段不强依赖：

- `inventory-service`
- `order-service`
- `payment-service`

原因：

- 当前优先验证“用户 + 商品 + 购物车”三段链路
- 避免一开始就把库存与订单事务复杂度引入

## 6. MVP 范围

建议按以下最小范围落地：

### 6.1 HTTP 接口

建议先提供：

- `GET /api/v1/cart`
  - 查询当前用户购物车
- `POST /api/v1/cart/items`
  - 新增加购项
- `PUT /api/v1/cart/items/:item_id`
  - 修改数量
- `DELETE /api/v1/cart/items/:item_id`
  - 删除购物车项
- `DELETE /api/v1/cart`
  - 清空购物车

统一约束：

- 全部需要登录
- 当前只支持登录用户购物车
- 不支持匿名购物车
- 购物车资源默认只允许用户访问自己的数据

### 6.2 RPC 接口

建议先定义以下 RPC：

- `GetCart`
- `AddCartItem`
- `UpdateCartItem`
- `RemoveCartItem`
- `ClearCart`

当前 `idl/cart.thrift` 还是空壳，建议先补齐这五个接口，再生成代码。

### 6.3 最小业务规则

当前阶段建议采用以下简单规则：

- 一个用户对同一 `sku_id` 只保留一条购物车项
- 重复加购时累加数量，而不是插入新行
- 数量必须大于 `0`
- 购物车展示优先读取落库快照字段
- 商品是否允许加购，先只校验：
  - 商品存在
  - 商品状态允许售卖
  - SKU 状态允许售卖

当前阶段不在购物车层处理：

- 实时库存校验
- 价格变动重算
- 商品复杂上下架联动

### 6.4 授权规则建议

按当前 [service-development-spec.md](./service-development-spec.md) 的授权规范，`cart-service` 不应在下游 RPC 服务里自行扩散权限判断，对外授权仍由 `gateway` 统一收口。

当前阶段建议采用最小授权模型：

- `guest`
  - 不允许访问购物车接口
- `user`
  - 可以访问自己的购物车
- `admin`
  - 当前阶段不额外赋予特殊购物车权限
- `superadmin`
  - 当前阶段不单独开放“查看他人购物车”能力，除非后续明确有平台治理需求

当前阶段建议抽象的资源与动作：

- `obj`
  - `cart`
- `act`
  - `read_own`
  - `write_own`

说明：

- 当前购物车不需要一开始就引入复杂 Casbin matcher
- 但应保持与现有授权模型一致的抽象方式，避免后续再重构
- 购物车接口的核心权限语义本质上是“只可访问自己的购物车”

## 7. 数据模型建议

### 7.1 当前建议表

建议先落一张主表：

- `cart_items`

建议字段：

- `id`
- `user_id`
- `product_id`
- `sku_id`
- `quantity`
- `checked`
- `title_snapshot`
- `sku_title_snapshot`
- `sale_price_snapshot`
- `cover_url_snapshot`
- `created_at`
- `updated_at`

### 7.2 当前设计说明

当前建议采用“用户购物车项表”而不是更复杂的主从模型。

原因：

- 当前阶段只支持登录用户购物车
- 一个用户一组购物车项就足够表达当前需求
- 后续如果需要购物车头信息，再扩充 `carts` 主表也来得及

### 7.3 约束建议

- 唯一索引：`(user_id, sku_id)`
- 普通索引：`user_id`
- 普通索引：`updated_at`

说明：

- 唯一索引用于保证“同一用户同一 SKU 只有一条购物车项”
- 更新数量时直接基于该唯一键收口

## 8. 分层落地建议

按 [service-development-spec.md](./service-development-spec.md) 现有规范，建议结构保持为：

```text
services/cart-service/
├── cmd/cart/main.go
├── config/
├── rpc/
│   ├── main.go
│   ├── bootstrap/
│   ├── handler/
│   └── script/
├── biz/
│   ├── dto/
│   ├── errno/
│   ├── model/
│   ├── repository/
│   └── service/
├── dal/
│   ├── db/
│   ├── model/
│   └── redis/
├── migrations/
└── script/
```

当前分层要求：

- `rpc/bootstrap`
  - 复用现有服务生命周期治理模板
- `rpc/handler`
  - 只做 RPC 请求转换与响应组装
- `biz/service`
  - 收口加购、改数量、删项、清空等核心规则
- `biz/repository`
  - 收口按用户查询、按唯一键 upsert、删除等数据访问
- `dal/model`
  - 只保留持久化模型

授权落点要求：

- `gateway/internal/handler/cart`
  - 不直接手写分散角色判断
- `gateway/internal/logic/cart`
  - 结合当前登录用户完成“只访问自己购物车”的约束
- `gateway/internal/authz`
  - 如果后续把购物车纳入 Casbin，应优先在这里扩展资源与动作定义
- `cart-service`
  - 默认不直接承载 Casbin 策略执行

## 9. 配置与治理接入要求

`cart-service` 作为新服务，默认必须复用当前治理模板。

至少要接入：

- `/healthz`
- `/readyz`
- `/metrics`
- preflight
- draining + graceful shutdown
- Consul 注册与 `direct/consul` 双态发现约定
- MySQL query timeout
- 结构化日志
- OTel tracing
- RPC metrics
- 网关侧统一授权约定

建议配置分组：

- `MySQL`
- `Migration`
- `Snowflake`
- `Timeout`
- `Metrics`
- `Telemetry`

建议环境变量命名：

- `CART_RPC_SERVICE`
- `CART_SERVICE_ADDR`
- `CART_SERVICE_REGISTRY`
- `CART_METRICS_ADDR`
- `CART_SERVICE_PREFLIGHT_TIMEOUT_MS`
- `CART_SERVICE_DRAIN_TIMEOUT_MS`
- `CART_SERVICE_SHUTDOWN_TIMEOUT_MS`

## 10. Gateway 接入计划

当前 `cart-service` 落地不应只停在 RPC 服务目录内，还应同步规划 `gateway` 接入。

建议新增：

- `gateway/rpc/cart`
- `gateway/internal/types/cart.go`
- `gateway/internal/handler/cart/`
- `gateway/internal/logic/cart/`

接入要求：

- 所有购物车接口必须挂 JWT
- handler 不直接访问 RPC client
- logic 统一处理技术错误映射
- 购物车接口默认纳入现有网关限流体系
- 授权默认按“当前用户只能访问自己的购物车”收口
- 如果后续引入 Casbin 规则，应在 `gateway/internal/authz` 增加 `cart` 资源和对应动作

## 11. 开发顺序建议

建议按以下顺序推进：

1. 补 `idl/cart.thrift`
2. 生成 `kitex_gen`
3. 设计并创建 `cart_items` migration
4. 实现 `dal/model` 与 `biz/repository`
5. 实现 `biz/service`
6. 实现 `rpc/handler` 与 `rpc/bootstrap`
7. 接入 `gateway/rpc/cart`
8. 接入 `gateway` 的 handler / logic / types
9. 补文档和测试

## 12. 测试计划

按当前规范，至少应补：

### 12.1 服务内测试

- `biz/service` 单测
  - 加购合并
  - 数量更新边界
  - 删除与清空
- `biz/repository` 单测
  - 唯一键 upsert
  - 按用户查询
  - timeout 行为

### 12.2 Gateway 侧测试

- `gateway/internal/logic/cart` 单测
- `gateway` 验收测试
  - 登录后查询购物车
  - 加购
  - 改数量
  - 删除
  - 非本人购物车访问拦截

### 12.3 治理相关测试

- preflight 行为接线
- `/healthz`、`/readyz`、`/metrics` 接线
- draining / shutdown 行为接线

## 13. 文档同步计划

完成 `cart-service` MVP 后，至少同步以下文档：

- [architecture.md](./architecture.md)
- [api.md](./api.md)
- [casbin-design.md](./casbin-design.md)
- [runbook.md](./runbook.md)
- [service-development-spec.md](./service-development-spec.md)
- 新增 `cart-service` 设计文档

建议在本计划基础上，后续再拆出正式的 `cart-service` 设计文档，用于记录：

- 最终表结构
- 最终 RPC/HTTP 接口
- 最终服务边界
- 后续演进方向

## 14. 里程碑建议

### M1：服务骨架可运行

- `idl` 补齐
- `rpc/bootstrap` 跑通
- MySQL migration 跑通
- probe / preflight / draining 接入完成

### M2：MVP 业务闭环

- 加购
- 查购物车
- 改数量
- 删除 / 清空
- `gateway -> cart-service` 联调跑通

### M3：测试与文档收口

- 关键单测与验收测试补齐
- 文档同步完成
- 可作为后续 `inventory-service` 或 `order-service` 的上游输入

## 15. 当前结论

`cart-service` 适合作为下一条优先业务线，因为它：

- 能直接复用当前服务开发规范和治理模板
- 依赖关系清晰，复杂度低于 `order-service`
- 能较快验证“新增服务是否可以低成本按统一标准落地”

因此，当前建议：

1. 先按本文档完成 `cart-service` MVP
2. 再基于真实问题决定是否推进更复杂的库存与订单链路
