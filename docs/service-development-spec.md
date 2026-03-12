# MeshCart 服务开发设计规范

## 1. 目的

本文档用于约束 MeshCart 后续新增 `gateway` 模块能力、RPC 服务、通用组件时的设计与落地方式。

目标：

- 统一新服务的目录结构与职责边界
- 统一 `gateway -> rpc -> biz -> dal` 的实现风格
- 统一可观测性、微服务治理、错误码、配置等横切规范
- 让后续代码生成、脚手架扩展、人工补代码时有稳定参照

本文档不替代各服务自己的模块设计文档。

关系如下：

- 本文档：定义“所有新服务都应遵守”的总规范
- 模块设计文档：描述某个具体服务或业务域的边界与规则

## 2. 适用范围

当前适用于以下代码区域：

- `gateway/`
- `services/*-service/`
- `app/`
- `docs/`

当后续新增如 `order-service`、`cart-service`、`inventory-service`、`payment-service` 的完整实现时，默认应遵守本文档。

## 3. 总体架构约定

MeshCart 当前采用：

- `gateway` 对外提供 HTTP
- `services/*-service` 对内提供 Kitex RPC
- `gateway` 通过 RPC 调用下游服务
- 服务内部按 `biz + dal + rpc` 分层
- 可观测性统一复用 `app/log`、`app/trace`、`app/metrics`
- 服务发现统一复用 Consul 约定
- 已落地的微服务治理统一沿用当前 timeout 规范

新增服务时，优先保持和 `user-service`、`product-service` 同风格，不要为单个服务引入新分层模型。

## 4. 目录规范

### 4.1 Gateway

`gateway` 目录当前标准结构：

```text
gateway/
├── cmd/gateway/main.go
├── config/
├── internal/
│   ├── authz/
│   ├── component/
│   ├── handler/
│   ├── logic/
│   ├── middleware/
│   ├── svc/
│   └── types/
├── rpc/
└── script/
```

各层职责：

- `cmd/gateway`
  - 只保留启动入口
- `config`
  - 网关配置结构、环境变量解析
- `internal/component`
  - 日志、OTel、HTTP Server、metrics 等组件装配
- `internal/handler`
  - HTTP 参数绑定、调用 logic、统一响应
- `internal/logic`
  - 网关侧业务编排、下游 RPC 调用、错误映射
- `internal/middleware`
  - JWT、超时、trace 辅助、后续限流等横切能力
- `internal/svc`
  - 依赖容器，集中持有配置、RPC client、鉴权组件
- `internal/types`
  - HTTP 请求/响应结构
- `rpc/<module>`
  - 对下游服务的 Kitex client 封装
- `script`
  - 本地启动、生成代码、辅助脚本

约束：

- 新增 HTTP 接口时，优先按业务模块在 `handler/<module>` 和 `logic/<module>` 下扩展
- 不要把业务逻辑写进 `handler`
- 不要让 `handler` 直接调下游 RPC client
- `gateway/rpc` 只做 client 封装和协议转换，不写业务判断

### 4.2 RPC 服务

完整 RPC 服务当前标准结构参考 `user-service` 和 `product-service`：

```text
services/<service>-service/
├── cmd/<service>/main.go
├── config/
├── rpc/
│   ├── main.go
│   ├── bootstrap/
│   ├── handler/
│   └── script/
├── biz/
│   ├── dto/
│   ├── errno/
│   ├── handler/
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

各层职责：

- `cmd/<service>`
  - 服务主入口
- `config`
  - 服务配置加载
- `rpc/main.go`
  - 只保留 `bootstrap.Run()`
- `rpc/bootstrap`
  - 启动期装配：日志、OTel、migration、MySQL、metrics、Kitex server、Consul 注册
- `rpc/handler`
  - RPC 入站方法，按接口拆文件
- `biz/service`
  - 核心业务逻辑
- `biz/repository`
  - 数据访问封装
- `biz/dto`
  - 业务传输对象
- `biz/model`
  - 业务领域模型
- `biz/errno`
  - 服务私有错误码
- `biz/handler`
  - 可选的业务层适配器；如无明确收益，不要增加额外包装
- `dal/db`
  - MySQL 初始化、migration
- `dal/model`
  - GORM 模型
- `dal/redis`
  - Redis 访问封装
- `migrations`
  - 数据库迁移文件

约束：

- `rpc/main.go` 必须保持轻量
- `rpc/bootstrap` 只负责启动装配，不写业务逻辑
- `rpc/handler` 只做 RPC 请求转发、参数转换、统一响应装配
- `biz/service` 才是业务规则核心位置
- `repository` 不直接返回给网关使用

### 4.3 共享能力层

`app/` 是跨服务共享能力层。

当前约定：

- `app/common`
  - 通用错误、统一响应结构、基础常量
- `app/log`
  - 统一日志初始化与 `context` 绑定
- `app/trace`
  - OTel 初始化、trace 工具、Hertz trace 适配
- `app/metrics`
  - Prometheus 指标注册与暴露辅助
- `app/middleware`
  - 预留跨服务中间件能力
- `app/mq`
  - 预留消息队列统一封装
- `app/xconfig`
  - 预留统一配置封装

约束：

- 跨服务都会用到的能力，优先收敛到 `app/`
- 只属于某一个服务的能力，不要提前塞进 `app/`
- `app/` 中组件应当偏基础设施，不承载业务域规则

## 5. 分层职责规范

### 5.1 Gateway Handler

允许：

- 绑定请求
- 基础参数校验
- 获取 trace_id、identity
- 调用 logic
- 返回统一 JSON

禁止：

- 直接访问数据库
- 直接写业务判断
- 直接调用多个下游服务拼复杂流程

### 5.2 Gateway Logic

允许：

- 网关侧业务编排
- 调用一个或多个 RPC client
- 对技术错误做统一映射
- 组装最终返回对象

禁止：

- 直接访问 `dal`
- 承担长事务逻辑

### 5.3 RPC Handler

允许：

- RPC 请求转 DTO
- 调用 `biz/service`
- DTO/业务对象转 RPC 响应

禁止：

- 在 RPC 层实现核心业务规则
- 在 RPC 层直接写数据库逻辑

### 5.4 Biz Service

允许：

- 核心业务规则
- 领域校验
- 事务编排
- 调用 repository

要求：

- 业务错误优先在这里收口
- 技术错误要打日志

### 5.5 Repository

允许：

- GORM/Redis 查询和写入
- 数据映射
- 统一超时、分页、基础查询封装

禁止：

- 业务规则判断
- 组装对外接口响应

## 6. 配置规范

### 6.1 配置来源

当前项目默认采用：

- 服务配置文件
- 环境变量覆盖

约束：

- 启动期配置必须通过 `config.Config` 收口
- 不要在 `main`、`bootstrap`、`handler`、`logic` 中散落大量 `os.Getenv`
- 允许在 bootstrap 内少量读取“仅启动期使用”的环境变量，但优先考虑下沉到 `config`

### 6.2 配置分组建议

新增服务时，配置结构优先按以下分组：

- `App`
- `Log`
- `Telemetry`
- `Metrics`
- `Server`
- `RateLimit`
- `MySQL`
- `Redis`
- `Migration`
- `Snowflake`
- `Timeout`
- `<Downstream>RPC`

### 6.3 命名约定

环境变量命名建议：

- 网关：`GATEWAY_*`
- 用户服务：`USER_*`
- 商品服务：`PRODUCT_*`
- 新服务保持同一模式，例如：
  - `ORDER_RPC_SERVICE`
  - `ORDER_SERVICE_ADDR`
  - `ORDER_METRICS_ADDR`

## 7. RPC 与服务发现规范

### 7.1 服务名

统一使用：

- `meshcart.user`
- `meshcart.product`
- 后续服务按同样规则：
  - `meshcart.order`
  - `meshcart.cart`
  - `meshcart.inventory`

不要混用：

- `UserService`
- `user-service`
- `meshcart.user`

一个服务只保留一套稳定 RPC 服务名。

### 7.2 发现模式

统一保留双态：

- `consul`
- `direct`

约束：

- 默认优先走 `consul`
- 出现发现问题时允许临时回退 `direct`
- 新服务的 client 与 server 都应遵循当前 Consul 注册与发现约定

### 7.3 Gateway RPC Client 规范

每个下游服务在 `gateway/rpc/<module>` 下独立建 client 包。

要求：

- `NewClient(...)` 负责统一创建 Kitex client
- 显式配置 `ConnectTimeout` 和 `RPCTimeout`
- 统一挂 resolver、trace suite、必要的 dialer/mock 扩展位
- 对响应结构做安全校验
- 不在 client 层写业务规则

## 8. 可观测性规范

### 8.1 日志

统一使用 `app/log`。

要求：

- 使用结构化日志
- 业务链路日志优先通过 `ctx` 传递 trace 字段
- 技术错误必须记录真实 error
- 业务失败记录业务 code/message，但不要把正常业务失败都打成 error

建议字段：

- `trace_id`
- `span_id`
- `service`
- `module`
- `action`
- `user_id`
- `code`

### 8.2 Trace

统一使用 OpenTelemetry。

要求：

- HTTP server span 由 Hertz tracing middleware 自动创建
- RPC client/server span 由 Kitex tracing suite 自动创建
- 业务层只在必要位置补 internal span
- 不重复手写 HTTP/RPC 入站基础 span

internal span 命名建议：

- `<domain>.<layer>.<module>.<action>`

示例：

- `gateway.logic.user.login`
- `product.biz.create_product`

### 8.3 Metrics

统一通过 `app/metrics` 暴露 Prometheus 指标。

要求：

- `gateway` 暴露 HTTP metrics
- RPC 服务暴露 RPC requests total / duration / errors total
- 新服务的 metrics 端口单独配置，不与业务端口混用

### 8.4 文档同步

新增可观测性接入点时，需要同步更新：

- `docs/logging-tracing.md`
- 对应服务设计文档

## 9. 微服务治理规范

### 9.1 当前已落地能力

后续新增服务默认应复用当前已落地治理规范：

- HTTP 入口 read/write/idle timeout
- HTTP request-level 协作式 timeout
- 网关入口限流第一阶段实现
- `gateway -> rpc` 的 connect timeout / rpc timeout
- `service -> db` 的 query timeout

当前网关限流约定：

- 第一阶段只在 `gateway` 落地，不在 RPC 层重复实现一套限流
- 第一阶段使用单机内存 limiter store，不依赖 Redis
- 当前对整个 `/api/v1` 先挂一层宽松的全局 IP 限流
- 当前已保护的入口包括：
  - `/api/v1/*`
  - `POST /api/v1/user/login`
  - `POST /api/v1/user/register`
  - 商品管理写接口
- 对写接口优先按用户限流，对匿名入口优先按 IP 限流
- 限流 key 应优先使用稳定路由模板，不直接使用带路径参数的原始 URL

当前限流配置字段含义：

- `GATEWAY_RATE_LIMIT_ENABLED`
  - 网关限流总开关
- `GATEWAY_RATE_LIMIT_ENTRY_TTL_MS`
  - 单个 limiter key 在内存中的保留时间
- `GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL_MS`
  - 过期限流桶的清理周期
- `GATEWAY_GLOBAL_IP_RATE_LIMIT_RPS` / `GATEWAY_GLOBAL_IP_RATE_LIMIT_BURST`
  - 整个 `/api/v1` 按 IP 维度的全局宽松限流速率和突发容量
- `GATEWAY_LOGIN_IP_RATE_LIMIT_RPS` / `GATEWAY_LOGIN_IP_RATE_LIMIT_BURST`
  - 登录接口按 IP 限流的持续速率和突发容量
- `GATEWAY_REGISTER_IP_RATE_LIMIT_RPS` / `GATEWAY_REGISTER_IP_RATE_LIMIT_BURST`
  - 注册接口按 IP 限流的持续速率和突发容量
- `GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_RPS` / `GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_BURST`
  - 商品管理写接口按用户限流的持续速率和突发容量
- `GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_RPS` / `GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_BURST`
  - 商品管理写接口按路由限流的持续速率和突发容量

使用约束：

- `RPS` 控制持续吞吐上限
- `Burst` 控制短时间可容忍的峰值
- 调大 `Burst` 不等于长期吞吐变高，只表示允许更大的瞬时波峰

### 9.2 预算关系

当前默认预算：

- HTTP read timeout：`5000ms`
- HTTP write timeout：`5000ms`
- HTTP idle timeout：`60000ms`
- HTTP request timeout：`3000ms`
- RPC connect timeout：`500ms`
- RPC timeout：`2000ms`
- DB query timeout：`1500ms`

约束：

- DB timeout < RPC timeout < HTTP request timeout
- 如果新服务需要不同预算，必须说明原因并更新文档

### 9.3 当前边界

当前 request timeout 是协作式超时，不是强制打断型超时。

这意味着：

- 对会尊重 `ctx.Done()` 的链路有效
- 对完全不理会 `ctx` 的纯阻塞逻辑无效

新增服务时应尽量让：

- biz/service 尊重传入 `ctx`
- repository 基于 `ctx` 做 DB timeout
- 下游 RPC 调用保持 `ctx` 透传

### 9.4 后续治理扩展

当前未落地但未来可能统一接入的能力：

- 重试
- 熔断
- 隔离
- RPC 层自保护限流

约束：

- 新增网关写接口时，应评估是否需要挂入现有限流中间件
- 不要在某个服务里单独引入另一套限流框架或 Redis 限流实现
- 如果后续需要分布式限流，应在当前中间件抽象上扩展 store，而不是重写路由接线

新增服务时不要自行引入另一套治理框架，优先等仓库统一方案。

## 10. 错误码与响应规范

### 10.1 HTTP 统一响应

对外 HTTP 响应统一使用：

- `code`
- `message`
- `data`
- `trace_id`

当前业务错误默认仍使用 `HTTP 200`，真实结果通过 `code/message` 表达。

### 10.2 错误码分层

统一遵循 [docs/error-code.md](/Users/ruitong/GolandProjects/MeshCart/docs/error-code.md)。

要求：

- 通用错误放 `app/common/errors.go`
- 服务私有错误放 `services/<service>/biz/errno/errors.go`
- 新模块先登记模块号，再新增错误码

### 10.3 技术错误映射

当前 gateway 已统一处理：

- timeout -> `服务繁忙，请稍后重试`
- connection unavailable -> `下游服务暂不可用，请稍后重试`
- 其他技术错误 -> `系统内部错误`

新增网关 logic 时，应复用统一映射 helper，不要手写分散判断。

## 11. 测试规范

### 11.1 新增服务至少应具备的测试

- 业务层单测
- repository 单测
- 关键 RPC 行为测试
- 关键治理行为测试

### 11.2 当前已形成的测试风格

可直接复用的思路：

- 真实 RPC timeout 测试
  - 本地起测试 Kitex server
  - 人为制造慢响应或建连阻塞
- 真实 DB timeout 测试
  - 内存 sqlite + GORM callback 阻塞到 `ctx.Done()`
- HTTP timeout 测试
  - 原始 TCP/本地 Hertz server 验证框架层 timeout
  - 中间件行为测试验证 request-level timeout
- 错误映射测试
  - 验证 timeout/unavailable 到对外文案的统一映射

### 11.3 新增治理能力时的要求

只加配置、不补行为测试，不算完成。

新增如下能力时，至少要补一个真实行为测试：

- timeout
- 限流
- 重试
- 熔断

## 12. 新服务落地清单

新增一个完整服务时，至少检查以下事项：

1. 目录是否符合 `cmd/config/rpc/biz/dal/migrations/script`
2. `rpc/main.go` 是否已收敛为 `bootstrap.Run()`
3. `rpc/handler` 是否按接口拆文件
4. 配置是否进入 `config.Config`
5. 是否接入日志、trace、metrics
6. 是否接入 Consul 服务发现约定
7. 是否接入当前 timeout 治理
8. 是否定义服务私有错误码
9. 是否补最小行为测试
10. 是否同步更新文档

## 13. 文档同步规范

当新增服务或新增跨服务组件时，除了代码本身，还应同步更新文档。

最少同步范围：

- 本文档：如果改动影响通用规范
- `docs/architecture.md`：如果新增了服务或新的全局组件
- 对应服务设计文档：如果新增或变更具体服务能力
- `docs/logging-tracing.md`：如果改动可观测性接入方式
- `docs/microservice-governance.md`：如果改动治理基线
- `docs/error-code.md`：如果新增模块号或错误码规范
- `docs/api.md`：如果对外 HTTP 接口发生变化

规则：

- 只影响单一服务的改动，不要反向污染全局规范
- 一旦某个能力进入多个服务复用，应尽快上升到本文档

## 14. 当前推荐做法

当前仓库新增功能时，优先按下面顺序考虑：

1. 先判断这是通用能力还是单服务能力
2. 通用能力优先沉到 `app/` 或全局规范
3. 单服务能力先落在服务内，再观察是否值得抽象
4. 先复用当前 `gateway`、`user-service`、`product-service` 的已验证模式
5. 只有现有模式明显不够用时，才引入新的结构或组件
