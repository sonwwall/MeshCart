# Gateway 技术设计文档

## 1. 文档信息

- 系统：MeshCart Gateway
- 框架：Hertz（HTTP）+ Kitex（RPC）
- 文档类型：架构与实现规范

## 2. 目标与范围

### 2.1 目标

- 规范 Gateway 分层结构与职责边界
- 统一请求处理链路与错误处理模型
- 统一配置、依赖注入与模块扩展方式

### 2.2 范围

本规范覆盖以下目录：

- `gateway/cmd/gateway`
- `gateway/config`
- `gateway/internal/component`
- `gateway/internal/{handler,logic,middleware,svc,types}`
- `gateway/rpc`
- `app/common`（网关返回结构与错误对象）

## 3. 目录结构

```text
gateway/
├── cmd/gateway/main.go
├── config/config.go
├── internal/
│   ├── component/
│   │   ├── observability.go
│   │   └── server.go
│   ├── handler/
│   │   ├── routes.go
│   │   └── user/
│   │       ├── routes.go
│   │       ├── login.go
│   │       └── register.go
│   ├── logic/
│   │   └── user/
│   │       ├── login.go
│   │       └── register.go
│   ├── middleware/
│   │   ├── jwt.go
│   │   └── trace.go
│   ├── svc/
│   │   └── service_context.go
│   └── types/
│       └── user.go
└── rpc/
    └── user/
        └── client.go
```

## 4. 分层职责

### 4.1 启动层（`cmd`）

职责：

- 读取配置
- 编排组件初始化顺序
- 初始化 `ServiceContext`
- 启动服务

说明：

- `cmd/gateway/main.go` 只保留启动主流程，不承载日志、OTel、HTTP Server 的具体构造细节
- 具体初始化逻辑下沉到 `internal/component`

### 4.2 配置层（`config`）

职责：

- 定义配置结构体
- 读取环境变量并生成运行时配置

当前配置按职责分组：

- `App`：服务名、环境
- `Log`：日志级别、日志目录
- `Telemetry`：OTLP 上报地址、是否 insecure
- `Metrics`：metrics 暴露地址与路径
- `Server`：网关 HTTP 监听地址
- `UserRPC`：下游用户服务连接与服务发现
- `JWT`：登录态签发与刷新配置

这样做的目的：

- `cmd` 与 `component` 不再各自读环境变量
- 启动期配置有单一来源，便于排障和后续切换配置源

### 4.3 组件初始化层（`internal/component`）

职责：

- 初始化日志
- 初始化 OTel Provider
- 构造 Hertz Server
- 组装 tracing / metrics / route 注册

边界：

- 这里只负责“通用启动组件”的装配
- 不承载具体业务逻辑
- 不直接持有业务状态，业务依赖仍由 `svc.ServiceContext` 管理

### 4.4 接口层（`internal/handler`）

职责：

- HTTP 参数绑定与基础校验
- 调用 `logic`
- 输出标准化响应

禁止：

- 直接访问下游 RPC/数据库
- 编排业务流程

### 4.5 业务编排层（`internal/logic`）

职责：

- 业务规则处理
- 下游服务调用编排
- 业务错误码映射

### 4.6 客户端层（`rpc/<module>`）

职责：

- 维护下游 Kitex Client
- 执行 RPC 调用
- 协议对象转换（Thrift <-> 网关内部结构）

禁止：

- 承载业务规则判断

### 4.7 依赖容器层（`internal/svc`）

职责：

- 集中初始化与持有共享依赖（配置、RPC Client）
- 向 `handler/logic` 提供依赖注入入口

### 4.8 协议类型层（`internal/types`）

职责：

- 维护 HTTP 请求/响应结构体
- 与 Thrift 生成结构解耦

## 5. 请求处理链路

登录接口链路：

1. `handler` 接收 `POST /api/v1/user/login`
2. `handler` 绑定请求体到 `types.UserLoginRequest`
3. `handler` 调用 `userlogic.NewLoginLogic(ctx, svcCtx).Login(&req)`
4. `logic` 执行业务校验并调用 `svcCtx.UserClient.Login(...)`
5. `rpc/user/client.go` 发起 Kitex 调用并返回网关内部结构
6. `logic` 根据返回码进行成功/失败判定
7. `logic` 调用 `svcCtx.JWT.TokenGenerator(...)` 生成 JWT
8. `handler` 通过 `common.Success/Fail` 输出标准 JSON

## 6. 响应与错误规范

### 6.1 HTTP 响应结构

统一返回结构（见 `app/common/response.go`）：

- `code`
- `message`
- `data`
- `trace_id`

### 6.2 错误对象

统一错误对象（见 `app/common/errors.go`）：

- `BizError{Code, Msg}`

### 6.3 处理规则

- `handler` 仅处理输入错误与输出映射
- `logic` 负责业务错误码生成
- `rpc` 仅返回技术错误（网络错误、超时、响应结构异常）

## 7. RPC 返回约束

针对 user-service `login`：

- 必须返回 `base.BaseResponse`
- 成功：`base.code == 0`
- 失败：`base.code != 0`
- `resp` 或 `resp.base` 缺失视为异常响应

说明：

- `resp.base != nil` 为结构安全校验（防空指针）
- 成功判定依据为 `code`，不是 `base` 是否存在

## 8. 配置规范

当前配置源为环境变量（见 `gateway/config/config.go`）：

- `APP_NAME`：服务名，默认 `gateway`
- `APP_ENV`：运行环境，默认 `dev`
- `LOG_LEVEL`：日志级别，默认 `info`
- `LOG_DIR`：日志目录，默认 `logs`
- `OTEL_EXPORTER_OTLP_ENDPOINT`：OTLP 上报地址，默认 `localhost:4319`
- `OTEL_EXPORTER_OTLP_INSECURE`：是否使用 insecure OTLP 连接，默认 `true`
- `GATEWAY_PROM_ADDR`：Gateway metrics 暴露地址，默认 `:9092`
- `GATEWAY_PROM_PATH`：Gateway metrics 路径，默认 `/metrics`
- `GATEWAY_ADDR`：网关监听地址
- `USER_RPC_SERVICE`：用户服务名
- `USER_RPC_ADDR`：用户服务地址
- `USER_RPC_DISCOVERY`：下游服务发现模式
- `USER_RPC_CONNECT_TIMEOUT_MS`：用户服务 RPC 建连超时，默认 `500ms`
- `USER_RPC_TIMEOUT_MS`：用户服务 RPC 调用超时，默认 `2000ms`
- `PRODUCT_RPC_CONNECT_TIMEOUT_MS`：商品服务 RPC 建连超时，默认 `500ms`
- `PRODUCT_RPC_TIMEOUT_MS`：商品服务 RPC 调用超时，默认 `2000ms`
- `CONSUL_ADDR`：Consul 地址
- `JWT_SECRET`：JWT 签名密钥
- `JWT_ISSUER`：JWT 签发者
- `JWT_TIMEOUT_MINUTES`：访问令牌过期时间（分钟）
- `JWT_MAX_REFRESH_MINUTES`：刷新窗口（分钟）

补充说明：

- 当前网关已经把下游 RPC connect timeout 与 rpc timeout 收口到 `config.Config`
- HTTP 入口超时暂未在 Hertz 启动层显式配置，后续可继续补齐

启动关系：

- `config.Load()` 负责把环境变量装配成 `config.Config`
- `component.InitLogger/InitOpenTelemetry/NewGatewayServer` 只消费 `config.Config`
- 这样 `main` 中不再散落 `getEnv(...)` 调用

## 9. JWT 设计

### 9.1 设计目标

- 由 `gateway` 统一签发和校验 HTTP 登录态
- 后续受保护接口只需要复用统一中间件，不重复实现 token 解析逻辑
- 把 JWT 配置集中在网关层，而不是分散到业务 handler

### 9.2 实现位置

JWT 相关实现分布在以下位置：

- `gateway/internal/middleware/jwt.go`
  - 初始化 Hertz JWT 中间件
  - 约定 claims 字段
  - 统一未登录响应
  - 统一 refresh token 响应
- `gateway/internal/svc/service_context.go`
  - 初始化并持有共享的 JWT 中间件实例
- `gateway/internal/logic/user/login.go`
  - 登录成功后签发 token
- `gateway/internal/handler/user/routes.go`
  - 给受保护路由挂载 JWT 中间件

### 9.3 使用的库

当前使用 `github.com/hertz-contrib/jwt`。

原因：

- 它直接适配 Hertz 中间件模型
- 提供 token 生成、解析、刷新和 claims 提取能力
- 能和当前网关路由分组方式自然集成

### 9.4 Token 字段设计

当前 JWT payload 里写入以下业务字段：

- `user_id`
- `username`
- `iss`

除此之外，JWT 中间件会自动写入标准时间相关字段：

- `exp`：过期时间
- `orig_iat`：原始签发时间

字段来源：

- `user_id`、`username`：登录成功后由 `gateway/internal/logic/user/login.go` 从登录结果中组装
- `iss`：来自 `JWT_ISSUER`
- `exp`、`orig_iat`：由 `hertz-contrib/jwt` 根据配置自动生成

### 9.5 Token 返回格式

当前返回给客户端的 token 使用标准格式：

```text
Bearer <jwt-token>
```

这意味着客户端可以直接把登录接口或刷新接口返回的 `token` 原样写入：

```text
Authorization: Bearer <jwt-token>
```

不需要客户端自己再拼接 `Bearer ` 前缀。

### 9.6 过期时间设计

当前默认配置：

- 访问令牌过期时间：`120` 分钟
- 最大刷新窗口：`720` 分钟

对应环境变量：

- `JWT_TIMEOUT_MINUTES`
- `JWT_MAX_REFRESH_MINUTES`

含义：

- `JWT_TIMEOUT_MINUTES` 控制单个 access token 的有效期
- `JWT_MAX_REFRESH_MINUTES` 控制 token 自首次签发后，最长还能被刷新多久

### 9.7 在哪里修改

如果要调整 token 设计，修改入口如下：

- 修改过期时间或 issuer：
  - `gateway/config/config.go`
  - `gateway/script/start.sh`
- 修改 claims 字段：
  - `gateway/internal/middleware/jwt.go` 中的 `PayloadFunc`
- 修改 token 返回格式：
  - `gateway/internal/middleware/jwt.go` 中的 `FormatBearerToken`
- 修改登录成功后的签发逻辑：
  - `gateway/internal/logic/user/login.go`
- 修改哪些接口需要鉴权：
  - `gateway/internal/handler/user/routes.go`

### 9.8 当前受保护接口

当前已经接入 JWT 鉴权的接口：

- `GET /api/v1/user/me`

当前已接入 token 刷新能力的接口：

- `GET /api/v1/user/refresh_token`

### 9.9 默认行为说明

- 网关启动后会初始化一个全局 JWT 中间件实例，并放入 `ServiceContext`
- 登录成功后由网关签发 token，不依赖 `user-service` 返回 token
- 受保护接口通过 `Authorization` Header 解析 token
- token 解析失败、缺失或过期时，统一返回 `1000002`，即“未登录或登录已过期”

### 9.10 未来升级方案：双 Token

当前实现是单 token 方案，适合项目现阶段：

- 实现简单
- 网关无状态
- 接入和调试成本低

如果后续出现以下需求，建议升级为双 token 方案：

- 需要真正的登出即失效
- 需要多端登录管理
- 需要踢人下线或强制会话失效
- 需要缩短 access token 有效期，同时保留较长登录态
- 需要降低 token 泄漏后的风险窗口

升级后的目标方案：

- `access_token`
  - 使用 JWT
  - 有效期较短，建议 `15` 到 `30` 分钟
  - 只承载最小身份信息
- `refresh_token`
  - 使用高熵随机串，推荐不要复用 access token 的 JWT
  - 有效期较长，建议 `7` 到 `30` 天
  - 服务端维护白名单或会话表

推荐的 claims / 会话字段设计：

- `access_token` 中保留：
  - `user_id`
  - `username`
  - `session_id`
  - `iss`
  - `exp`
- `refresh_token` 服务端存储中保留：
  - `token_id`
  - `user_id`
  - `session_id`
  - `device_id` 或客户端标识
  - `expire_at`
  - `status`

推荐的服务端存储设计：

- 优先使用 Redis 维护 refresh token 白名单
- key 可设计为：`auth:refresh:<token_id>`
- value 存储用户、会话、设备、过期时间和状态

推荐的刷新流程：

1. 登录成功后同时签发 `access_token` 和 `refresh_token`
2. `refresh_token` 写入 Redis 白名单
3. 客户端携带 `refresh_token` 请求刷新
4. 服务端校验白名单、状态和过期时间
5. 刷新成功后执行 rotation：
   - 生成新的 `access_token`
   - 生成新的 `refresh_token`
   - 旧 `refresh_token` 立即作废

推荐新增接口：

- `POST /api/v1/user/login`
  - 返回 `access_token` 和 `refresh_token`
- `POST /api/v1/user/refresh_token`
  - 使用 `refresh_token` 换新 token
- `POST /api/v1/user/logout`
  - 主动作废当前 session
- `POST /api/v1/user/logout_all`
  - 作废用户全部 session

升级后的职责拆分建议：

- `gateway`
  - 继续负责 access token 的签发与校验
  - 负责 HTTP 层鉴权中间件
- `auth` 相关基础设施
  - 负责 refresh token 白名单存储
  - 负责 refresh rotation
  - 负责会话吊销

为了便于未来平滑升级，当前阶段建议遵守以下约束：

- 不把过多业务数据塞进 access token
- 后续新增 claims 时优先考虑 `session_id`
- 受保护接口只依赖统一鉴权中间件，不在业务代码里手写 token 解析
- 刷新接口的语义保持独立，避免把 access token 刷新逻辑散落到业务接口中

当前不立即切到双 token 的原因：

- 当前项目还处于基础能力建设阶段
- 现阶段没有 Redis 会话白名单和登出/踢下线需求
- 单 token 方案能先满足大多数接口联调和登录态验证需求

## 10. 模块扩展规范

### 10.1 新增接口（同模块）

1. 在 `internal/types` 增加请求/响应结构
2. 在 `internal/logic/<module>` 增加业务文件
3. 在 `internal/handler/<module>` 增加 handler 文件
4. 在 `internal/handler/<module>/routes.go` 注册路由
5. 在 `rpc/<module>` 增加对应客户端方法

### 10.2 新增业务模块

1. 新建 `internal/handler/<module>`
2. 新建 `internal/logic/<module>`
3. 新建 `internal/types/<module>.go`
4. 新建 `rpc/<module>/client.go`
5. 在 `internal/svc/service_context.go` 注入模块客户端
6. 在 `internal/handler/routes.go` 聚合模块路由

## 11. 代码约束

- `handler` 不直接访问 `rpc` 生成客户端实现细节
- `logic` 不依赖 HTTP 细节（Header、状态码）
- `rpc` 不处理业务语义分支
- 所有对外响应必须走标准化返回结构
- 错误码遵循 `docs/error-code.md`
