# MeshCart 可观测性设计与使用手册

## 1. 目标与范围

本文档描述 MeshCart 当前可观测性体系的设计与使用方式，覆盖：

- 日志：Zap（结构化日志）
- 指标：Prometheus（HTTP/RPC 指标采集）
- 链路追踪：OpenTelemetry SDK + Jaeger
- 日志采集：Promtail -> Loki
- 可视化：Grafana（数据源 + 预置看板）

适用服务：

- `gateway`
- `user-service`

## 2. 总体架构

数据流如下：

1. 应用输出 Zap JSON 日志到 stdout
2. Promtail 采集容器日志并推送到 Loki
3. 应用通过 OTel SDK 上报 trace 到 OTel Collector
4. OTel Collector 转发 trace 到 Jaeger
5. 应用暴露 Prometheus 指标端点，Prometheus 抓取
6. Grafana 统一展示 Prometheus / Loki / Jaeger 数据

## 3. 组件清单与落点

### 3.1 基础设施（Docker Compose）

- `prometheus`
- `grafana`
- `loki`
- `promtail`
- `jaeger`
- `otel-collector`

主配置文件：

- `docker-compose.yml`
- `deploy/docker/observability/`

### 3.2 代码模块

- 日志：`app/log`
  - `logger.go`
  - `context.go`
- 链路追踪：`app/trace`
  - `otel.go`
  - `hertz.go`
- 指标：`app/metrics`
  - `http.go`
  - `rpc.go`
  - `prom.go`

### 3.3 服务接入点

- Gateway
  - `gateway/cmd/gateway/main.go`
  - `gateway/internal/handler/user/login.go`
  - `gateway/internal/logic/user/login.go`
  - `gateway/rpc/user/client.go`
- User Service
  - `services/user-service/rpc/main.go`
  - `services/user-service/rpc/handler.go`

## 4. 设计说明

## 4.1 日志设计（Zap）

### 4.1.1 设计原则

- 全量结构化 JSON 输出
- 字段命名统一，避免跨服务不一致
- 通过 `context` 自动注入链路字段

### 4.1.2 统一字段

基础字段：

- `service`：服务名，如 `gateway`
- `env`：环境，如 `dev`/`prod`
- `level`：日志级别
- `msg`：日志消息
- `ts`：时间戳
- `caller`：代码位置

链路字段：

- `trace_id`：一次完整请求链路唯一标识
- `span_id`：链路中单个调用片段标识
- `user_id`：用户标识（登录后可注入）

说明：

- `trace_id/span_id` 优先读业务 context
- 若 context 内有 OTel span，会自动补齐对应字段

## 4.2 Trace 设计（OpenTelemetry）

### 4.2.1 设计原则

- 服务启动时初始化全局 TracerProvider
- 使用 W3C TraceContext 做上下文传播
- 在关键链路创建 Server/Internal/Client span

### 4.2.2 Span 命名规范

格式：`<domain>.<layer>.<module>.<action>`

示例：

- `gateway.user.login`
- `gateway.logic.user.login`
- `gateway.rpc.user.login`
- `user.rpc.login`

### 4.2.3 链路传播

- Gateway 从 HTTP Header 提取 trace context
- Gateway 内部调用保持同一 context
- RPC 调用使用同一 context 继续传递

## 4.3 Metrics 设计（Prometheus）

### 4.3.1 HTTP 指标（gateway）

- `meshcart_http_requests_total`
  - 类型：Counter
  - 标签：`service`, `method`, `path`, `status`
- `meshcart_http_request_duration_seconds`
  - 类型：Histogram
  - 标签：`service`, `method`, `path`
- `meshcart_http_request_errors_total`
  - 类型：Counter
  - 标签：`service`, `method`, `path`, `status`
  - 条件：`status >= 400`

### 4.3.2 RPC 指标（user-service）

- `meshcart_rpc_requests_total`
  - 类型：Counter
  - 标签：`service`, `method`, `code`
- `meshcart_rpc_request_duration_seconds`
  - 类型：Histogram
  - 标签：`service`, `method`
- `meshcart_rpc_errors_total`
  - 类型：Counter
  - 标签：`service`, `method`, `code`
  - 条件：`code != 0`

## 4.4 Grafana 设计

- 数据源：Prometheus / Loki / Jaeger
- 数据源 UID：
  - Prometheus：`prometheus`
  - Loki：`loki`
  - Jaeger：`jaeger`
- 预置看板：`MeshCart Observability`

看板文件：

- `deploy/docker/observability/grafana/provisioning/dashboards/dashboards.yaml`
- `deploy/docker/observability/grafana/provisioning/dashboards/json/meshcart-observability.json`

## 5. 配置说明

## 5.1 应用环境变量

通用：

- `APP_ENV`：运行环境
- `LOG_LEVEL`：日志级别
- `OTEL_EXPORTER_OTLP_ENDPOINT`：OTLP 上报地址

建议本地值：

- `APP_ENV=dev`
- `LOG_LEVEL=info`
- `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4319`

user-service：

- `USER_METRICS_ADDR`：metrics 监听地址（默认 `:9091`）

## 5.2 端口说明

- Grafana：`3000`
- Prometheus：`9090`
- Loki：`3100`
- Jaeger UI：`16686`
- OTel Collector OTLP gRPC（宿主机）：`4319`
- OTel Collector OTLP HTTP（宿主机）：`4320`
- Gateway metrics：`8080/metrics`
- User-service metrics：`9091/metrics`

## 6. 启动说明

### 6.1 启动可观测性组件

```bash
docker compose up -d
```

### 6.2 重启 Grafana 以加载最新看板（首次或看板更新后）

```bash
docker compose restart grafana
```

### 6.3 启动业务服务

分别启动 `gateway` 与 `user-service`，并设置环境变量：

```bash
export APP_ENV=dev
export LOG_LEVEL=info
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4319
```

## 7. 使用说明

## 7.1 验证指标

- Gateway: `http://localhost:8080/metrics`
- User-service: `http://localhost:9091/metrics`
- Prometheus Targets: `http://localhost:9090/targets`

## 7.2 验证链路

1. 调用 `POST /api/v1/user/login`
2. 打开 Jaeger：`http://localhost:16686`
3. 查询 `gateway`、`user-service` 相关 trace

## 7.3 验证日志

1. 打开 Grafana：`http://localhost:3000`
2. 进入 Loki Explore
3. 用 `trace_id` 过滤对应链路日志

## 7.4 查看默认看板

Grafana -> Folder `MeshCart` -> `MeshCart Observability`

## 8. 常见问题

### 8.1 Jaeger 无链路

检查：

- `OTEL_EXPORTER_OTLP_ENDPOINT` 是否正确
- `otel-collector`、`jaeger` 容器是否运行
- 对应接口是否创建了 span

### 8.2 日志缺少 `trace_id`

检查：

- 是否使用 `log.L(ctx)` 打日志
- 是否正确透传了 context
- 当前上下文是否已有 span

### 8.3 Prometheus 抓取失败

检查：

- `gateway` 和 `user-service` metrics 端点是否可访问
- `prometheus.yml` target 是否可达
- Docker 环境下 `host.docker.internal` 是否可解析

### 8.4 Grafana 无数据

检查：

- 数据源状态是否 Healthy
- Prometheus 是否有对应时序数据
- Loki 是否收到日志（Promtail 是否运行）

## 9. 开发约束

- 新接口必须至少创建一个 server span
- 跨服务调用必须透传同一 context
- 日志必须使用 `log.L(ctx)` 输出
- 日志字段命名必须使用统一规范
- 新业务指标必须遵循统一前缀 `meshcart_`
