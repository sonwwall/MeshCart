# Cart Service 设计文档

## 1. 目的

本文档用于说明 `cart-service` 的当前设计方案，包括服务边界、协作关系、数据模型、接口定义、治理接入方式和后续演进方向。

本文档直接遵循 [service-development-spec.md](./service-development-spec.md) 中的通用服务规范，不再使用“开发计划”口径描述未定事项。

## 2. 设计依据

- [service-development-spec.md](./service-development-spec.md)
- [microservice-governance.md](./microservice-governance.md)
- [evolution-plan.md](./evolution-plan.md)
- [casbin-design.md](./casbin-design.md)

## 3. 目标与边界

### 3.1 当前目标

`cart-service` 的目标是作为当前阶段第一条完整复用现有治理模板的新业务服务，形成“用户 -> 商品 -> 购物车”的最小闭环。

### 3.2 当前负责

- 查询当前用户购物车
- 加购
- 修改购物车商品数量
- 删除购物车项
- 清空购物车
- 保存购物车展示所需的商品快照字段

### 3.3 当前不负责

- 游客购物车
- 购物车跨端合并
- 下单
- 库存预占与扣减
- 价格锁定
- 优惠券、促销、满减
- 购物车失效自动修复

## 4. 总体方案

### 4.1 协作关系

- `gateway`
  - 暴露购物车 HTTP 接口
  - 负责 JWT 登录态识别
  - 负责“只能访问自己的购物车”这一层授权收口
  - 负责加购前调用 `product-service` 做商品和 SKU 状态校验
  - 负责加购前调用 `inventory-service` 做库存可售校验
  - 负责把商品快照注入到 cart RPC 请求中
- `cart-service`
  - 提供购物车读写 RPC
  - 持久化购物车主数据
  - 不直接承载 Casbin 判定
- `product-service`
  - 提供商品详情只读能力
  - 当前主要给加购链路提供商品在线、SKU 可售和快照来源
- `inventory-service`
  - 提供库存查询和库存可售校验能力
  - 当前主要给加购链路提供库存是否足够的判断

### 4.2 核心链路

#### 查询购物车

1. 客户端调用 `GET /api/v1/cart`
2. `gateway` 从 JWT 解析当前用户身份
3. `gateway` 调用 `cart-service.GetCart`
4. `cart-service` 按 `user_id` 查询 `cart_items`
5. 返回购物车项列表

#### 加购

1. 客户端调用 `POST /api/v1/cart/items`
2. `gateway` 从 JWT 解析当前用户身份
3. `gateway` 调用 `product-service.GetProductDetail`
4. 校验商品状态为在线、目标 SKU 状态为可售
5. `gateway` 调用 `inventory-service.CheckSaleableStock`
6. 校验目标 SKU 当前库存充足
7. `gateway` 从商品详情提取标题、SKU 标题、价格、封面快照
8. `gateway` 调用 `cart-service.AddCartItem`
9. `cart-service` 按 `(user_id, sku_id)` 唯一约束执行“创建或累加”
10. 返回最终购物车项

#### 修改、删除、清空

- 修改：`gateway -> cart-service.UpdateCartItem`
- 删除：`gateway -> cart-service.RemoveCartItem`
- 清空：`gateway -> cart-service.ClearCart`

这些操作都只允许当前登录用户作用于自己的购物车数据。

## 5. 数据模型设计

### 5.1 表结构

当前采用单表模型：

- 表名：`cart_items`

字段：

- `id`
  - 购物车项主键 ID
- `user_id`
  - 用户 ID
- `product_id`
  - 商品 ID
- `sku_id`
  - SKU ID
- `quantity`
  - 当前数量
- `checked`
  - 当前是否勾选
- `title_snapshot`
  - 商品标题快照
- `sku_title_snapshot`
  - SKU 标题快照
- `sale_price_snapshot`
  - 销售价快照
- `cover_url_snapshot`
  - 封面图快照
- `created_at`
- `updated_at`

### 5.2 索引约束

- 唯一索引：`(user_id, sku_id)`
- 普通索引：`user_id`
- 普通索引：`updated_at`

### 5.3 当前设计理由

- 当前只支持登录用户购物车，不需要单独的购物车头表
- 用 `(user_id, sku_id)` 可以直接收口“重复加购累加”
- 用快照字段避免购物车查询时每次回源商品服务

## 6. 业务规则设计

### 6.1 已实现规则

- 同一用户同一 `sku_id` 只保留一条购物车项
- 重复加购相同 `sku_id` 时累加数量
- `quantity` 必须大于 `0`
- `checked` 在加购时默认写入 `true`
- `UpdateCartItem` 当前支持同时更新数量和勾选状态
- `GetCart` 当前按 `updated_at DESC, id DESC` 返回
- 购物车展示优先读取快照字段，不在查询时回源商品服务

### 6.2 当前未处理规则

- 价格变化后的重算
- 商品下架后的自动剔除或标红
- 结算前购物车失效修复
- 购物车已存在项的自动库存重算

## 7. 授权与安全设计

### 7.1 当前授权原则

- 所有购物车 HTTP 接口都必须登录
- 当前购物车资源只允许访问自己的数据
- 对外授权统一收口在 `gateway`
- `cart-service` 不直接执行 Casbin 策略

### 7.2 当前角色语义

- `guest`
  - 不允许访问购物车接口
- `user`
  - 可以访问自己的购物车
- `admin`
  - 当前阶段不额外放大购物车权限
- `superadmin`
  - 当前阶段不开放“查看他人购物车”能力

### 7.3 当前资源抽象

- `obj`
  - `cart`
- `act`
  - `read_own`
  - `write_own`

## 8. 服务结构与治理接入

### 8.1 当前目录结构

```text
services/cart-service/
├── cmd/cart/main.go
├── config/
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

### 8.2 当前治理接入

`cart-service` 已复用当前服务生命周期治理基线：

- `/healthz`
- `/readyz`
- `/metrics`
- preflight
- draining + graceful shutdown
- Consul 注册与 `direct/consul` 双态发现
- MySQL query timeout
- 结构化日志
- OTel tracing
- RPC metrics

### 8.3 关键配置

- `CART_RPC_SERVICE`
- `CART_SERVICE_ADDR`
- `CART_SERVICE_REGISTRY`
- `CART_METRICS_ADDR`
- `CART_SERVICE_PREFLIGHT_TIMEOUT_MS`
- `CART_SERVICE_DRAIN_TIMEOUT_MS`
- `CART_SERVICE_SHUTDOWN_TIMEOUT_MS`

### 8.4 运行与排障

当前购物车链路日志已经按统一规范做过一轮重构：

- `gateway/internal/logic/cart/`
  - `get`
  - `add`
  - `update`
  - `remove`
  - `clear`
- `services/cart-service/biz/service/`
  - `get_cart`
  - `add_cart_item`
  - `update_cart_item`
  - `remove_cart_item`
  - `clear_cart`
- `services/cart-service/biz/repository/repository.go`

当前日志口径：

- Gateway
  - 会区分商品详情失败、库存校验失败、购物车 RPC 失败
  - 下游业务错误会记录 `code/message`
  - nil item / nil result 会打 `Error`
- cart-service service
  - 会记录 `start / reject / completed`
  - 重点字段为 `user_id`、`item_id`、`product_id`、`sku_id`、`quantity`
- repository
  - 会记录加购累加失败、购物车项更新失败、删除/清空失败等原始 DB 错误

当前排障建议：

1. 先看 `gateway/internal/logic/cart/`
   - 判断是商品校验失败、库存校验失败，还是购物车服务本身失败
2. 再看 `cart-service` service / repository
   - 判断是参数拒绝、购物车项不存在，还是底层 DB 更新失败

## 9. 接口设计

### 9.1 HTTP 接口

#### `GET /api/v1/cart`

- 接口名：`GetCart`
- Gateway Logic：`gateway/internal/logic/cart.GetLogic.Get`
- 对应 RPC：`GetCart`
- 作用：查询当前用户购物车

返回示例：

```json
{
  "items": [
    {
      "id": 1,
      "product_id": 2001,
      "sku_id": 3001,
      "quantity": 2,
      "checked": true,
      "title_snapshot": "MeshCart Tee",
      "sku_title_snapshot": "Blue XL",
      "sale_price_snapshot": 1999,
      "cover_url_snapshot": "https://example.test/cover.png"
    }
  ]
}
```

字段说明：

- `items`
  - 当前用户购物车项列表
- `id`
  - 购物车项主键 ID，用于后续更新和删除
- `product_id`
  - 商品 ID
- `sku_id`
  - SKU ID，也是当前购物车唯一收口的关键字段
- `quantity`
  - 当前购物车项最终数量
- `checked`
  - 当前条目是否勾选
- `title_snapshot`
  - 商品标题快照
- `sku_title_snapshot`
  - SKU 标题快照
- `sale_price_snapshot`
  - 加购时记录的销售价快照
- `cover_url_snapshot`
  - 加购时记录的封面图快照

#### `POST /api/v1/cart/items`

- 接口名：`AddCartItem`
- Gateway Logic：`gateway/internal/logic/cart.AddLogic.Add`
- 对应 RPC：`AddCartItem`
- 作用：加购

请求体：

```json
{
  "product_id": 2001,
  "sku_id": 3001,
  "quantity": 2,
  "checked": true
}
```

字段说明：

- `product_id`
  - 商品 ID
  - `gateway` 会用它查询商品详情
- `sku_id`
  - 目标 SKU ID
- `quantity`
  - 本次加购数量，必须大于 `0`
- `checked`
  - 可选字段，未传时默认 `true`

返回：

- 返回单个购物车项，结构与 `GET /api/v1/cart` 中单条 `item` 一致

#### `PUT /api/v1/cart/items/:item_id`

- 接口名：`UpdateCartItem`
- Gateway Logic：`gateway/internal/logic/cart.UpdateLogic.Update`
- 对应 RPC：`UpdateCartItem`
- 作用：修改购物车项数量和可选勾选状态

请求体：

```json
{
  "quantity": 3,
  "checked": true
}
```

字段说明：

- `item_id`
  - 路由参数，对应购物车项主键 ID
- `quantity`
  - 更新后的目标数量，不是增量
- `checked`
  - 可选字段，传了才更新勾选状态

返回：

- 返回单个购物车项，结构与 `GET /api/v1/cart` 中单条 `item` 一致

#### `DELETE /api/v1/cart/items/:item_id`

- 接口名：`RemoveCartItem`
- Gateway Logic：`gateway/internal/logic/cart.RemoveLogic.Remove`
- 对应 RPC：`RemoveCartItem`
- 作用：删除单条购物车项

字段说明：

- `item_id`
  - 路由参数，对应购物车项主键 ID

返回：

- `null`

#### `DELETE /api/v1/cart`

- 接口名：`ClearCart`
- Gateway Logic：`gateway/internal/logic/cart.ClearLogic.Clear`
- 对应 RPC：`ClearCart`
- 作用：清空当前用户购物车

返回：

- `null`

### 9.2 RPC 接口

#### `GetCart`

- RPC 方法名：`CartService.getCart`
- RPC Handler：`services/cart-service/rpc/handler.GetCart`

```thrift
struct GetCartRequest {
    1: i64 user_id
}
```

字段说明：

- `user_id`
  - 当前登录用户 ID
  - 由 `gateway` 从 JWT 注入

#### `AddCartItem`

- RPC 方法名：`CartService.addCartItem`
- RPC Handler：`services/cart-service/rpc/handler.AddCartItem`

```thrift
struct AddCartItemRequest {
    1: i64 user_id
    2: i64 product_id
    3: i64 sku_id
    4: i32 quantity
    5: optional bool checked
    6: string title_snapshot
    7: string sku_title_snapshot
    8: i64 sale_price_snapshot
    9: string cover_url_snapshot
}
```

字段说明：

- `user_id`
  - 当前登录用户 ID
- `product_id`
  - 商品 ID
- `sku_id`
  - SKU ID
- `quantity`
  - 本次新增数量
- `checked`
  - 可选勾选状态
- `title_snapshot`
  - 商品标题快照
- `sku_title_snapshot`
  - SKU 标题快照
- `sale_price_snapshot`
  - SKU 销售价快照
- `cover_url_snapshot`
  - SKU 封面图快照

说明：

- 这些快照字段由 `gateway` 调用 `product-service` 后提取并传给 `cart-service`

#### `UpdateCartItem`

- RPC 方法名：`CartService.updateCartItem`
- RPC Handler：`services/cart-service/rpc/handler.UpdateCartItem`

```thrift
struct UpdateCartItemRequest {
    1: i64 user_id
    2: i64 item_id
    3: i32 quantity
    4: optional bool checked
}
```

字段说明：

- `user_id`
  - 当前登录用户 ID
- `item_id`
  - 购物车项 ID
- `quantity`
  - 修改后的最终数量
- `checked`
  - 可选勾选状态

#### `RemoveCartItem`

- RPC 方法名：`CartService.removeCartItem`
- RPC Handler：`services/cart-service/rpc/handler.RemoveCartItem`

```thrift
struct RemoveCartItemRequest {
    1: i64 user_id
    2: i64 item_id
}
```

字段说明：

- `user_id`
  - 当前登录用户 ID
- `item_id`
  - 要删除的购物车项 ID

#### `ClearCart`

- RPC 方法名：`CartService.clearCart`
- RPC Handler：`services/cart-service/rpc/handler.ClearCart`

```thrift
struct ClearCartRequest {
    1: i64 user_id
}
```

字段说明：

- `user_id`
  - 当前登录用户 ID

## 10. 当前实现状态

当前仓库已经落地以下内容：

- `idl/cart.thrift` 已补齐并完成 `kitex_gen` 生成
- `cart-service` 已实现配置、migration、repository、service、RPC handler、bootstrap、启动脚本
- `gateway` 已实现 `cart` 路由、`cart` logic、`cart` RPC client
- 已实现 JWT 下“只访问自己的购物车”
- 已实现加购前通过 `product-service` 做商品在线、SKU 可售校验
- 已实现加购前通过 `inventory-service` 做库存可售校验
- 已实现加购时快照落库
- 已补 cart service 层与 gateway cart logic 层最小单测

## 11. 当前限制

- 还没有游客购物车
- 还没有购物车合并
- 还没有商品失效自动修复
- 还没有批量勾选 / 取消勾选
- 还没有面向结算的专用购物车读取模型
- 还没有完整的远程环境联调验收闭环

## 12. Cart 后续需要做的

建议按以下顺序继续演进：

1. 完成真实联调
   - 建立 `meshcart_cart` 数据库
   - 跑通 migration
   - 用 `gateway` 实测增删改查链路
2. 补 gateway 验收测试
   - 登录后查购物车
   - 加购
   - 改数量
   - 删除 / 清空
3. 补购物车状态能力
   - 批量勾选 / 取消勾选
   - 失效商品标记
4. 补结算前模型
   - 提供“仅返回可结算条目”的读取能力
   - 为 `order-service` 预留输入
5. 与 `order-service` 联动
   - 支持从购物车生成订单输入
   - 支持下单后清理已结算购物车项

当前结论：

- `cart-service` 已经达到 MVP 设计和实现闭环
- 后续不建议继续在购物车上无限细化基础设施
- 更合理的下一条业务线是 `inventory-service`
