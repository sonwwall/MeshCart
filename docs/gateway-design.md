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
- `gateway/internal/{handler,logic,middleware,svc,types}`
- `gateway/rpc`
- `app/common`（网关返回结构与错误对象）

## 3. 目录结构

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

## 4. 分层职责

### 4.1 启动层（`cmd`）

职责：

- 读取配置
- 初始化 `ServiceContext`
- 初始化 Hertz Server
- 注册路由并启动服务

### 4.2 配置层（`config`）

职责：

- 定义配置结构体
- 读取环境变量并生成运行时配置

### 4.3 接口层（`internal/handler`）

职责：

- HTTP 参数绑定与基础校验
- 调用 `logic`
- 输出标准化响应

禁止：

- 直接访问下游 RPC/数据库
- 编排业务流程

### 4.4 业务编排层（`internal/logic`）

职责：

- 业务规则处理
- 下游服务调用编排
- 业务错误码映射

### 4.5 客户端层（`rpc/<module>`）

职责：

- 维护下游 Kitex Client
- 执行 RPC 调用
- 协议对象转换（Thrift <-> 网关内部结构）

禁止：

- 承载业务规则判断

### 4.6 依赖容器层（`internal/svc`）

职责：

- 集中初始化与持有共享依赖（配置、RPC Client）
- 向 `handler/logic` 提供依赖注入入口

### 4.7 协议类型层（`internal/types`）

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
7. `handler` 通过 `common.Success/Fail` 输出标准 JSON

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

- `GATEWAY_ADDR`：网关监听地址
- `USER_RPC_SERVICE`：用户服务名
- `USER_RPC_ADDR`：用户服务地址

## 9. 模块扩展规范

### 9.1 新增接口（同模块）

1. 在 `internal/types` 增加请求/响应结构
2. 在 `internal/logic/<module>` 增加业务文件
3. 在 `internal/handler/<module>` 增加 handler 文件
4. 在 `internal/handler/<module>/routes.go` 注册路由
5. 在 `rpc/<module>` 增加对应客户端方法

### 9.2 新增业务模块

1. 新建 `internal/handler/<module>`
2. 新建 `internal/logic/<module>`
3. 新建 `internal/types/<module>.go`
4. 新建 `rpc/<module>/client.go`
5. 在 `internal/svc/service_context.go` 注入模块客户端
6. 在 `internal/handler/routes.go` 聚合模块路由

## 10. 代码约束

- `handler` 不直接访问 `rpc` 生成客户端实现细节
- `logic` 不依赖 HTTP 细节（Header、状态码）
- `rpc` 不处理业务语义分支
- 所有对外响应必须走标准化返回结构
- 错误码遵循 `docs/error-code.md`
