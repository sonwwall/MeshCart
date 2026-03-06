# 可观测性设计与使用说明

## 1. 文档范围

本文档描述 MeshCart 当前可观测性方案的设计与使用方式，覆盖以下组件：

- 日志：Zap（结构化 JSON）
- 链路追踪：OpenTelemetry SDK + Jaeger
- 日志存储检索：Loki + Promtail
- 指标：Prometheus
- 可视化：Grafana

## 2. 总体架构

### 2.1 数据流

1. 应用（gateway / user-service）输出结构化日志（stdout）
2. Promtail 采集容器日志并推送到 Loki
3. 应用通过 OTel SDK 上报 trace 到 OTel Collector
4. OTel Collector 转发 trace 到 Jaeger
5. OTel Collector 暴露 metrics，Prometheus 抓取
6. Grafana 统一读取 Prometheus / Loki / Jaeger 数据源

### 2.2 部署组件

通过 `docker-compose.yml` 部署以下基础设施：

- `prometheus`
- `grafana`
- `loki`
- `promtail`
- `jaeger`
- `otel-collector`

配置文件位置：`deploy/docker/observability/`

## 3. 代码设计

## 3.1 日志模块（`app/log`）

### 3.1.1 目标

- 统一日志格式
- 统一字段命名
- 自动携带链路字段

### 3.1.2 关键文件

- `app/log/logger.go`：Zap 初始化、全局 logger、级别控制
- `app/log/context.go`：从 context 读取并注入日志字段

### 3.1.3 字段规范

统一字段名：

- `service`：服务名
- `env`：环境
- `trace_id`：链路 ID
- `span_id`：Span ID
- `user_id`：用户 ID（已登录场景）
- `level`：日志级别
- `msg`：日志消息
- `ts`：时间
- `caller`：代码位置

说明：

- `trace_id/span_id` 优先从业务 context 字段读取
- 若 context 中存在 OTel span，上述字段会自动从 span context 补齐

## 3.2 Trace 模块（`app/trace`）

### 3.2.1 关键文件

- `app/trace/otel.go`
  - 初始化 OTel TracerProvider
  - 配置 OTLP gRPC exporter
  - 设置全局 Propagator（W3C TraceContext + Baggage）
  - 提供 `StartSpan/TraceID/SpanID` 工具方法

- `app/trace/hertz.go`
  - Hertz 请求头 carrier
  - 从 HTTP 请求头提取 trace context
  - 向请求头注入 trace context

### 3.2.2 Span 命名

当前命名规范：`<domain>.<layer>.<module>.<action>`

示例：

- `gateway.user.login`
- `gateway.logic.user.login`
- `gateway.rpc.user.login`
- `user.rpc.login`

## 3.3 服务接入点

### 3.3.1 启动初始化

- `gateway/cmd/gateway/main.go`
- `services/user-service/rpc/main.go`

启动时初始化：

1. `log.Init(...)`
2. `trace.Init(...)`
3. 服务启动

并在退出时调用 `log.Sync()`、`traceShutdown()`。

### 3.3.2 请求链路接入

Gateway 登录请求：

1. handler 从 Header 提取 trace context（`ExtractFromHertz`）
2. 创建 server span（`gateway.user.login`）
3. logic 创建 internal span（`gateway.logic.user.login`）
4. rpc client 创建 client span（`gateway.rpc.user.login`）
5. user-service handler 创建 server span（`user.rpc.login`）

## 4. 配置说明

## 4.1 应用配置（环境变量）

- `APP_ENV`：环境标识（如 `dev` / `prod`）
- `LOG_LEVEL`：日志级别（如 `info`）
- `OTEL_EXPORTER_OTLP_ENDPOINT`：OTLP 上报地址

本地使用当前 compose 时建议：

- `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4319`

## 4.2 观测组件端口

- Grafana：`3000`
- Prometheus：`9090`
- Loki：`3100`
- Jaeger UI：`16686`
- OTel Collector OTLP gRPC：`4319`（宿主机）
- OTel Collector OTLP HTTP：`4320`（宿主机）

## 5. 使用步骤

### 5.1 启动可观测性组件

```bash
docker compose up -d
```

### 5.2 启动应用

分别启动 gateway、user-service，并设置：

```bash
export APP_ENV=dev
export LOG_LEVEL=info
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4319
```

### 5.3 触发请求

调用 `POST /api/v1/user/login`。

### 5.4 查看结果

- Jaeger（`http://localhost:16686`）查看 trace
- Grafana（`http://localhost:3000`）查看 Loki 日志与 Prometheus 指标
- Loki 中按 `trace_id` 过滤日志

## 6. 日志与链路关联方式

关联键使用 `trace_id`：

- 在 Jaeger 中获取 trace id
- 在 Loki 中按该 `trace_id` 查询同链路日志

这是当前问题排查的核心路径：

1. Jaeger 定位慢/失败 span
2. Loki 查看该 trace_id 对应详细日志

## 7. 开发约束

- 新接口必须创建至少一个 server span
- 跨服务调用必须使用同一 context 透传
- 日志必须使用 `log.L(ctx)`，避免丢失 trace 信息
- 日志字段命名不得自定义变体（如 `traceId`）

## 8. 常见问题

### 8.1 Jaeger 看不到链路

检查：

- `OTEL_EXPORTER_OTLP_ENDPOINT` 是否正确
- `otel-collector` 与 `jaeger` 是否正常运行
- 代码是否实际创建了 span

### 8.2 日志中没有 trace_id

检查：

- 是否使用 `log.L(ctx)`
- handler 是否提取并透传了 context
- span 是否在当前 context 中

### 8.3 Grafana 无日志

检查：

- promtail 是否运行
- 容器日志目录挂载是否可读
- Loki 数据源是否健康

## 9. 后续扩展建议

- 接入 OTel Metrics SDK（应用级 RED 指标）
- 在 Grafana 增加业务看板（登录成功率、错误率、延迟分位）
- 接入 Alertmanager 与告警规则
