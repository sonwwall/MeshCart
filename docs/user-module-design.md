# User 模块设计

## 1. 模块目标

`user` 模块当前实现两个基础能力：

- `login`
- `register`

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
- RPC Server：`services/user-service/rpc/handler.go`
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
- RPC Server：`services/user-service/rpc/handler.go`
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
- 成功时写入新用户记录并返回统一成功响应

## 4. 用户 ID 设计

- ID 生成库：`github.com/bwmarrin/snowflake`
- 生成位置：`user-service` 业务层
- 节点号来源：`services/user-service/config/user-service.local.yaml` 中的 `snowflake.node`

设计原因：

- 避免依赖数据库自增主键
- 便于后续多节点部署
- 便于在注册写库前直接拿到唯一用户 ID

## 5. 密码设计

- 密码算法：`bcrypt`
- 注册时：明文密码 -> `bcrypt` 哈希 -> 存库
- 登录时：使用 `bcrypt.CompareHashAndPassword` 校验

设计原因：

- 避免明文密码存储
- 降低数据库泄露时的直接风险

## 6. 可观测性设计

`login` 和 `register` 都接入了现有可观测性体系：

- 日志：统一使用 `log.L(ctx)`
- 链路：gateway logic 补 internal span，HTTP/RPC span 由框架插件自动生成
- 业务属性：
  - `biz.module=user`
  - `biz.action=login/register`
  - `biz.success`
  - `biz.code`
  - `biz.message`

约定：

- 技术错误标记为 trace error
- 业务失败不标红，只记录业务属性

## 7. 接口文档

## 7.1 用户注册

- 方法：`POST`
- 路径：`/api/v1/user/register`
- Content-Type：`application/json`

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

## 7.2 用户登录

- 方法：`POST`
- 路径：`/api/v1/user/login`
- Content-Type：`application/json`

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
    "user_id": 0,
    "token": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "username": "test_user"
  },
  "trace_id": "8f2d3f..."
}
```

说明：

- 登录成功后由 `gateway` 使用 Hertz JWT 中间件签发访问令牌
- 当前 `user-service` IDL 还没有返回真实 `user_id`，因此示例响应中可能仍为 `0`
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

## 7.3 当前登录态接口

### 7.3 获取当前用户信息

- 方法：`GET`
- 路径：`/api/v1/user/me`
- 鉴权：需要在 Header 中携带 `Authorization: Bearer <token>`

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "user_id": 0,
    "username": "test_user"
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
- 当前示例版本中，如果下游还未返回真实 `user_id`，这里仍可能为 `0`

### 7.4 刷新访问令牌

- 方法：`GET`
- 路径：`/api/v1/user/refresh_token`
- 鉴权：需要在 Header 中携带 `Authorization: Bearer <token>`

成功响应：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "token": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expire_at": "2026-03-09T18:00:00Z"
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

- 该接口用于基于当前有效 token 刷新访问令牌
- 返回的 `token` 已带 `Bearer ` 前缀，可直接写回 `Authorization` 头
