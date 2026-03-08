# MeshCart 可观测性设计与使用手册

## 1. 目标与范围

本文档描述 MeshCart 当前可观测性体系的设计与使用方式，覆盖：

- 日志：Zap（结构化日志）
- 指标：Prometheus（HTTP/RPC 指标采集）
- 链路追踪：OpenTelemetry + Jaeger
- 日志采集：Promtail -> Loki
- 可视化：Grafana（数据源 + 预置看板）

适用服务：

- `gateway`
- `user-service`

## 2. 总体架构

数据流如下：

1. 应用输出 Zap JSON 日志到 stdout，并同时写入项目 `logs/` 目录
2. Promtail 采集容器日志和项目日志文件并推送到 Loki
3. 应用通过 Hertz / Kitex 观测插件和 OpenTelemetry Provider 上报 trace 到 OTel Collector
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
- 开发阶段同时保留控制台输出与本地文件输出
- 本地日志文件使用滚动切分，避免单文件无限增长

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

- 服务启动时初始化 OpenTelemetry Provider
- Gateway 使用 `hertz-contrib/obs-opentelemetry` 完成 HTTP 入站 trace 提取与 server span 创建
- RPC 使用 `kitex-contrib/obs-opentelemetry` 完成 Kitex client/server span 创建与 trace 透传
- 业务层只在必要位置补充 internal span，不重复手写 HTTP server span 或 RPC client/server span

### 4.2.2 Span 命名规范

格式：`<domain>.<layer>.<module>.<action>`

示例：

- `gateway.logic.user.login`

### 4.2.3 链路传播

- Gateway 由 `hertz-contrib` tracing middleware 自动提取 HTTP Header 中的 trace context
- Gateway 内部调用保持同一 `context.Context`
- Gateway 调用 user-service 时，Kitex ClientSuite 自动透传 trace context
- User-service 由 Kitex ServerSuite 自动接收上游 trace context 并创建 RPC server span

## 4.3 Metrics 设计（Prometheus）

### 4.3.1 HTTP 指标（gateway）

- `hertz_server_throughput`
  - 类型：Counter
  - 标签：`method`, `statusCode`
- `hertz_server_latency_us`
  - 类型：Histogram
  - 标签：`method`, `statusCode`

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
- Loki API：`3100`
- Jaeger UI：`16686`
- OTel Collector OTLP gRPC（宿主机）：`4319`
- OTel Collector OTLP HTTP（宿主机）：`4320`
- Gateway metrics：`9092/metrics`
- User-service metrics：`9091/metrics`

## 5.3 访问入口总表

- Grafana 看板入口：`http://localhost:3000`
- Prometheus 控制台：`http://localhost:9090`
- Prometheus Targets：`http://localhost:9090/targets`
- Jaeger 链路查询：`http://localhost:16686`
- Loki HTTP API：`http://localhost:3100`
- Gateway 指标端点：`http://localhost:9092/metrics`
- User-service 指标端点：`http://localhost:9091/metrics`

说明：

- Grafana 默认账号密码：`admin/admin`
- Loki 通常不直接打开页面使用，主要通过 Grafana Explore 或 HTTP API 查询
- `http://localhost:3100` 根路径返回 `404` 属于正常现象，Loki 主要提供 API，不提供首页 UI

## 6. 启动说明

### 6.1 启动可观测性组件

```bash
docker compose up -d
```

### 6.2 重启 Grafana 以加载最新看板（首次或看板更新后）

```bash
docker compose restart grafana
```

### 6.2.1 配置变更后的重载规则

- 修改 `prometheus.yml` 后，需要重启或重建 `prometheus`
- 修改 `promtail-config.yml` 后，需要重启或重建 `promtail`
- 修改 `deploy/docker/observability/promtail/positions.yaml` 后，需要重启或重建 `promtail`
- 修改 Grafana provisioning/dashboard 后，需要重启 `grafana`
- 修改业务代码中的日志、trace、metrics 逻辑后，需要重启对应业务服务

### 6.2.2 开发阶段日志方案

- 业务日志同时输出到控制台和 `logs/` 目录
- 本地日志文件由 `lumberjack` 做滚动切分
- `promtail` 只采集 `logs/gateway.log` 和 `logs/user-service.log`
- `promtail` 的读取位点文件使用宿主机持久化，避免重启后反复从头扫描旧日志

### 6.3 启动业务服务

分别启动 `gateway` 与 `user-service`，并设置环境变量：

```bash
export APP_ENV=dev
export LOG_LEVEL=info
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4319
```

## 7. 使用说明

## 7.0 面板用途与使用指南

### 7.0.1 Grafana

地址：

- `http://localhost:3000`

用途：

- 统一查看指标、日志、链路
- 查看预置看板 `MeshCart Observability`
- 进入 Explore 手工查 Prometheus、Loki、Jaeger 数据

常用入口：

- Dashboards：看整体监控面板
- Explore -> Prometheus：查指标
- Explore -> Loki：查日志
- Explore -> Jaeger：跳转链路

### 7.0.2 Prometheus

地址：

- `http://localhost:9090`

用途：

- 查看原始 metrics 数据
- 验证抓取是否正常
- 手工执行 PromQL 查询

常用入口：

- `http://localhost:9090/targets`
  - 查看 `gateway`、`user-service` 是否抓取成功
- `Graph`
  - 手工查询指标，比如 `hertz_server_throughput`
  - 手工查询 RPC 指标，比如 `meshcart_rpc_requests_total`

### 7.0.3 Jaeger

地址：

- `http://localhost:16686`

用途：

- 查看一次请求的完整调用链
- 排查慢请求、错误请求、跨服务 trace 断链

常用方法：

- 选择 `gateway` 或 `user-service`
- 点击 `Find Traces`
- 打开某条 trace 后查看每个 span 的耗时、父子关系、错误信息

界面说明：

- 不同颜色主要用于区分不同 `service`
- 红色错误标记才表示该 span 被标记为 `error=true`
- 黄色、蓝色、绿色本身不表示告警等级

### 7.0.4 Loki

地址：

- `http://localhost:3100`

用途：

- Loki 是日志存储后端
- 一般不直接使用页面，而是通过 Grafana Explore 查询

常用方法：

- 打开 Grafana -> Explore -> 选择 `Loki`
- 查询 gateway 文件日志：
  - `{project="meshcart", source="file", service="gateway"}`
- 查询 user-service 文件日志：
  - `{project="meshcart", source="file", service="user-service"}`
- 查询容器日志：
  - `{project="meshcart", compose_service="loki"}`
- 按链路查询：
  - `{project="meshcart", source="file", service="gateway"} |= "trace_id"`

查看原则：

- 平时优先查 `source="file"` 的应用日志，不优先查容器内部组件日志
- 如果查到了 `service_name="meshcart-loki"` 或 `compose_service="loki"`，说明你看到的是 Loki 自己的查询日志，不是业务日志
- 查询业务日志时，先限定服务，再加 `trace_id`、`level`、`msg` 等条件收敛范围

推荐查询：

- 查 gateway 最近错误：
  - `{project="meshcart", source="file", service="gateway"} |= "\"level\":\"error\""`
- 查 user-service 最近警告：
  - `{project="meshcart", source="file", service="user-service"} |= "\"level\":\"warn\""`
- 按 trace_id 查整条链路：
  - `{project="meshcart", source="file"} |= "你的trace_id"`
- 查登录失败：
  - `{project="meshcart", source="file"} |= "user login failed"`

### 7.0.5 Metrics 端点

地址：

- Gateway：`http://localhost:9092/metrics`
- User-service：`http://localhost:9091/metrics`

用途：

- 直接查看服务暴露的原始指标
- 判断是“服务没产出指标”还是“Prometheus 没抓到指标”

## 7.1 验证指标

- Gateway: `http://localhost:9092/metrics`
- User-service: `http://localhost:9091/metrics`
- Prometheus Targets: `http://localhost:9090/targets`

## 7.2 验证链路

1. 调用 `POST /api/v1/user/login`
2. 打开 Jaeger：`http://localhost:16686`
3. 查询 `gateway`、`user-service` 相关 trace

## 7.3 验证日志

1. 打开 Grafana：`http://localhost:3000`
2. 进入 Loki Explore
3. 先查 `{project="meshcart", source="file", service="gateway"}` 或 `{project="meshcart", source="file", service="user-service"}`
4. 再结合 `trace_id` 或错误关键字过滤对应链路日志

## 7.4 查看默认看板

Grafana -> Folder `MeshCart` -> `MeshCart Observability`

## 8. 常见问题

### 8.1 Jaeger 无链路

检查：

- `OTEL_EXPORTER_OTLP_ENDPOINT` 是否正确
- `otel-collector`、`jaeger` 容器是否运行
- Gateway 是否注册了 `hztrace.ServerMiddleware(...)`
- Kitex client/server 是否注册了 `kitextrace.NewClientSuite()` / `kitextrace.NewServerSuite()`

### 8.1.1 Jaeger 红色 span 但接口可用

说明：

- Jaeger 中的红色 span 不一定表示接口不可用
- 推荐口径是：只有技术异常标记为 `Error`
- 普通业务失败（如参数错误、用户名密码错误、库存不足）不标记为 `Error`

当前项目约定：

- 技术错误：
  - `span.RecordError(err)`
  - `span.SetStatus(codes.Error, ...)`
- 业务错误：
  - 不设置 `codes.Error`
  - 使用 `biz.success=false`、`biz.type=business`、`biz.code`、`biz.message` 等属性记录

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
- 本地服务是否已在项目根目录生成 `logs/gateway.log` 或 `logs/user-service.log`
- 查询条件是否错误选成了 `loki`、`grafana` 等观测组件自身日志

### 8.5 Promtail 重启后日志查询不稳定

检查：

- `promtail` 是否已挂载持久化的 `positions.yaml`
- 业务日志文件是否持续滚动写入
- 查询时间范围是否覆盖到最新日志
- 是否存在大量过旧日志导致 `too far behind`

## 9. 开发约束

- HTTP server span 与 RPC client/server span 由框架插件自动生成
- 跨服务调用必须透传同一 context
- 日志必须使用 `log.L(ctx)` 输出
- 日志字段命名必须使用统一规范
- 新业务指标必须遵循统一前缀 `meshcart_`
- 仅在业务拆分有意义时补充 internal span，避免重复创建与框架职责冲突的 span

## 10. 单条链路追踪详解（以 user/login 为例）

本节按一次真实调用路径展开：

`HTTP 请求 -> gateway handler -> gateway logic -> gateway rpc client -> user-service rpc handler`

### 10.1 初始化阶段（服务启动时）

Gateway 启动文件：`gateway/cmd/gateway/main.go`  
User-service 启动文件：`services/user-service/rpc/main.go`

Gateway 调用函数：`otelprovider.NewOpenTelemetryProvider(...)`（`hertz-contrib`）  
User-service 调用函数：`otelprovider.NewOpenTelemetryProvider(...)`（`kitex-contrib`）

参数说明（Gateway / User-service `provider.Option`）：

- `WithServiceName`：服务名，写入资源属性
- `WithDeploymentEnvironment`：环境标识
- `WithExportEndpoint`：OTLP 上报地址（示例：`localhost:4319`）
- `WithInsecure`：是否使用非 TLS 方式连接 collector

初始化做了什么：

1. 创建 OTLP trace exporter（gRPC）
2. 构建 Resource（包含 `service.name`、`deployment.environment`）
3. 创建全局 TracerProvider
4. 设置全局 Propagator（W3C TraceContext + Baggage）

返回值说明：

- `Shutdown(context.Context)`：进程退出前调用，确保 span 刷盘上报

### 10.2 入站请求提取上下文（Gateway）

文件：`gateway/internal/handler/user/login.go`  
函数：`Login(svcCtx *svc.ServiceContext) app.HandlerFunc`

关键调用（在 `gateway/cmd/gateway/main.go` 注册）：

1. `hztrace.NewServerTracer()`  
2. `h.Use(hztrace.ServerMiddleware(traceCfg))`

作用：

- `hertz-contrib` 自动从请求头提取 `traceparent`
- 为当前 HTTP 请求创建 gateway 的入站 server span
- 把带有 span 的 `context.Context` 传给后续 handler / logic / rpc client

说明：

- 这一层不再手写 `StartSpan(..., SpanKindServer)`
- [login.go](/Users/ruitong/GolandProjects/MeshCart/gateway/internal/handler/user/login.go) 只负责取请求参数、调用 logic、写响应

### 10.3 业务层 span（Gateway Logic）

文件：[login.go](/Users/ruitong/GolandProjects/MeshCart/gateway/internal/logic/user/login.go)  
函数：`func (l *LoginLogic) Login(req *types.UserLoginRequest) ...`

关键调用：

- `tracex.StartSpan(ctx, "meshcart.gateway", "gateway.logic.user.login", SpanKindInternal)`

作用：

- 补充一个业务 internal span
- 把 handler 与下游 RPC 调用之间的业务耗时单独量化
- internal span 仍然挂在 gateway 的 HTTP server span 下面

### 10.4 出站 RPC span（Gateway -> user-service）

文件：[client.go](/Users/ruitong/GolandProjects/MeshCart/gateway/rpc/user/client.go)  
函数：`func (c *kitexClient) Login(ctx context.Context, req *LoginRequest) ...`

关键调用：

- `client.WithSuite(kitextrace.NewClientSuite())`
- `client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "gateway"})`
- `c.cli.Login(ctx, ...)`

重点：

- RPC client span 由 `kitex-contrib` 自动创建
- 当前代码不再手写 `gateway.rpc.user.login` 这一层 span
- 同一个 `ctx` 透传是跨服务链路串联的关键
- 如果替换了 ctx 或未透传，会导致 Jaeger 中链路断裂

### 10.5 下游入站 span（user-service）

文件：`services/user-service/rpc/main.go`  
业务处理文件：`services/user-service/rpc/handler.go`  
函数：`func (s *UserServiceImpl) Login(ctx context.Context, request *user.UserLoginRequest) ...`

关键调用：

- `server.WithSuite(kitextrace.NewServerSuite())`
- `server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "user-service"})`

作用：

- user-service 的 RPC server span 由 `kitex-contrib` 自动创建
- 与 gateway 侧的 RPC client span 形成父子关系，完成跨服务追踪闭环
- `handler.go` 中不再手写 `SpanKindServer`

### 10.6 状态与错误记录

当前实现分为两类：

- 框架自动 span
  - HTTP server span：由 `hertz-contrib` 创建
  - RPC client/server span：由 `kitex-contrib` 创建
- 业务补充 span
  - gateway logic 中的 internal span 仍可调用 `tracex.StartSpan(...)`

说明：

- 如果需要记录更细的业务标签，可以在 logic/service 层的 internal span 上调用 `SetAttributes(...)`
- 技术异常会体现在 RPC 返回错误、日志和 span 状态中，Jaeger 中可直接查看耗时与错误链路
- 普通业务失败不建议直接标红，而是写入业务属性

推荐属性：

- `biz.success`
- `biz.type`
- `biz.code`
- `biz.message`
- `biz.module`
- `biz.action`

当前 `gateway.logic.user.login` 的口径：

- 参数错误、用户名密码错误：
  - `biz.success=false`
  - `biz.type=business`
  - 不设置 `codes.Error`
- RPC 调用失败、网络异常、下游不可用：
  - `biz.success=false`
  - `biz.type=technical`
  - 设置 `codes.Error`
- 请求成功：
  - `biz.success=true`
  - `codes.Ok`

### 10.7 日志与 trace 关联

文件：[context.go](/Users/ruitong/GolandProjects/MeshCart/app/log/context.go)  
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
- `service.name`：服务名（来自 Provider 的 `WithServiceName`）

### 10.9 最小排障路径（实践）

1. 在 Jaeger 按服务/接口找到慢链路
2. 复制该链路 `trace_id`
3. 在 Grafana Loki 用 `trace_id` 检索日志
4. 结合 span status + error 日志定位问题点
