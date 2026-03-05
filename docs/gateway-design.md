# Gateway 分层设计说明

## 1. 设计目标

当前 Gateway 采用分层 + 依赖注入设计，目标是：

- 保持 `handler` 轻量，避免业务逻辑堆积
- 支持模块化扩展（`user`、`order`、`product` 等）
- 统一依赖管理（RPC Client、配置、日志、缓存）
- 方便单元测试与后续替换实现（mock -> 真实 RPC）

## 2. 目录结构与职责

```text
gateway/
├── cmd/gateway/main.go
├── config/config.go
├── internal/
│   ├── handler/
│   │   ├── routes.go
│   │   └── user/
│   │       ├── routes.go
│   │       └── login.go
│   ├── logic/
│   │   └── user/
│   │       └── login.go
│   ├── middleware/
│   │   └── trace.go
│   ├── svc/
│   │   └── service_context.go
│   └── types/
│       └── user.go
└── rpc/
    └── user/
        └── client.go
```

### 2.1 `cmd/gateway/main.go`

- 服务启动入口
- 负责初始化配置、ServiceContext、路由注册和 Hertz 启动

### 2.2 `config`

- 配置定义和加载（当前使用环境变量）
- 统一管理网关监听地址、下游服务地址等

### 2.3 `internal/handler`

- HTTP 层，只负责：
  - 参数绑定与基础校验
  - 调用 `logic`
  - 输出标准化响应
- 不承载核心业务逻辑

### 2.4 `internal/logic`

- 业务编排层，负责：
  - 业务规则校验
  - 调用下游 RPC
  - 组装返回数据
- `handler` 通过 `NewXxxLogic(ctx, svcCtx).Method(...)` 调用

### 2.5 `internal/svc/ServiceContext`

- 网关依赖容器，集中管理：
  - 配置
  - RPC Client
  - 后续可扩展：Redis、Logger、限流器、熔断器等
- 避免全局变量，支持测试替换

### 2.6 `internal/types`

- HTTP 请求/响应结构体定义
- 与 handler 解耦，便于复用和文档维护

### 2.7 `rpc/<module>`

- 下游服务客户端封装层
- 当前 user 使用 mock client，后续可替换为真实 kitex client

## 3. 为什么 `handler` 里不是直接写业务

示例调用：

```go
loginLogic := userlogic.NewLoginLogic(ctx, svcCtx)
data, bizErr := loginLogic.Login(&req)
```

这样写的原因：

- 依赖收敛：`ctx/svcCtx` 在构造时注入，方法参数更清晰
- 职责分离：HTTP 协议处理和业务编排分开
- 可测试：logic 层可以独立单测
- 易扩展：同模块多个接口可复用同一套依赖

## 4. 请求链路

`main -> handler.Register -> handler(user/login) -> logic(user/login) -> rpc/user client -> response`

其中：

- `handler` 决定路由与返回格式
- `logic` 决定业务流程
- `rpc` 决定下游调用细节

## 5. 使用方法

### 5.1 新增一个用户接口（示例：`/api/v1/user/profile`）

1. 在 `internal/types/user.go` 新增请求/响应结构
2. 在 `internal/logic/user/` 新增 `profile.go`
3. 在 `internal/handler/user/` 新增 `profile.go`
4. 在 `internal/handler/user/routes.go` 注册路由
5. 需要下游调用时，在 `rpc/user/client.go` 增加方法

### 5.2 新增一个业务模块（示例：`order`）

1. 新建 `internal/handler/order/` 和 `internal/logic/order/`
2. 新建 `internal/types/order.go`
3. 新建 `rpc/order/client.go`
4. 在 `internal/svc/service_context.go` 注入 `OrderClient`
5. 在 `internal/handler/routes.go` 调用 `order.RegisterRoutes(...)`

## 6. 开发约束（建议）

- `handler` 不直接访问数据库
- `handler` 不直接调用多个下游服务做复杂编排
- `logic` 不关心 HTTP 细节（状态码、Header）
- `rpc` 层不混入业务规则
- 全部接口统一使用标准化响应（`code/message/data/trace_id`）

## 7. 当前状态与后续计划

当前已完成：

- user/login 路由、handler、logic、rpc(mock) 全链路
- 标准化响应与 trace_id 输出

后续建议：

1. 将 `rpc/user/client.go` 替换为真实 kitex client
2. 增加鉴权、recover、日志中间件
3. 增加配置文件加载（dev/prod）并保留 env 覆盖
