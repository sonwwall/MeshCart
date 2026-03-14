# 商品服务设计方案

## 1. 文档目标

本文档描述当前仓库中 `product-service` 的设计方案与实现落点。

本文档覆盖以下内容：

- 服务边界
- 核心数据模型
- 数据库设计
- 对外能力与服务协作
- 权限模型
- 当前项目文件结构
- HTTP / RPC 接口文档
- 后续演进方向

## 2. 设计目标

当前 `product-service` 承担商品中心职责，聚焦以下能力：

- 商品 SPU / SKU 基础信息管理
- 商品上下架状态管理
- 商品列表与详情查询
- 面向交易链路提供商品只读信息

当前阶段明确不承担：

- 库存扣减
- 营销价格、优惠券、活动价
- 搜索引擎检索
- 复杂类目树运营
- 图片媒体管理平台

当前权限由 `gateway` 统一控制：

- 普通用户只能查询 `online` 商品
- 管理员可以创建商品
- 管理员只能查询和修改自己创建的 `draft/offline` 商品
- 管理员也只能修改自己创建的商品
- `superadmin` 可以管理全量商品，并负责用户角色治理

这样设计的原因：

- 商品服务先把商品主数据站稳，比一开始把价格、库存、营销全部揉在一起更可控
- 库存天然适合独立到 `inventory-service`
- 订单创建时对商品服务的核心诉求，本质上是“查商品是否可售、拿到商品快照字段”

## 3. 服务边界

`product-service` 负责：

- 商品基础属性
- SKU 销售属性
- 商品展示状态
- 商品售卖状态
- 商品当前标价
- 后台商品录入与修改
- 前台商品列表、详情读取

`product-service` 不负责：

- 实时库存数量
- 库存预占与扣减
- 支付状态
- 订单状态
- 用户购物车

协作关系如下：

- `gateway`：承接 HTTP，对外暴露商品查询与管理接口
- `product-service`：提供商品读写 RPC
- `inventory-service`：提供库存查询、预占、扣减 RPC
- `order-service`：下单时调用商品服务获取商品快照，再调用库存服务处理库存

补充说明：

- 商品创建请求当前可以携带 SKU 的 `initial_stock`
- 但 `product-service` 仍不直接保存库存字段
- 商品创建成功后，由 `gateway` 调用 `inventory-service.InitSkuStocks` 完成库存初始化
- 商品或 SKU 删除时，当前不建议默认联动物理删除库存记录，而应优先走“下架 / 不可售 + 库存冻结”方案

## 4. 核心模型

当前实现采用 `SPU + SKU` 模型。

### 4.1 SPU

SPU 表示一类商品的抽象定义，例如：

- iPhone 15
- Nike Air Force 1

当前核心字段：

- `id`
- `title`
- `sub_title`
- `category_id`
- `brand`
- `description`
- `status`
- `creator_id`
- `updated_by`
- `created_at`
- `updated_at`

其中 `status` 当前约定：

- `0=draft`
- `1=offline`
- `2=online`

### 4.2 SKU

SKU 表示具体可售卖单元，例如颜色、规格的组合。

当前核心字段：

- `id`
- `spu_id`
- `sku_code`
- `title`
- `sale_price`
- `market_price`
- `status`
- `cover_url`
- `created_at`
- `updated_at`

其中 `status` 当前约定：

- `0=inactive`
- `1=active`

说明：

- `sale_price` 使用最小货币单位存储，即“分”
- 如果后续支持多币种，再单独引入 `currency`
- `sku_id` 才是系统内部真实标识，`sku_code` 当前只保留为可选业务编码
- `sku_code` 不再要求全局唯一；如果管理端不传，系统按空串处理

### 4.3 SKU 销售属性

当前实现单独建表表达 SKU 销售属性，而不是把属性 JSON 塞进主表。

当前字段：

- `id`
- `sku_id`
- `attr_name`
- `attr_value`
- `sort`

例子：

- `颜色=黑色`
- `内存=128G`

## 5. 数据库设计

数据库使用独立库：

- 数据库名：`meshcart_product`
- 目的：与 `meshcart_user` 解耦，便于单服务 migration、权限隔离和后续独立扩容

### 5.1 `products`

存 SPU 主信息，是商品聚合根。

当前表结构：

```sql
CREATE TABLE `products` (
  `id` BIGINT NOT NULL,
  `title` VARCHAR(255) NOT NULL,
  `sub_title` VARCHAR(255) NOT NULL DEFAULT '',
  `category_id` BIGINT NOT NULL DEFAULT 0,
  `brand` VARCHAR(128) NOT NULL DEFAULT '',
  `description` TEXT,
  `status` TINYINT NOT NULL DEFAULT 0,
  `creator_id` BIGINT NOT NULL DEFAULT 0,
  `updated_by` BIGINT NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_category_id` (`category_id`),
  KEY `idx_status` (`status`),
  KEY `idx_creator_id` (`creator_id`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

字段说明：

- `id`：SPU 主键，使用雪花 ID
- `title`：商品主标题，用于列表页、详情页主展示
- `sub_title`：商品副标题，用于补充卖点
- `category_id`：商品所属类目 ID
- `brand`：品牌名称
- `description`：商品详情描述
- `status`：SPU 状态，`0=draft`、`1=offline`、`2=online`
- `creator_id`：商品创建者用户 ID，用于权限控制
- `updated_by`：最后一次修改商品的用户 ID
- `created_at`：创建时间
- `updated_at`：最后更新时间

关键约束：

- 主键：`id`
- 索引：`idx_category_id`
- 索引：`idx_status`
- 索引：`idx_creator_id`
- 索引：`idx_created_at`

说明：

- `products` 是商品聚合根，面向商品管理和商品展示
- 是否允许下单，不能只看 `products.status`，还要结合 SKU 状态一起判断

### 5.2 `product_skus`

存 SKU 主信息。

当前表结构：

```sql
CREATE TABLE `product_skus` (
  `id` BIGINT NOT NULL,
  `spu_id` BIGINT NOT NULL,
  `sku_code` VARCHAR(64) NOT NULL DEFAULT '',
  `title` VARCHAR(255) NOT NULL,
  `sale_price` BIGINT NOT NULL,
  `market_price` BIGINT NOT NULL DEFAULT 0,
  `status` TINYINT NOT NULL DEFAULT 0,
  `cover_url` VARCHAR(512) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_sku_code` (`sku_code`),
  KEY `idx_spu_id` (`spu_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

字段说明：

- `id`：SKU 主键
- `spu_id`：所属 SPU ID
- `sku_code`：可选业务编码，主要给商家侧对账、ERP 或人工识别使用
- `title`：SKU 展示标题
- `sale_price`：销售价，单位分
- `market_price`：划线价或市场价
- `status`：SKU 状态，`0=inactive`、`1=active`
- `cover_url`：SKU 封面图
- `created_at`：创建时间
- `updated_at`：最后更新时间

关键约束：

- 主键：`id`
- 普通索引：`idx_sku_code`
- 索引：`idx_spu_id`
- 索引：`idx_status`

说明：

- 一个 `products` 可以对应多个 `product_skus`
- 下游交易链路通常更关心 SKU，而不是 SPU
- 商品、库存、购物车、订单等内部协作统一使用 `sku_id`
- `sku_code` 当前不是系统内部主键，也不要求商家必须填写
- 当前仅在“同一次创建/更新请求”内限制非空 `sku_code` 不可重复，用于避免单商品内编码混淆
- 商品创建成功后，SKU ID 也会作为库存服务初始化库存记录的关联键
- 订单落库时建议保存 `sku_id`、`sku_code`、`title`、`sale_price` 等快照字段

### 5.3 `product_sku_attrs`

存 SKU 销售属性。

当前表结构：

```sql
CREATE TABLE `product_sku_attrs` (
  `id` BIGINT NOT NULL,
  `sku_id` BIGINT NOT NULL,
  `attr_name` VARCHAR(64) NOT NULL,
  `attr_value` VARCHAR(128) NOT NULL,
  `sort` INT NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_sku_id` (`sku_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

字段说明：

- `id`：销售属性记录主键
- `sku_id`：所属 SKU ID
- `attr_name`：属性名，例如“颜色”“尺码”“内存”
- `attr_value`：属性值，例如“黑色”“XL”“128G”
- `sort`：同一 SKU 下属性展示顺序
- `created_at`：创建时间
- `updated_at`：最后更新时间

关键约束：

- 主键：`id`
- 索引：`idx_sku_id`

说明：

- 当前这张表适合表达简单的规格键值对
- 如果后续要支持规格模板、可选值去重、类目属性继承，再进一步拆表

### 5.4 建表约定

当前统一约定：

- 所有主键使用 `BIGINT`
- 金额统一使用 `BIGINT`，单位用分
- 状态字段统一使用数值枚举
- 时间字段统一保留 `created_at` 和 `updated_at`
- 图片先存 URL，不在当前版本引入媒体资源表

后续如果需要软删除，可以预留：

- `deleted_at`：删除时间，空值表示未删除

### 5.5 Migration 说明

当前 migration 文件包括：

- `000001_create_products`：创建 `products`
- `000002_create_product_skus`：创建 `product_skus`
- `000003_create_product_sku_attrs`：创建 `product_sku_attrs`
- `000004_add_product_owner_fields`：为 `products` 增加 `creator_id`、`updated_by`

## 6. 对外能力

当前已落地 6 个核心能力。

### 6.1 后台写接口

- `CreateProduct`
- `UpdateProduct`
- `ChangeProductStatus`

### 6.2 前台读接口

- `GetProductDetail`
- `ListProducts`
- `BatchGetSku`

说明：

- `BatchGetSku` 主要服务于订单、购物车、结算场景
- 下游不要循环单查 SKU，避免出现 N+1 RPC

## 7. Gateway 接口

### 7.1 面向 C 端

- `GET /api/v1/products`
- `GET /api/v1/products/detail/:product_id`

### 7.2 面向管理端

- `POST /api/v1/admin/products`
- `PUT /api/v1/admin/products/:product_id`
- `POST /api/v1/admin/products/:product_id/status`

当前管理端接口已接入 JWT 和 Casbin 权限控制。

## 8. RPC 接口

`product-service` 当前提供以下 RPC：

- `CreateProduct`
- `UpdateProduct`
- `ChangeProductStatus`
- `GetProductDetail`
- `ListProducts`
- `BatchGetSku`

返回模型分两层：

- `Product`：SPU 视角
- `Sku`：SKU 视角

`GetProductDetail` 直接返回：

- 商品主信息
- 该商品下 SKU 列表
- 每个 SKU 的销售属性

IDL 文件：

- `idl/product.thrift`

默认服务配置：

- 服务名：`meshcart.product`
- 监听地址：`127.0.0.1:8889`
- metrics 地址：`:9093`
- 配置文件：`services/product-service/config/product-service.local.yaml`
- migration 目录：`services/product-service/migrations`

说明：

- `CreateProductRequest` 当前会携带 `creator_id`
- `UpdateProductRequest` 当前会携带 `operator_id`
- `ChangeProductStatusRequest` 当前会携带 `operator_id`
- `ListProductsRequest` 当前支持按 `creator_id` 过滤

## 9. 与库存服务的边界

当前边界规则：

- `product-service` 只判断“商品是否可售”
- `inventory-service` 只判断“库存是否足够”

其中“商品是否可售”至少包含：

- 商品是否存在
- 商品是否 `online`
- SKU 是否 `active`
- 价格是否有效

其中“库存是否足够”只在库存服务里判断：

- 可售库存
- 预占库存
- 扣减库存

不要让 `product-service` 保存库存字段的原因：

- 商品主数据和库存变化频率差异太大
- 库存更新是高并发写，商品信息通常是低频写
- 混在一起后，事务和缓存策略都会变差

## 10. 与订单服务的协作

下单链路按以下方式协作：

1. `gateway` 接收下单请求
2. `gateway` 调 `order-service`
3. `order-service` 调 `product-service.BatchGetSku`
4. `order-service` 校验商品是否可售并生成订单商品快照
5. `order-service` 调 `inventory-service` 预占库存
6. `order-service` 落订单

这里的关键点：

- 订单里必须落商品快照，不能依赖后续再回查商品表
- 商品名称、封面、成交价都应该写入订单明细

否则商品改名、改价后，历史订单会失真。

## 11. 权限模型

商品模块的权限设计不直接写死在 `product-service` 内，而是由 `gateway` 统一控制。

具体设计见：

- `docs/casbin-design.md`

当前商品模块权限规则：

- 未登录用户和普通用户只能查询 `online` 商品
- 管理员可以创建商品
- 管理员查询 `draft/offline` 商品时，只能查询自己创建的商品
- 管理员修改商品或修改商品状态时，也只能操作自己创建的商品
- `superadmin` 可以查询和管理全量商品

当前角色来源：

- 登录后由 `user-service` 返回用户角色
- 角色信息写入 JWT 的 `role` claim

## 12. 当前实现范围

当前版本已经实现：

- 商品建档
- 商品编辑
- 上下架
- 商品列表
- 商品详情
- 批量获取 SKU

当前仍然保持以下简化前提：

- 单商户
- 单币种
- 不做类目树
- 不做品牌中心
- 不做营销价
- 不做搜索
- 不做图片集，只保留一个 `cover_url`

这是当前版本的 MVP 范围。

## 13. 目录结构

当前实现采用和 `user-service` 接近的分层，同时把 `service` 和 `rpc` 都拆成“一接口一文件”，避免单文件持续膨胀。

### 13.1 商品服务目录

```text
services/product-service/
├── biz/
│   ├── dto/
│   │   └── dto.go
│   ├── errno/
│   │   └── errors.go
│   ├── handler/
│   │   └── handler.go
│   ├── model/
│   │   └── model.go
│   ├── repository/
│   │   └── repository.go
│   └── service/
│       ├── service.go
│       ├── mapper.go
│       ├── build_models.go
│       ├── create_product.go
│       ├── update_product.go
│       ├── change_product_status.go
│       ├── get_product_detail.go
│       ├── list_products.go
│       └── batch_get_sku.go
├── config/
│   ├── config.go
│   └── product-service.local.yaml
├── dal/
│   ├── db/
│   │   ├── db.go
│   │   └── migrate.go
│   └── model/
│       └── model.go
├── migrations/
│   ├── 000001_create_products.up.sql
│   └── 000001_create_products.down.sql
├── rpc/
│   ├── main.go
│   ├── bootstrap/
│   │   └── bootstrap.go
│   └── handler/
│       ├── handler.go
│       ├── create_product.go
│       ├── update_product.go
│       ├── change_product_status.go
│       ├── get_product_detail.go
│       ├── list_products.go
│       └── batch_get_sku.go
└── script/
    └── start.sh
```

各文件说明如下：

- `biz/dto/dto.go`：当前仍是预留文件，后续如果商品域出现独立 DTO，可在这里定义。
- `biz/errno/errors.go`：商品域业务错误码定义，如商品不存在、SKU 不存在、SKU 编码冲突。
- `biz/handler/handler.go`：当前仍是预留文件；如果后续引入 service 内部适配层，可在这里承接。
- `biz/model/model.go`：当前仍是预留文件；如果后续需要独立领域模型，可放在这里。
- `biz/repository/repository.go`：仓储接口和 MySQL 实现，负责商品聚合的读写、事务更新、列表查询、SKU 查询。
- `biz/service/service.go`：`ProductService` 定义、构造函数和商品 / SKU 状态常量。
- `biz/service/mapper.go`：DAL 模型到 RPC 模型的转换，以及 repository 错误到业务错误的映射。
- `biz/service/build_models.go`：商品写入时的入参校验、SKU / 属性模型组装、ID 生成。
- `biz/service/create_product.go`：创建商品业务逻辑。
- `biz/service/update_product.go`：更新商品业务逻辑，当前按全量覆盖方式更新 SKU 集合。
- `biz/service/change_product_status.go`：修改商品上下架状态。
- `biz/service/get_product_detail.go`：读取商品详情，返回商品、SKU、销售属性。
- `biz/service/list_products.go`：商品列表查询，支持分页、状态、类目、关键字过滤。
- `biz/service/batch_get_sku.go`：按 SKU ID 批量读取 SKU 信息，供订单链路使用。
- `config/config.go`：商品服务配置加载，支持文件配置和 Apollo 扩展点。
- `config/product-service.local.yaml`：本地开发环境默认配置。
- `dal/db/db.go`：MySQL 连接初始化和连接池配置。
- `dal/db/migrate.go`：启动时执行数据库 migration。
- `dal/model/model.go`：GORM 持久化模型定义，对应 `products`、`product_skus`、`product_sku_attrs`，并包含 `creator_id`、`updated_by`。
- `migrations/000001_create_products.up.sql`：商品服务第一版建表脚本。
- `migrations/000001_create_products.down.sql`：第一版建表回滚脚本。
- `migrations/000002_create_product_skus.up.sql`：创建 SKU 表。
- `migrations/000002_create_product_skus.down.sql`：回滚 SKU 表。
- `migrations/000003_create_product_sku_attrs.up.sql`：创建 SKU 属性表。
- `migrations/000003_create_product_sku_attrs.down.sql`：回滚 SKU 属性表。
- `migrations/000004_add_product_owner_fields.up.sql`：增加商品创建者与最后更新人字段。
- `migrations/000004_add_product_owner_fields.down.sql`：回滚商品创建者与最后更新人字段。
- `migrations/000005_relax_product_sku_code_constraint.up.sql`：放宽 SKU 编码约束，取消全局唯一并补普通索引。
- `migrations/000005_relax_product_sku_code_constraint.down.sql`：回滚 SKU 编码约束变更。
- `rpc/main.go`：商品服务启动入口，只保留 `bootstrap.Run()` 调用。
- `rpc/bootstrap/bootstrap.go`：启动装配层，负责日志、OTel、配置、数据库、Snowflake、metrics、RPC Server 初始化。
- `rpc/handler/handler.go`：`ProductServiceImpl` 定义和构造函数。
- `rpc/handler/create_product.go`：`CreateProduct` RPC 入站处理。
- `rpc/handler/update_product.go`：`UpdateProduct` RPC 入站处理。
- `rpc/handler/change_product_status.go`：`ChangeProductStatus` RPC 入站处理。
- `rpc/handler/get_product_detail.go`：`GetProductDetail` RPC 入站处理。
- `rpc/handler/list_products.go`：`ListProducts` RPC 入站处理。
- `rpc/handler/batch_get_sku.go`：`BatchGetSku` RPC 入站处理。
- `script/start.sh`：本地启动脚本，统一注入环境变量并运行 `product-service`。

### 13.2 网关侧商品相关目录

```text
gateway/
├── internal/authz/
│   └── casbin.go
├── internal/handler/product/
│   ├── routes.go
│   ├── helpers.go
│   ├── create.go
│   ├── update.go
│   ├── change_status.go
│   ├── detail.go
│   └── list.go
├── internal/logic/product/
│   ├── create.go
│   ├── update.go
│   ├── change_status.go
│   ├── detail.go
│   └── list.go
├── internal/types/product.go
└── rpc/product/client.go
```

各文件说明如下：

- `gateway/internal/authz/casbin.go`：内嵌 Casbin model / policy，并对商品权限做统一判定。
- `gateway/internal/handler/product/routes.go`：注册商品相关 HTTP 路由。
- `gateway/internal/handler/product/helpers.go`：公共解析函数，如 `product_id` 路径参数解析。
- `gateway/internal/handler/product/create.go`：处理创建商品 HTTP 请求。
- `gateway/internal/handler/product/update.go`：处理更新商品 HTTP 请求。
- `gateway/internal/handler/product/change_status.go`：处理商品状态变更 HTTP 请求。
- `gateway/internal/handler/product/detail.go`：处理商品详情 HTTP 请求。
- `gateway/internal/handler/product/list.go`：处理商品列表 HTTP 请求。
- `gateway/internal/logic/product/create.go`：网关侧创建商品业务编排，负责参数检查和下游 RPC 调用。
- `gateway/internal/logic/product/update.go`：网关侧更新商品业务编排，并在写入前校验是否有权修改该商品。
- `gateway/internal/logic/product/change_status.go`：网关侧商品状态变更业务编排，并校验是否有权修改该商品。
- `gateway/internal/logic/product/detail.go`：网关侧商品详情业务编排，并根据商品状态和创建者做可见性控制。
- `gateway/internal/logic/product/list.go`：网关侧商品列表业务编排，对普通用户和管理员应用不同查询策略。
- `gateway/internal/types/product.go`：商品相关 HTTP 请求 / 响应结构定义。
- `gateway/rpc/product/client.go`：商品服务 Kitex Client 封装。

## 14. 可观测性设计

当前实现直接复用项目已有模式：

- 日志：`app/log`
- trace：`app/trace`
- metrics：`app/metrics`

当前优先关注的指标：

- 商品详情查询次数
- 商品列表查询次数
- 商品变更次数
- RPC 错误数
- 数据库错误数

当前埋点约定：

- `biz.module=product`
- `biz.action=create/update/list/detail`
- `product_id`
- `sku_id`
- `biz.success`
- `biz.code`

## 15. 超时治理

当前 `product-service` 已补齐数据库查询超时配置：

- 配置入口：`services/product-service/config/config.go`
- 本地配置示例：`services/product-service/config/product-service.local.yaml`
- 当前字段：`timeout.db_query_ms`

当前落点：

- `gateway` 调 `product-service` 的 Kitex Client 已显式设置 connect timeout 和 rpc timeout
- `product-service` repository 在商品查询、列表、状态变更和事务写入时，会统一套上 DB query timeout

这样做的目的：

- 避免商品查询和写入链路无限等待
- 让 `gateway -> rpc -> db` 这条调用链开始具备基础超时预算
- 为后续限流、重试和熔断提供更稳定的前提

## 16. 错误码设计

商品域当前已经使用独立错误码段：

- `2020001`：商品不存在
- `2020002`：SKU 不存在
- `2020005`：SKU 编码冲突
- `1000001`：请求参数错误
- `1009999`：系统内部错误

后续如果继续扩展，可以补充：

- 商品已下架
- SKU 不可售
- 商品价格非法

## 17. 当前实现说明

当前代码已经实现以下模块：

- `product-service` 的配置加载、MySQL 初始化、migration 执行、Snowflake ID 生成
- 商品领域的三张核心表：`products`、`product_skus`、`product_sku_attrs`
- `products` 已支持 `creator_id`、`updated_by`
- 商品 RPC 服务：
  - `CreateProduct`
  - `UpdateProduct`
  - `ChangeProductStatus`
  - `GetProductDetail`
  - `ListProducts`
  - `BatchGetSku`
- `gateway` 对外 HTTP 接口：
  - `GET /api/v1/products`
  - `GET /api/v1/products/detail/:product_id`
  - `POST /api/v1/admin/products`
  - `PUT /api/v1/admin/products/:product_id`
  - `POST /api/v1/admin/products/:product_id/status`
- `gateway` 权限控制：
  - JWT 中新增 `role` claim
  - 通过 Casbin 做商品访问授权
  - 普通用户仅能查询 `online`
  - 管理员仅能查询 / 修改自己创建的非公开商品

当前代码组织补充说明：

- `biz/service` 已按接口拆分：
  - `create_product.go`
  - `update_product.go`
  - `change_product_status.go`
  - `get_product_detail.go`
  - `list_products.go`
  - `batch_get_sku.go`
- `rpc` 入口层也按接口拆分：
  - `rpc/handler/create_product.go`
  - `rpc/handler/update_product.go`
  - `rpc/handler/change_product_status.go`
  - `rpc/handler/get_product_detail.go`
  - `rpc/handler/list_products.go`
  - `rpc/handler/batch_get_sku.go`
- `rpc/main.go` 不再直接承载初始化细节，只作为进程入口
- 启动期装配逻辑统一收敛在 `rpc/bootstrap/bootstrap.go`
- 公共转换逻辑放在 `biz/service/mapper.go`
- 商品写入时的模型组装与校验逻辑放在 `biz/service/build_models.go`

当前约定的状态值：

- 商品状态：
  - `0=draft`
  - `1=offline`
  - `2=online`
- SKU 状态：
  - `0=inactive`
  - `1=active`

## 18. HTTP 接口文档

说明：

- 所有接口统一返回 `code`、`message`、`data`、`trace_id`
- 成功时 `code=0`
- 当前管理端接口已接入 JWT 与 Casbin 权限控制

### 17.1 商品列表

- 方法：`GET`
- 路径：`/api/v1/products`

查询参数：

- `page`：页码，选填，默认 `1`
- `page_size`：每页数量，选填，默认 `20`，最大 `100`
- `status`：商品状态，选填
- `category_id`：类目 ID，选填
- `keyword`：标题 / 副标题 / 品牌关键字，选填

权限说明：

- 未登录用户和普通用户：不传 `status` 时默认只看 `online`
- 普通用户即使主动传 `status=0/1`，也不会获得草稿 / 下架数据
- 管理员查询 `status=0/1` 时，只能看到自己创建的商品

请求示例：

```http
GET /api/v1/products?page=1&page_size=10&status=2&keyword=iphone
```

成功响应示例：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "products": [
      {
        "id": 192000000000000001,
        "title": "iPhone 15",
        "sub_title": "A16 芯片",
        "category_id": 1001,
        "brand": "Apple",
        "status": 2,
        "min_sale_price": 599900,
        "cover_url": "https://cdn.example.com/iphone15-black.jpg"
      }
    ],
    "total": 1
  },
  "trace_id": "8f2d3f..."
}
```

### 17.2 商品详情

- 方法：`GET`
- 路径：`/api/v1/products/detail/:product_id`

请求示例：

```http
GET /api/v1/products/detail/192000000000000001
```

权限说明：

- `online` 商品：公开可见
- `draft/offline` 商品：只有创建者本人且具备管理员角色时可见

成功响应示例：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "id": 192000000000000001,
    "title": "iPhone 15",
    "sub_title": "A16 芯片",
    "category_id": 1001,
    "brand": "Apple",
    "description": "iPhone 15 商品详情",
    "status": 2,
    "skus": [
      {
        "id": 192000000000000101,
        "spu_id": 192000000000000001,
        "sku_code": "IP15-BLACK-128G",
        "title": "iPhone 15 黑色 128G",
        "sale_price": 599900,
        "market_price": 699900,
        "status": 1,
        "cover_url": "https://cdn.example.com/iphone15-black.jpg",
        "attrs": [
          {
            "id": 192000000000000201,
            "sku_id": 192000000000000101,
            "attr_name": "颜色",
            "attr_value": "黑色",
            "sort": 1
          }
        ]
      }
    ]
  },
  "trace_id": "8f2d3f..."
}
```

失败响应示例：

```json
{
  "code": 2020001,
  "message": "商品不存在",
  "trace_id": "8f2d3f..."
}
```

### 17.3 创建商品

- 方法：`POST`
- 路径：`/api/v1/admin/products`
- Content-Type：`application/json`

权限说明：

- 需要登录
- 仅管理员可访问
- `creator_id` 不由前端传入，由网关从当前登录用户写入

请求体示例：

```json
{
  "title": "iPhone 15",
  "sub_title": "A16 芯片",
  "category_id": 1001,
  "brand": "Apple",
  "description": "iPhone 15 商品详情",
  "status": 2,
  "skus": [
    {
      "sku_code": "IP15-BLACK-128G",
      "title": "iPhone 15 黑色 128G",
      "sale_price": 599900,
      "market_price": 699900,
      "status": 1,
      "initial_stock": 100,
      "cover_url": "https://cdn.example.com/iphone15-black.jpg",
      "attrs": [
        {
          "attr_name": "颜色",
          "attr_value": "黑色",
          "sort": 1
        },
        {
          "attr_name": "内存",
          "attr_value": "128G",
          "sort": 2
        }
      ]
    }
  ]
}
```

补充说明：

- `initial_stock` 是管理端创建商品时可选携带的初始库存
- 该字段只作为 `gateway` 编排库存初始化的输入，不会落到 `product-service` 自己的库表中
- 如果不传 `initial_stock`，当前按 `0` 处理，并仍然会在库存服务中创建对应库存记录
- `sku_code` 当前为可选字段；如果不传，系统仍会创建商品和库存记录
- 库存初始化当前按创建返回的 `sku_id` 顺序执行，不依赖 `sku_code` 做映射

成功响应示例：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "product_id": 192000000000000001,
    "skus": [
      {
        "id": 193000000000000001,
        "sku_code": "IP15-BLACK-128G"
      }
    ]
  },
  "trace_id": "8f2d3f..."
}
```

### 17.4 更新商品

- 方法：`PUT`
- 路径：`/api/v1/admin/products/:product_id`
- Content-Type：`application/json`

请求体说明：

- 结构与创建商品一致，但 `skus` 的语义是“更新后商品应保留的完整 SKU 集合”
- 如果某个 SKU 是当前商品下已经存在的 SKU，必须带上这个商品详情里查到的真实 `id`
- 如果某个 SKU 是本次新增的 SKU，不要传 `id`，或者传 `0`
- `attrs` 当前不要求传属性 `id`，服务端会按请求内容重建该 SKU 的属性集合
- 如果请求中缺失某个已有 SKU，则当前实现会视为删除该 SKU
- 如果传入了一个 `sku.id`，但它不属于当前 `product_id`，会返回 `2020002 / SKU 不存在`

当前补充约定：

- 当前删除 SKU 只会影响商品侧数据
- 不会自动删除库存服务中的库存记录
- 后续更合理的设计是：SKU 删除或失效时，同步把库存状态收口为 `frozen`
- `sku_code` 在更新接口中同样为可选字段；不传或传空表示该 SKU 没有业务编码
- 当前只校验同一次请求里的非空 `sku_code` 不重复，不再要求全局唯一

推荐调用顺序：

1. 先调用 `GET /api/v1/products/detail/:product_id` 查询商品详情
2. 从返回结果里拿到该商品下真实存在的 `skus[].id`
3. 更新已有 SKU 时，把这个真实 `id` 原样带回更新请求

典型场景：

- 更新已有 SKU：带真实 `sku.id`
- 新增 SKU：不带 `id`
- 删除 SKU：不要把该 SKU 放进本次请求的 `skus` 数组

权限说明：

- 需要登录
- 仅管理员可访问
- 且只能修改自己创建的商品

请求体示例：

```json
{
  "title": "iPhone 15",
  "sub_title": "A16 芯片升级版",
  "category_id": 1001,
  "brand": "Apple",
  "description": "更新后的商品详情",
  "status": 2,
  "skus": [
    {
      "id": 2031294457118203905,
      "sku_code": "IP15-BLACK-128G",
      "title": "iPhone 15 黑色 128G",
      "sale_price": 589900,
      "market_price": 699900,
      "status": 1,
      "cover_url": "https://cdn.example.com/iphone15-black.jpg",
      "attrs": [
        {
          "attr_name": "颜色",
          "attr_value": "黑色",
          "sort": 1
        },
        {
          "attr_name": "内存",
          "attr_value": "128G",
          "sort": 2
        }
      ]
    }
  ]
}
```

新增 SKU 示例：

```json
{
  "title": "iPhone 15",
  "sub_title": "A16 芯片升级版",
  "category_id": 1001,
  "brand": "Apple",
  "description": "更新后的商品详情",
  "status": 2,
  "skus": [
    {
      "id": 2031294457118203905,
      "sku_code": "IP15-BLACK-128G",
      "title": "iPhone 15 黑色 128G",
      "sale_price": 589900,
      "market_price": 699900,
      "status": 1,
      "cover_url": "https://cdn.example.com/iphone15-black.jpg",
      "attrs": [
        {
          "attr_name": "颜色",
          "attr_value": "黑色",
          "sort": 1
        },
        {
          "attr_name": "内存",
          "attr_value": "128G",
          "sort": 2
        }
      ]
    },
    {
      "sku_code": "IP15-WHITE-256G",
      "title": "iPhone 15 白色 256G",
      "sale_price": 699900,
      "market_price": 799900,
      "status": 1,
      "cover_url": "https://cdn.example.com/iphone15-white.jpg",
      "attrs": [
        {
          "attr_name": "颜色",
          "attr_value": "白色",
          "sort": 1
        },
        {
          "attr_name": "内存",
          "attr_value": "256G",
          "sort": 2
        }
      ]
    }
  ]
}
```

说明：

- 第一个 SKU 带 `id`，表示更新已有 SKU
- 第二个 SKU 不带 `id`，表示新增 SKU
- 如果只想保留部分已有 SKU，未出现在 `skus` 数组中的旧 SKU 会被删除

成功响应示例：

```json
{
  "code": 0,
  "message": "成功",
  "trace_id": "8f2d3f..."
}
```

### 17.5 修改商品状态

- 方法：`POST`
- 路径：`/api/v1/admin/products/:product_id/status`
- Content-Type：`application/json`

权限说明：

- 需要登录
- 仅管理员可访问
- 且只能修改自己创建的商品

请求体示例：

```json
{
  "status": 1
}
```

成功响应示例：

```json
{
  "code": 0,
  "message": "成功",
  "trace_id": "8f2d3f..."
}
```

## 19. RPC 接口文档

IDL：`idl/product.thrift`

当前 RPC 列表：

- `createProduct(CreateProductRequest) -> CreateProductResponse`
- `updateProduct(UpdateProductRequest) -> UpdateProductResponse`
- `changeProductStatus(ChangeProductStatusRequest) -> ChangeProductStatusResponse`
- `getProductDetail(GetProductDetailRequest) -> GetProductDetailResponse`
- `listProducts(ListProductsRequest) -> ListProductsResponse`
- `batchGetSku(BatchGetSkuRequest) -> BatchGetSkuResponse`

### 18.1 `createProduct`

核心字段：

- 商品基本信息：`title`、`sub_title`、`category_id`、`brand`、`description`、`status`
- SKU 列表：`skus`
- SKU 属性：`skus[].attrs`
- 创建者：`creator_id`

成功返回：

- `product_id`
- `base.code=0`

### 18.2 `updateProduct`

核心字段：

- `product_id`
- 其余字段与 `createProduct` 一致
- 操作人：`operator_id`

说明：

- 当前实现按“全量覆盖”方式更新商品及其 SKU 集合
- 请求中缺失的旧 SKU 会被删除
- 更新已有 SKU 时，`skus[].id` 必须是当前商品下真实存在的 SKU ID
- 新增 SKU 时，不要传 `skus[].id`
- 如果传入的 `skus[].id` 不属于当前 `product_id`，会返回 `2020002 / SKU 不存在`

### 18.3 `changeProductStatus`

核心字段：

- `product_id`
- `status`
- `operator_id`

### 18.4 `getProductDetail`

核心字段：

- `product_id`

成功返回：

- 商品主信息
- 全量 SKU 列表
- 每个 SKU 的销售属性列表
- 商品创建者 `creator_id` 会在 RPC 层返回，供 gateway 判权

### 18.5 `listProducts`

核心字段：

- `page`
- `page_size`
- `status`
- `category_id`
- `keyword`
- `creator_id`

成功返回：

- 商品列表摘要 `products`
- 总数 `total`

### 18.6 `batchGetSku`

核心字段：

- `sku_ids`

典型调用方：

- `order-service`
- `cart-service`

成功返回：

- 与请求 SKU ID 对应的 SKU 明细
- 每个 SKU 携带属性列表，便于订单服务直接生成商品快照

## 20. 未来设计设想

当前版本已经能支撑商品基础管理、列表详情查询和订单链路的 SKU 批量读取，但后续还有几个明确的演进方向。

### 18.1 领域模型演进

- 从当前轻量 `SPU + SKU + Attr` 模型，逐步扩展到类目属性模板、品牌中心、图片集、多媒体资源。
- 如果商品信息继续复杂化，可以把 `biz/model` 从预留目录变成真正的领域模型层，把 GORM 模型与业务模型彻底分离。

### 18.2 价格与营销演进

- 当前 `sale_price`、`market_price` 由商品服务直接维护。
- 后续如果引入活动价、会员价、阶梯价，建议把“商品主数据”和“价格计算”拆开，商品服务只保留基础标价，营销域负责最终成交价计算。

### 18.3 库存协作演进

- 当前商品服务只负责“是否可售”的商品侧判断，不保存库存字段。
- 后续可以在商品详情页聚合库存展示信息，但仍建议由 `inventory-service` 提供库存读接口，避免库存数据回写到商品库。

### 18.4 查询能力演进

- 当前列表查询仍是基于 MySQL 的分页和简单条件过滤。
- 后续如果要支持复杂搜索、排序、筛选、聚合统计，建议引入搜索引擎或独立查询索引，而不是持续堆在关系库查询里。

### 18.5 管理端能力演进

- 当前管理端接口已经接入管理员身份与创建者约束。
- 后续可以继续补操作审计日志、商品变更历史、发布审批流、资源级角色授权。

### 18.6 稳定性与性能演进

- 当前读路径已经适合第一阶段，但还没有做缓存、读写分离、热点商品保护。
- 后续可以按访问特征补充商品详情缓存、SKU 批量读取缓存、热点商品降级策略和更细粒度指标。
