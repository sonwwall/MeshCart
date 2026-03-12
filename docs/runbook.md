# Runbook

## 1. 目的

本文档记录 MeshCart 当前本地开发环境的启动、验证和常见排障步骤。

当前 runbook 面向以下链路：

- `gateway`
- `user-service`
- `product-service`
- Consul 服务发现
- MySQL migration
- Prometheus / Grafana / Loki / Jaeger / OTel Collector

当前运行方式仍以“Docker 启动基础依赖 + 宿主机手工启动业务服务”为主。

## 2. 当前拓扑

当前本地最小链路如下：

- `gateway`
  - 对外提供 HTTP
  - 通过 Consul 发现 `meshcart.user`、`meshcart.product`
- `user-service`
  - 提供用户注册、登录、角色治理 RPC
  - 依赖 MySQL、Consul、OTel Collector
- `product-service`
  - 提供商品查询、商品管理 RPC
  - 依赖 MySQL、Consul、OTel Collector
- Docker 依赖
  - Consul
  - Prometheus
  - Grafana
  - Loki
  - Promtail
  - Jaeger
  - OTel Collector

## 3. 前置条件

启动前需要具备：

- Go `1.24.x`
- Docker / Docker Compose
- 可用的 MySQL
- 本地可访问：
  - `services/user-service/config/user-service.local.yaml`
  - `services/product-service/config/product-service.local.yaml`

启动前建议先确认：

- MySQL 地址可连通
- `docker compose` 可正常启动依赖容器
- 项目根目录下 `logs/` 可写

## 4. 启动顺序

建议按以下顺序启动：

1. 启动 Docker 依赖
2. 启动 `user-service`
3. 启动 `product-service`
4. 启动 `gateway`
5. 做最小功能验证
6. 再做日志、指标、链路追踪验证

## 5. 启动 Docker 依赖

启动本地基础组件：

```bash
docker compose up -d consul prometheus grafana loki promtail jaeger otel-collector
```

检查容器状态：

```bash
docker compose ps
```

关键入口：

- Consul UI：`http://localhost:8500`
- Grafana：`http://localhost:3000`
- Prometheus：`http://localhost:9090`
- Jaeger：`http://localhost:16686`
- Loki：`http://localhost:3100`
- OTel Collector gRPC：`localhost:4319`
- OTel Collector HTTP：`localhost:4320`

如果只想看某个组件日志：

```bash
docker compose logs --tail=120 consul
docker compose logs --tail=120 otel-collector
docker compose logs --tail=120 promtail
```

## 6. 启动业务服务

### 6.1 启动 `user-service`

```bash
./services/user-service/script/start.sh
```

默认行为：

- 读取 `services/user-service/config/user-service.local.yaml`
- RPC 监听地址默认 `127.0.0.1:8888`
- 默认注册到 Consul：`127.0.0.1:8500`
- 默认 metrics 地址：`:9091`
- 默认使用 Consul TTL 健康检查
- 启动时自动执行 migration

常用环境变量：

```bash
export USER_SERVICE_CONFIG=services/user-service/config/user-service.local.yaml
export USER_RPC_SERVICE=meshcart.user
export USER_SERVICE_ADDR=127.0.0.1:8888
export USER_SERVICE_REGISTRY=consul
export USER_SERVICE_CONSUL_TCP_CHECK=false
export USER_METRICS_ADDR=:9091
export CONSUL_ADDR=127.0.0.1:8500
```

启动成功后重点看：

- 控制台出现 `user-service starting`
- Consul 中出现 `meshcart.user`
- `http://127.0.0.1:9091/metrics` 可访问

### 6.2 启动 `product-service`

```bash
./services/product-service/script/start.sh
```

默认行为：

- 读取 `services/product-service/config/product-service.local.yaml`
- RPC 监听地址默认 `127.0.0.1:8889`
- 默认注册到 Consul：`127.0.0.1:8500`
- 默认 metrics 地址：`:9093`
- 默认使用 Consul TTL 健康检查
- 启动时自动执行 migration

常用环境变量：

```bash
export PRODUCT_SERVICE_CONFIG=services/product-service/config/product-service.local.yaml
export PRODUCT_RPC_SERVICE=meshcart.product
export PRODUCT_SERVICE_ADDR=127.0.0.1:8889
export PRODUCT_SERVICE_REGISTRY=consul
export PRODUCT_SERVICE_CONSUL_TCP_CHECK=false
export PRODUCT_METRICS_ADDR=:9093
export CONSUL_ADDR=127.0.0.1:8500
```

启动成功后重点看：

- 控制台出现 `product-service starting`
- Consul 中出现 `meshcart.product`
- `http://127.0.0.1:9093/metrics` 可访问

### 6.3 启动 `gateway`

```bash
./gateway/script/start.sh
```

默认行为：

- HTTP 监听默认 `:8080`
- 默认通过 Consul 发现 `meshcart.user`、`meshcart.product`
- 默认 metrics 地址 `:9092/metrics`
- 默认启用 JWT
- 默认启用第一阶段网关限流

常用环境变量：

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
export PRODUCT_RPC_SERVICE=meshcart.product
export PRODUCT_RPC_DISCOVERY=consul
export PRODUCT_RPC_ADDR=127.0.0.1:8889
export CONSUL_ADDR=127.0.0.1:8500
export JWT_SECRET=meshcart-dev-secret-change-me
export JWT_ISSUER=meshcart.gateway
export JWT_TIMEOUT_MINUTES=120
export JWT_MAX_REFRESH_MINUTES=720
```

限流相关环境变量：

```bash
export GATEWAY_RATE_LIMIT_ENABLED=true
export GATEWAY_GLOBAL_IP_RATE_LIMIT_RPS=50
export GATEWAY_GLOBAL_IP_RATE_LIMIT_BURST=100
export GATEWAY_LOGIN_IP_RATE_LIMIT_RPS=5
export GATEWAY_LOGIN_IP_RATE_LIMIT_BURST=10
export GATEWAY_REGISTER_IP_RATE_LIMIT_RPS=2
export GATEWAY_REGISTER_IP_RATE_LIMIT_BURST=5
```

启动成功后重点看：

- 控制台出现 `gateway starting`
- `http://127.0.0.1:9092/metrics` 可访问
- 访问 `gateway` 接口时能返回 JSON 响应

## 7. 最小功能验证

### 7.1 注册用户

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
- `role`
- `Bearer <token>` 格式的 `token`

### 7.3 获取当前用户

```bash
curl http://127.0.0.1:8080/api/v1/user/me \
  -H 'Authorization: Bearer <token>'
```

### 7.4 刷新 token

```bash
curl http://127.0.0.1:8080/api/v1/user/refresh_token \
  -H 'Authorization: Bearer <token>'
```

### 7.5 商品列表

```bash
curl 'http://127.0.0.1:8080/api/v1/products?page=1&page_size=10'
```

### 7.6 商品详情

```bash
curl http://127.0.0.1:8080/api/v1/products/detail/<product_id>
```

说明：

- 如果列表和详情都能走通，说明 `gateway -> product-service` 基础链路正常
- 如果注册、登录、`me` 能走通，说明 `gateway -> user-service` 基础链路正常

## 8. 运行状态检查

### 8.1 Consul

打开 `http://localhost:8500`，检查：

- `meshcart.user` 已注册且状态为 `passing`
- `meshcart.product` 已注册且状态为 `passing`

如果服务是红色，优先检查：

- 服务进程是否仍在运行
- `*_SERVICE_ADDR` 是否正确
- 是否把宿主机 `127.0.0.1` 注册给了容器内 Consul 同时又开启了 TCP 检查

### 8.2 Metrics

检查指标端点：

- Gateway：`http://127.0.0.1:9092/metrics`
- User-service：`http://127.0.0.1:9091/metrics`
- Product-service：`http://127.0.0.1:9093/metrics`

再打开：

- Prometheus Targets：`http://localhost:9090/targets`

看 Prometheus 是否真的抓到了这些端点。

### 8.3 Trace

打开 `http://localhost:16686`，检索：

- `gateway`
- `user-service`
- `product-service`

建议优先查询：

- 登录链路
- 商品列表链路

### 8.4 日志

应用日志默认会落到项目根目录的 `logs/`。

重点文件：

- `logs/gateway.log`
- `logs/user-service.log`
- `logs/product-service.log`

如果 Promtail 正常工作，也可以通过 Grafana Explore 查询 Loki 日志。

## 9. 常见排障路径

### 9.1 `gateway` 接口报“下游服务暂不可用，请稍后重试”

优先检查：

1. Consul 中 `meshcart.user` 或 `meshcart.product` 是否存在健康实例
2. `gateway` 当前是不是走 `consul` 发现模式
3. 对应服务进程是否仍在运行
4. 该服务监听地址是否和注册到 Consul 的地址一致

必要时可临时切回直连排障：

```bash
export USER_RPC_DISCOVERY=direct
export USER_RPC_ADDR=127.0.0.1:8888
export PRODUCT_RPC_DISCOVERY=direct
export PRODUCT_RPC_ADDR=127.0.0.1:8889
./gateway/script/start.sh
```

如果直连正常、Consul 异常，优先看服务发现问题；如果直连也异常，优先看下游服务本身。

### 9.2 Consul 中服务显示 `critical`

常见原因：

- 业务服务已退出
- 注册地址错误
- Consul 在 Docker 容器里，而服务注册了宿主机 `127.0.0.1`，同时启用了 TCP 检查

当前推荐：

- 宿主机启动业务服务时保持：
  - `USER_SERVICE_CONSUL_TCP_CHECK=false`
  - `PRODUCT_SERVICE_CONSUL_TCP_CHECK=false`

### 9.3 登录或商品接口报“服务繁忙，请稍后重试”

优先判断是哪一层超时：

- `gateway` request timeout
- `gateway -> rpc` timeout
- service 内部 DB query timeout

排查顺序：

1. 看 `gateway` 日志是否有下游 RPC timeout
2. 看对应服务日志是否有数据库超时或底层错误
3. 打开 Jaeger 看超时发生在 `gateway`、RPC 还是 DB 前后

### 9.4 登录接口频繁报“请求过于频繁，请稍后再试”

优先检查：

- 当前 IP 是否连续请求过快
- 是否开启了第一阶段限流：`GATEWAY_RATE_LIMIT_ENABLED=true`
- 登录/注册限流阈值是否被手动调低

当前已启用的限流包括：

- `/api/v1/*` 全局 IP 宽松限流
- `login/register` 按 IP + 路由更严格限流
- 商品管理写接口按用户和路由限流

如果是本地调试需要临时放宽：

- 调整 `GATEWAY_*_RATE_LIMIT_RPS`
- 调整 `GATEWAY_*_RATE_LIMIT_BURST`
- 或临时关闭 `GATEWAY_RATE_LIMIT_ENABLED`

### 9.5 Migration 失败

优先检查：

- 配置文件中的 MySQL 地址、用户名、密码、数据库名是否正确
- 数据库是否可连接
- migration 目录是否存在
- 上一次 migration 是否留下 dirty 状态

当前配置文件位置：

- `services/user-service/config/user-service.local.yaml`
- `services/product-service/config/product-service.local.yaml`

当前 migration 目录：

- `services/user-service/migrations`
- `services/product-service/migrations`

如果 migration 报错后数据库结构已经部分变更，不要只清空 `schema_migrations` 状态；应同时核对真实表结构。

### 9.6 指标看不到

排查顺序：

1. 直接访问服务自身 metrics 端点
2. 再看 Prometheus Targets 是否 `UP`
3. 最后再看 Grafana 面板

如果服务端点有数据但 Prometheus 没抓到，优先看 Prometheus 配置或网络连通性。

### 9.7 Jaeger 中查不到链路

排查顺序：

1. `otel-collector` 是否正常运行
2. `OTEL_EXPORTER_OTLP_ENDPOINT` 是否指向 `localhost:4319`
3. 对应服务是否真的收到请求
4. `gateway` 与下游服务是否使用了同一个请求链路

如果只有 `gateway` 有 span、下游没有，优先怀疑 RPC 调用、下游服务启动状态或 trace 上报链路。

### 9.8 Loki / Grafana 查不到业务日志

排查顺序：

1. 本地 `logs/` 目录是否已经写出业务日志
2. `promtail` 是否正常运行
3. Grafana Explore 中是否选对了 Loki 数据源
4. 查询语句是否先限定了服务名

建议先按服务过滤，再加关键字：

- `gateway`
- `user-service`
- `product-service`
- `trace_id`
- 错误文案关键字

## 10. 建议的排障顺序

单条请求失败时，建议按这个顺序看：

1. 先看接口返回文案
2. 再看 `gateway` 日志
3. 再看 Consul 中下游服务是否健康
4. 再看下游服务日志
5. 再看 Jaeger 链路
6. 最后再看 Prometheus / Grafana 指标趋势

这样能避免一开始就跳进监控面板里盲查。

## 11. 常用回归测试

当前建议优先运行：

```bash
go test ./gateway/...
go test ./services/user-service/...
go test ./services/product-service/...
```

如果只验证治理相关改动，优先跑：

```bash
go test ./gateway/internal/middleware ./gateway/internal/component ./gateway/rpc/user ./gateway/rpc/product
```

## 12. 相关文档

- [微服务治理规划](./microservice-governance.md)
- [服务开发设计规范](./service-development-spec.md)
- [Consul 服务发现设计](./consul-service-discovery.md)
- [日志与链路追踪](./logging-tracing.md)
- [商品服务设计方案](./product-service-design.md)
- [用户模块设计](./user-module-design.md)
