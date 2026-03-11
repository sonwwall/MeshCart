# API

## 1. 说明

本文档汇总当前已落地的 gateway HTTP 接口中，与用户域和权限治理直接相关的部分。

统一响应格式：

```json
{
  "code": 0,
  "message": "成功",
  "data": {},
  "trace_id": "8f2d3f..."
}
```

统一约定：

- 网关当前业务错误仍返回 HTTP `200`
- 真实业务结果通过响应体中的 `code` 和 `message` 表达
- 受保护接口通过 `Authorization: Bearer <token>` 传递登录态

## 2. 用户接口

### 2.1 注册

- 方法：`POST`
- 路径：`/api/v1/user/register`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`

请求字段：

- `username`
- `password`

说明：

- 首个注册成功的用户会自动成为 `superadmin`

### 2.2 登录

- 方法：`POST`
- 路径：`/api/v1/user/login`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`

请求字段：

- `username`
- `password`

成功返回 `data` 字段：

- `user_id`
- `username`
- `role`
- `token`

说明：

- `role` 会写入 JWT claims

### 2.3 当前用户信息

- 方法：`GET`
- 路径：`/api/v1/user/me`
- 鉴权：需要登录

成功返回 `data` 字段：

- `user_id`
- `username`
- `role`

### 2.4 刷新访问令牌

- 方法：`GET`
- 路径：`/api/v1/user/refresh_token`
- 鉴权：需要登录

成功返回 `data` 字段：

- `token`
- `expire_at`

## 3. Superadmin 接口

### 3.1 修改用户角色

- 方法：`PUT`
- 路径：`/api/v1/admin/users/:user_id/role`
- Content-Type：`application/json`、`application/x-www-form-urlencoded`、`multipart/form-data`
- 鉴权：需要登录
- 权限：仅 `superadmin`

请求字段：

- `role`

支持角色：

- `user`
- `admin`
- `superadmin`

说明：

- 只有 `superadmin` 可以修改用户角色
- 不允许把最后一个 `superadmin` 降级
- 角色修改成功后，目标用户需要重新登录或刷新 token 才会拿到新角色

## 4. 相关文档

- 用户模块设计：`docs/user-module-design.md`
- Casbin 权限设计：`docs/casbin-design.md`
