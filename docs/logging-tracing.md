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

## 10. 单条链路追踪详解（以 user/login 为例）

本节按一次真实调用路径展开：

`HTTP 请求 -> gateway handler -> gateway logic -> gateway rpc client -> user-service rpc handler`

### 10.1 初始化阶段（服务启动时）

Gateway 启动文件：`gateway/cmd/gateway/main.go`  
User-service 启动文件：`services/user-service/rpc/main.go`

调用函数：`tracex.Init(ctx, tracex.Config{...})`（位于 `app/trace/otel.go`）

参数说明（`tracex.Config`）：

- `ServiceName`：服务名，写入资源属性，Jaeger 中用于区分服务
- `Environment`：环境标识（dev/prod）
- `Endpoint`：OTLP 上报地址（示例：`localhost:4319`）
- `Insecure`：是否使用非 TLS 方式连接 collector

初始化做了什么：

1. 创建 OTLP trace exporter（gRPC）
2. 构建 Resource（包含 `service.name`、`deployment.environment`）
3. 创建全局 TracerProvider
4. 设置全局 Propagator（W3C TraceContext + Baggage）

返回值说明：

- `shutdown func(context.Context) error`：进程退出前调用，确保 span 刷盘上报

### 10.2 入站请求提取上下文（Gateway）

文件：`gateway/internal/handler/user/login.go`  
函数：`Login(svcCtx *svc.ServiceContext) app.HandlerFunc`

关键调用：

1. `tracex.ExtractFromHertz(ctx, c)`  
   位置：`app/trace/hertz.go`  
   作用：从请求头提取 `traceparent`，把上游 trace 写入当前 context。

2. `tracex.StartSpan(ctx, "meshcart.gateway", "gateway.user.login", SpanKindServer)`  
   位置：`app/trace/otel.go`  
   作用：创建 Gateway 入站 server span。

参数说明（`StartSpan`）：

- `ctx`：当前上下文，必须向后透传
- `tracerName`：Tracer 名称（示例：`meshcart.gateway`）
- `spanName`：span 名称（示例：`gateway.user.login`）
- `SpanKind`：
  - `Server`：入站请求
  - `Internal`：进程内业务步骤
  - `Client`：出站调用下游

### 10.3 业务层 span（Gateway Logic）

文件：`gateway/internal/logic/user/login.go`  
函数：`func (l *LoginLogic) Login(req *types.UserLoginRequest) ...`

关键调用：

- `tracex.StartSpan(ctx, "meshcart.gateway", "gateway.logic.user.login", SpanKindInternal)`

作用：

- 把 handler 与下游 RPC 调用之间的业务耗时单独量化

### 10.4 出站 RPC span（Gateway -> user-service）

文件：`gateway/rpc/user/client.go`  
函数：`func (c *kitexClient) Login(ctx context.Context, req *LoginRequest) ...`

关键调用：

- `tracex.StartSpan(ctx, "meshcart.gateway", "gateway.rpc.user.login", SpanKindClient)`
- `c.cli.Login(ctx, ...)`（使用同一个 ctx 发起 kitex 调用）

重点：

- 同一个 `ctx` 透传是跨服务链路串联的关键
- 如果替换了 ctx 或未透传，会导致 Jaeger 中链路断裂

### 10.5 下游入站 span（user-service）

文件：`services/user-service/rpc/handler.go`  
函数：`func (s *UserServiceImpl) Login(ctx context.Context, request *user.UserLoginRequest) ...`

关键调用：

- `tracex.StartSpan(ctx, "meshcart.user-service", "user.rpc.login", SpanKindServer)`

作用：

- 与上游 client span 形成父子关系，完成跨服务追踪闭环

### 10.6 状态与错误记录

当前代码中使用：

- `span.SetStatus(codes.Ok, "ok")`：成功状态
- `span.SetStatus(codes.Error, "...")`：失败状态
- `span.RecordError(err)`：记录技术异常
- `span.SetAttributes(...)`：记录业务标签（如 `user.username`）

这些字段可在 Jaeger Span 详情中直接查看。

### 10.7 日志与 trace 关联

文件：`app/log/context.go`  
函数：`ContextFields(ctx context.Context) []zap.Field`

逻辑：

1. 先尝试从业务 context 读取 `trace_id/span_id`
2. 若没有，则从 OTel span context 自动提取 `TraceID/SpanID`
3. 最终由 `log.L(ctx)` 注入日志字段

效果：

- 同一条请求在 Jaeger 和 Loki 可通过 `trace_id` 互相定位

### 10.8 核心字段释义（链路层）

- `trace_id`：整条链路唯一 ID（跨服务不变）
- `span_id`：当前步骤 ID（每个 span 独立）
- `parent_span_id`：父 span ID（由 OTel 内部维护）
- `span.kind`：步骤类型（server/internal/client）
- `service.name`：服务名（来自 `tracex.Config.ServiceName`）

### 10.9 最小排障路径（实践）

1. 在 Jaeger 按服务/接口找到慢链路
2. 复制该链路 `trace_id`
3. 在 Grafana Loki 用 `trace_id` 检索日志
4. 结合 span status + error 日志定位问题点
