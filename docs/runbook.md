# Runbook

## 1. 目的

本文档记录 MeshCart 当前本地开发环境的启动、验证和常见排障步骤。

当前 runbook 以“手工启动业务服务 + Docker 启动基础依赖”为主，不要求业务服务先容器化。

## 2. 前置条件

启动前需要具备：

- Go 1.24.x
- 本地可用的 MySQL
- Docker / Docker Compose
- 可访问的 `services/user-service/config/user-service.local.yaml`

当前本地依赖关系：

- `gateway`
  - 依赖 `user-service`
  - 依赖 Consul
  - 依赖 OTel Collector
- `user-service`
  - 依赖 MySQL
  - 依赖 Consul
  - 依赖 OTel Collector

## 3. 启动顺序

建议按以下顺序启动：

1. 启动 Docker 依赖
2. 启动 `user-service`
3. 启动 `gateway`
4. 调接口验证

## 4. 启动 Docker 依赖

启动本地基础组件：

```bash
docker compose up -d consul prometheus grafana loki promtail jaeger otel-collector
```

关键端口：

- Consul UI：`http://localhost:8500`
- Grafana：`http://localhost:3000`
- Prometheus：`http://localhost:9090`
- Jaeger：`http://localhost:16686`
- Loki：`http://localhost:3100`
- OTel Collector gRPC：`localhost:4319`

## 5. 启动 user-service

使用启动脚本：

```bash
./services/user-service/script/start.sh
```

默认行为：

- 从 `services/user-service/config/user-service.local.yaml` 读取 MySQL 配置
- RPC 监听地址默认是 `127.0.0.1:8888`
- 默认注册到 Consul：`127.0.0.1:8500`
- 默认使用 Consul TTL 健康检查

常用可覆盖环境变量：

```bash
export USER_SERVICE_ADDR=127.0.0.1:8888
export USER_RPC_SERVICE=meshcart.user
export USER_SERVICE_REGISTRY=consul
export CONSUL_ADDR=127.0.0.1:8500
export USER_METRICS_ADDR=:9091
```

启动成功后应看到：

- `user-service starting`
- `server listen at addr=127.0.0.1:8888`

## 6. 启动 gateway

使用启动脚本：

```bash
./gateway/script/start.sh
```

默认行为：

- 默认监听 `:8080`
- 默认通过 Consul 发现 `meshcart.user`
- 默认启用 JWT
- 默认服务名为 `gateway`
- 默认 OTLP 地址为 `localhost:4319`
- 默认 metrics 暴露为 `:9092/metrics`

常用可覆盖环境变量：

```bash
export APP_NAME=gateway
export APP_ENV=dev
export LOG_LEVEL=info
export LOG_DIR=logs
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4319
export OTEL_EXPORTER_OTLP_INSECURE=true
export GATEWAY_ADDR=:8080
export GATEWAY_PROM_ADDR=:9092
export GATEWAY_PROM_PATH=/metrics
export USER_RPC_SERVICE=meshcart.user
export USER_RPC_DISCOVERY=consul
export USER_RPC_ADDR=127.0.0.1:8888
export CONSUL_ADDR=127.0.0.1:8500
export JWT_SECRET=meshcart-dev-secret-change-me
export JWT_ISSUER=meshcart.gateway
export JWT_TIMEOUT_MINUTES=120
export JWT_MAX_REFRESH_MINUTES=720
```

说明：

- `gateway` 的启动期配置已经集中在 `gateway/config/config.go`
- `cmd/gateway/main.go` 只负责编排启动流程，不再直接读取环境变量
- 日志、OTel、HTTP Server 初始化由 `gateway/internal/component` 负责

启动成功后应看到：

- `gateway starting`

## 7. 功能验证

### 7.1 注册

```bash
curl -X POST http://127.0.0.1:8080/api/v1/user/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"test_user","password":"123456"}'
```

### 7.2 登录

```bash
curl -X POST http://127.0.0.1:8080/api/v1/user/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"test_user","password":"123456"}'
```

成功时应返回：

- 真实 `user_id`
- `Bearer <token>` 格式的 `token`

### 7.3 获取当前用户

把登录返回的 `token` 原样放进 `Authorization`：

```bash
curl http://127.0.0.1:8080/api/v1/user/me \
  -H 'Authorization: Bearer <token>'
```

### 7.4 刷新 token

```bash
curl http://127.0.0.1:8080/api/v1/user/refresh_token \
  -H 'Authorization: Bearer <token>'
```

## 8. 运行状态检查

### 8.1 Consul

打开 `http://localhost:8500`，检查：

- 服务 `meshcart.user` 已注册
- 实例状态为 `passing`

如果服务是红色，优先检查：

- `user-service` 是否还在运行
- `USER_SERVICE_ADDR` 是否正确
- `USER_SERVICE_CONSUL_TCP_CHECK` 是否误设为 `true`

### 8.2 Metrics

检查指标端点：

- Gateway：`http://127.0.0.1:9092/metrics`
- User-service：`http://127.0.0.1:9091/metrics`

### 8.3 Trace

打开 `http://localhost:16686`，检索：

- `gateway`
- `user-service`

建议从登录链路开始排查。

## 9. 常见问题排查

### 9.1 登录报系统内部错误

优先检查：

- `gateway` 是否通过 Consul 找到了健康的 `meshcart.user`
- `user-service` 是否成功注册
- MySQL 是否可连接

### 9.2 Consul 中服务显示为 critical

常见原因：

- `user-service` 已退出
- 服务注册地址错误
- Consul 在容器里运行，而服务注册了宿主机 `127.0.0.1`，同时启用了 TCP 检查

当前推荐：

- 宿主机启动 `user-service` 时保持 `USER_SERVICE_CONSUL_TCP_CHECK=false`

### 9.3 JWT 鉴权失败

检查：

- 是否带了 `Authorization` Header
- Header 格式是否为 `Bearer <token>`
- `JWT_SECRET` 是否在网关重启前后发生变化
- token 是否已经过期

### 9.4 Gateway 启动后看不到日志 / metrics / trace

检查：

- `LOG_DIR` 是否可写，且项目根目录下是否生成 `logs/gateway.log`
- `GATEWAY_PROM_ADDR`、`GATEWAY_PROM_PATH` 是否符合预期
- `OTEL_EXPORTER_OTLP_ENDPOINT` 是否指向本地 Collector
- `OTEL_EXPORTER_OTLP_INSECURE` 是否与 Collector 监听方式匹配
- 启动日志中是否出现 `gateway starting`

### 9.5 注册成功但登录拿不到真实 user_id

检查：

- `user-service` 是否已经使用最新 IDL 和代码启动
- 是否重新启动了 `gateway`
- 数据库中的用户记录是否存在有效 `id`

## 10. 测试

当前建议优先运行以下测试：

```bash
go test ./gateway/...
go test ./services/user-service/...
```

如果只想验证本次核心链路：

```bash
go test ./gateway/internal/logic/user ./services/user-service/biz/service
```

## 11. 变更后验证清单

在修改以下能力后，建议至少完成一次回归：

- 用户登录 / 注册
- JWT 配置
- Consul 服务发现
- user-service 登录返回结构

最小回归步骤：

1. 启动依赖
2. 启动 `user-service`
3. 启动 `gateway`
4. 注册用户
5. 登录并检查 `user_id`
6. 调用 `/api/v1/user/me`
