# User 模块设计

## 1. 模块目标

`user` 模块当前实现以下能力：

- `login`
- `register`
- `me`
- `refresh_token`
- `logout`
- `update user role`

模块拆分遵循现有项目分层：

- `gateway`
  - `handler`：HTTP 请求绑定、统一响应
  - `logic`：参数校验、调用 RPC、补业务 span
- `user-service`
  - `rpc/handler`：RPC 入站转发
  - `biz/service`：核心业务逻辑
  - `biz/repository`：数据访问
  - `dal/model`：表结构映射

## 2. Login 设计

### 2.1 入口链路

- HTTP：`POST /api/v1/user/login`
- Gateway Handler：`gateway/internal/handler/user/login.go`
- Gateway Logic：`gateway/internal/logic/user/login.go`
- RPC Client：`gateway/rpc/user/client.go`
- RPC Server：`services/user-service/rpc/handler/`
- Biz Service：`services/user-service/biz/service/service.go`

### 2.2 设计思想

- `handler` 不写业务逻辑，只负责请求绑定、日志、统一响应
- `logic` 负责网关侧业务编排，并补充 internal span
- `user-service` 负责真正的用户认证逻辑
- 密码不做明文比对，统一使用哈希密码校验

### 2.3 登录校验规则

- 用户不存在：返回 `用户不存在`
- 用户被锁定：返回 `用户已被锁定`
- 密码不匹配：返回 `用户名或密码错误`
- 成功：返回统一成功响应

## 3. Register 设计

### 3.1 入口链路

- HTTP：`POST /api/v1/user/register`
- Gateway Handler：`gateway/internal/handler/user/register.go`
- Gateway Logic：`gateway/internal/logic/user/register.go`
- RPC Client：`gateway/rpc/user/client.go`
- RPC Server：`services/user-service/rpc/handler/`
- Biz Service：`services/user-service/biz/service/service.go`

### 3.2 设计思想

- 复用与 `login` 一致的 gateway 分层，保证模块风格统一
- 注册只接受 `username` 和 `password`
- 用户 ID 不依赖数据库自增，统一使用雪花算法生成
- 密码只保存哈希值，不落明文

### 3.3 注册规则

- `username` 不能为空
- `password` 不能为空
- `password` 长度至少 6 位
- 用户名重复时返回 `用户名已存在`
- 全新空库场景下，首个注册用户自动成为 `superadmin`
- 成功时写入新用户记录并返回统一成功响应

## 4. 角色设计

当前用户角色保存在 `users.role` 字段，支持：

- `user`
- `admin`
- `superadmin`

当前落地规则：

- 全新空库场景下，首个注册用户自动成为 `superadmin`
- 已有库迁移时，migration 会把最早创建的用户提升为初始 `superadmin`
- 登录成功后，`user-service` 返回当前角色，`gateway` 将其写入 JWT claims
- 角色变更后，旧 token 不会自动失效；需要重新登录或刷新 token 才能拿到新角色

## 5. 用户 ID 设计

- ID 生成库：`github.com/bwmarrin/snowflake`
- 生成位置：`user-service` 业务层
- 节点号来源：`services/user-service/config/user-service.local.yaml` 中的 `snowflake.node`

设计原因：

- 避免依赖数据库自增主键
- 便于后续多节点部署
- 便于在注册写库前直接拿到唯一用户 ID

## 6. 密码设计

- 密码算法：`bcrypt`
- 注册时：明文密码 -> `bcrypt` 哈希 -> 存库
- 登录时：使用 `bcrypt.CompareHashAndPassword` 校验

设计原因：

- 避免明文密码存储
- 降低数据库泄露时的直接风险

## 7. 可观测性设计

`login`、`register`、`getUser`、`updateUserRole` 都接入了现有可观测性体系：

- 日志：统一使用 `log.L(ctx)`
- 链路：gateway logic 补 internal span，HTTP/RPC span 由框架插件自动生成
- 业务属性：
  - `biz.module=user`
  - `biz.action=login/register/update_role`
  - `biz.success`
  - `biz.code`
  - `biz.message`

约定：

- 技术错误标记为 trace error
- 业务失败不标红，只记录业务属性
- `user-service` 业务层会记录底层技术错误，例如密码哈希失败、查库失败、写库失败，RPC 对外仍返回统一业务错误码

当前日志已经进一步细化到“可排障”级别：

- `gateway/internal/logic/user/`
  - `login`
  - `register`
  - `update_user_role`
  - 会区分 transport error 和下游业务错误
  - 业务拒绝时会记录 RPC `code/message`
- `services/user-service/biz/service/`
  - `login`
  - `register`
  - `get_user`
  - `update_user_role`
  - 会记录 `start / reject / completed`
  - 会明确记录“用户不存在”“密码错误”“最后一个 superadmin 不允许降级”等业务拒绝原因
- `services/user-service/biz/repository/`
  - 会记录查库失败、建用户失败、角色更新失败、唯一键冲突等原始 DB 错误

当前排障建议：

1. 先看 `gateway` 日志
   - 判断是 RPC 调用失败，还是 `user-service` 业务拒绝
2. 再看 `user-service` service/repository 日志
   - 判断是业务校验拒绝，还是底层 DB/密码哈希错误
3. 用 `trace_id` + `username/user_id` 交叉定位

## 8. 超时治理

当前 `user-service` 已补齐数据库查询超时配置：

- 配置位置：`services/user-service/config/config.go`
- 本地配置示例：`services/user-service/config/user-service.local.yaml`
- 当前字段：`timeout.db_query_ms`

当前落点：

- `gateway` 调 `user-service` 的 Kitex Client 已显式设置 connect timeout 和 rpc timeout
- `gateway` 调 `user-service` 的 `GetUser` 已启用一次有限重试；`Login`、`Register`、`UpdateUserRole` 不自动重试
- `user-service` repository 在执行 GORM 查询和更新时，会基于传入 `ctx` 统一套上 DB query timeout

这样做的目的：

- 避免下游数据库操作无限等待
- 让 RPC 超时预算和服务内查询超时预算可同时收口到配置层
- 在不放大写链路重复执行风险的前提下，提高用户读接口对瞬时网络抖动的容忍度
- 为后续熔断、排障提供稳定前提

## 9. 数据迁移说明

当前用户域 migration 目录：

- `services/user-service/migrations`

当前角色迁移依赖多语句 SQL：

- 给 `users` 表增加 `role` 列
- 回填初始 `superadmin`

实现说明：

- `user-service` 启动时自动执行 migration
- `services/user-service/rpc/main.go` 现在只保留 `bootstrap.Run()` 入口
- 启动装配下沉到 `services/user-service/rpc/bootstrap/bootstrap.go`
- RPC 入站方法按接口拆到 `services/user-service/rpc/handler/`
- migration 连接已单独开启 MySQL `multiStatements`
- 如果某次 migration 中途失败，不要只把 `dirty` 清零；需要同时核对 `schema_migrations.version` 与真实表结构是否一致

## 10. 接口文档

## 10.1 用户注册

- 方法：`POST`
- 路径：`/api/v1/user/register`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`

请求体：

```json
{
  "username": "test_user",
  "password": "123456"
}
```

请求字段：

- `username`：用户名，必填
- `password`：密码，必填，至少 6 位

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "trace_id": "8f2d3f..."
}
```

失败响应示例：

```json
{
  "code": 2010004,
  "message": "用户名已存在",
  "trace_id": "8f2d3f..."
}
```

可能返回的错误码：

- `1000001`：请求参数错误
- `2010004`：用户名已存在
- `2010005`：密码格式不合法
- `1009999`：系统内部错误

说明：

- 当前通过 Hertz `BindAndValidate` 同时支持 JSON 与表单提交

## 10.2 用户登录

- 方法：`POST`
- 路径：`/api/v1/user/login`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`

请求体：

```json
{
  "username": "test_user",
  "password": "123456"
}
```

请求字段：

- `username`：用户名，必填
- `password`：密码，必填

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "user_id": 123456789012345678,
    "token": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "username": "test_user",
    "role": "user"
  },
  "trace_id": "8f2d3f..."
}
```

说明：

- 登录成功后由 `gateway` 使用 Hertz JWT 中间件签发访问令牌
- `user_id` 由 `user-service` 登录成功后返回真实用户 ID
- `role` 来自 `user-service` 返回结果，并写入 JWT claims
- 后续受保护接口通过 `Authorization: Bearer <token>` 携带登录态

失败响应示例：

```json
{
  "code": 2010002,
  "message": "用户名或密码错误",
  "trace_id": "8f2d3f..."
}
```

可能返回的错误码：

- `1000001`：请求参数错误
- `2010001`：用户不存在
- `2010002`：用户名或密码错误
- `2010003`：用户已被锁定
- `1009999`：系统内部错误

## 10.3 当前登录态接口

### 9.3.1 获取当前用户信息

- 方法：`GET`
- 路径：`/api/v1/user/me`
- 鉴权：需要在 Header 中携带 `Authorization: Bearer <token>`

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "user_id": 123456789012345678,
    "username": "test_user",
    "role": "user"
  },
  "trace_id": "8f2d3f..."
}
```

失败响应示例：

```json
{
  "code": 1000002,
  "message": "未登录或登录已过期",
  "trace_id": "8f2d3f..."
}
```

说明：

- 该接口由网关 JWT 中间件保护
- 返回值来自当前 token 中的 claims 解析结果
- 当前返回字段包含 `session_id`、`user_id`、`username`、`role`

### 9.3.2 刷新访问令牌

- 方法：`POST`
- 路径：`/api/v1/user/refresh_token`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`

请求体：

```json
{
  "refresh_token": "0nD6xjF8rD3xX0cYB1x8A0fN6p8q3mL2G2oS8u9Qv7w"
}
```

请求字段：

- `refresh_token`：登录或上次刷新返回的 refresh token，必填

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "session_id": "18d6b63d-87f2-4f1d-89b0-0fd68f4f662e",
    "token_type": "Bearer",
    "access_token": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "access_expire_at": "2026-03-20T18:00:00Z",
    "refresh_token": "eJ6vL3jzQmX8sY2cT7rB6dN1pK4uH9fA0qW5mC8xR2t",
    "refresh_expire_at": "2026-04-19T18:00:00Z"
  },
  "trace_id": "8f2d3f..."
}
```

失败响应示例：

```json
{
  "code": 1000002,
  "message": "未登录或登录已过期",
  "trace_id": "8f2d3f..."
}
```

说明：

- 该接口用于基于当前有效 `refresh_token` 刷新双 token
- 刷新成功后会执行 rotation，旧 `refresh_token` 立即失效
- 刷新时会重新读取用户当前角色，并写入新的 access token claims
- 返回的 `access_token` 已带 `Bearer ` 前缀，可直接写回 `Authorization` 头

### 9.3.3 退出当前登录会话

- 方法：`POST`
- 路径：`/api/v1/user/logout`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`
- 鉴权：需要在 Header 中携带 `Authorization: Bearer <access_token>`

请求体：

```json
{
  "session_id": "18d6b63d-87f2-4f1d-89b0-0fd68f4f662e"
}
```

请求字段：

- `session_id`：可选；如传入则必须与当前 access token 中的 `session_id` 一致

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "trace_id": "8f2d3f..."
}
```

失败响应示例：

```json
{
  "code": 1000002,
  "message": "未登录或登录已过期",
  "trace_id": "8f2d3f..."
}
```

说明：

- 该接口用于单端登出，只吊销当前 session 对应的 refresh token
- 已签发的 access token 不会被立即拉黑，会在过期后自然失效
- 登出成功后，再使用该 session 的 `refresh_token` 刷新会返回未登录

## 10.4 Superadmin 角色管理接口

### 9.4.1 修改用户角色

- 方法：`PUT`
- 路径：`/api/v1/admin/users/:user_id/role`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`
- 鉴权：需要在 Header 中携带 `Authorization: Bearer <token>`
- 权限：仅 `superadmin` 可访问

路径参数：

- `user_id`：目标用户 ID，必填，必须大于 0

请求体：

```json
{
  "role": "admin"
}
```

请求字段：

- `role`：目标角色，必填

当前支持角色：

- `user`
- `admin`
- `superadmin`

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "trace_id": "8f2d3f..."
}
```

失败响应示例：

```json
{
  "code": 1000003,
  "message": "无权限访问",
  "trace_id": "8f2d3f..."
}
```

可能返回的错误码：

- `1000001`：请求参数错误
- `1000002`：未登录或登录已过期
- `1000003`：无权限访问
- `2010001`：用户不存在
- `2010006`：用户角色不合法
- `2010007`：至少保留一个 superadmin
- `1009999`：系统内部错误

说明：

- 网关会先根据 JWT claims 中的 `role` 做 Casbin 鉴权，只有 `superadmin` 可以发起角色修改
- 角色修改由 `user-service` 执行，并校验目标用户是否存在、目标角色是否合法
- 不允许把系统中的最后一个 `superadmin` 降级，避免平台失去最高治理权限
- 角色变更不会自动改写已签发的旧 token；目标用户需要重新登录或刷新 token 后才会拿到新角色
