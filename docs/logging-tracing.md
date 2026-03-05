# 日志与链路追踪设计说明

## 1. 目标

本项目日志系统的目标：

- 输出结构化 JSON 日志，便于检索与聚合
- 统一字段命名，便于跨服务关联
- 支持在 `context` 中透传链路字段（`trace_id/span_id/user_id`）
- 为后续接入 Loki、OpenTelemetry 做准备

## 2. 当前实现位置

### 2.1 日志基础封装

- `app/log/logger.go`
- `app/log/context.go`

### 2.2 已接入服务

- Gateway
  - `gateway/cmd/gateway/main.go`
  - `gateway/internal/handler/user/login.go`
  - `gateway/internal/logic/user/login.go`
- User Service
  - `services/user-service/rpc/main.go`
  - `services/user-service/rpc/handler.go`

## 3. 日志模型

当前采用 `zap` 输出 JSON 日志。每条日志由两部分组成：

1. 基础字段（启动时注入）
2. 业务/链路字段（按请求上下文注入）

### 3.1 基础字段

- `service`：服务名（如 `gateway`、`user-service`）
- `env`：运行环境（如 `dev`、`test`、`prod`）
- `level`：日志级别（`debug/info/warn/error`）
- `msg`：日志消息
- `ts`：时间戳（ISO8601）
- `caller`：源码位置

### 3.2 链路字段

- `trace_id`：一次完整请求链路的唯一标识
- `span_id`：链路中某一步调用的标识
- `user_id`：当前请求关联的用户标识（如果已登录）

这些字段由 `context` 注入：

- `log.WithTraceID(ctx, traceID)`
- `log.WithSpanID(ctx, spanID)`
- `log.WithUserID(ctx, userID)`

然后通过 `log.L(ctx)` 自动带入日志。

## 4. 字段含义详解

### 4.1 `trace_id`

作用：将网关、微服务、数据库访问日志串成同一条业务链路。

典型用途：

- 用户报错时，拿到 `trace_id` 后跨服务检索全部日志
- 排查请求在哪个服务、哪个步骤失败

### 4.2 `span_id`

作用：表示链路中的一个“步骤”或“片段”。

示例：

- 网关接收请求是一个 span
- 网关调用 user-service 是另一个 span

当前项目已预留该字段，但尚未接入完整 OpenTelemetry span 生命周期管理。

### 4.3 `user_id`

作用：关联用户维度日志，便于运营审计与用户问题排查。

使用建议：

- 登录前接口可以为空
- 鉴权通过后注入到 context

## 5. 你的项目里，日志如何流动

以 `POST /api/v1/user/login` 为例：

1. Gateway handler 从请求头读取 `X-Trace-Id`
2. `trace_id` 写入 context：`log.WithTraceID(...)`
3. handler 与 logic 使用 `log.L(ctx)` 打日志
4. 调用 user-service 后，user-service 在 handler/logic 打日志
5. 最终通过 `trace_id` 将两个服务日志关联

## 6. 日志级别规范

建议按以下标准使用：

- `debug`：调试细节（默认线上关闭）
- `info`：关键业务事件（启动、成功流程）
- `warn`：可恢复异常（参数不合法、业务失败）
- `error`：技术异常（RPC失败、DB异常）

当前代码中：

- 参数错误、业务错误：`warn`
- RPC技术错误：`error`
- 启动事件、成功事件：`info`

## 7. 为什么做结构化日志

结构化日志不是拼接字符串，而是 key-value。

优点：

- 查询快：例如 `service="gateway" and trace_id="xxx"`
- 聚合简单：按 `level`、`service`、`code` 统计
- 统一可观测性：日志平台可直接识别字段

这也是后续接 Loki 的基础。

## 8. 与 Loki 的关系

Loki 主要做日志存储与检索，当前你已经完成最关键准备：

- JSON结构化输出
- 统一字段名
- trace字段透传

后续接入步骤通常是：

1. 用 Promtail/Agent 收集 stdout 日志
2. 发送到 Loki
3. 在 Grafana 里按 `service`、`trace_id` 查询

## 9. 链路追踪（Tracing）入门说明

你还没用过 tracing，先记住两点：

1. 日志用于“记录事件”
2. tracing 用于“记录调用关系与耗时”

完整 tracing 一般包括：

- `trace_id`：整条链路 ID
- `span_id`：每个节点 ID
- `parent_span_id`：父子关系
- `duration`：每个 span 耗时

当前项目状态：

- 已支持日志携带 `trace_id/span_id`
- 尚未接入完整 OTel 自动埋点

## 10. 后续接 OpenTelemetry 的最小方案

建议顺序：

1. Gateway 接入 OTel 中间件，自动生成 `trace_id/span_id`
2. RPC 调用透传 trace context 到 user-service
3. user-service 继续传递 context 并输出日志
4. 将 tracing 数据上报到 Jaeger/Tempo

这样你会同时得到：

- 日志可检索（Loki）
- 链路可视化（Jaeger/Tempo）

## 11. 常见问题

### 11.1 为什么日志里有 `trace_id` 但查不到下游日志？

可能原因：

- 下游服务没有透传 context
- 下游没有使用 `log.L(ctx)` 打日志
- 请求头没有带 `X-Trace-Id`

### 11.2 `span_id` 为空是不是异常？

不是。当前还没接完整 tracing SDK，`span_id` 为空是预期行为。

### 11.3 `user_id` 什么时候写入？

在鉴权完成后写入 context，后续链路日志自动携带。

## 12. 当前接口与调用方式

### 12.1 初始化日志

```go
err := log.Init(log.Config{
    Service: "gateway",
    Env:     "dev",
    Level:   "info",
})
```

### 12.2 写入链路字段

```go
ctx = log.WithTraceID(ctx, traceID)
ctx = log.WithSpanID(ctx, spanID)
ctx = log.WithUserID(ctx, userID)
```

### 12.3 打日志

```go
log.L(ctx).Info("user login success")
log.L(ctx).Warn("invalid param", zap.String("field", "username"))
log.L(ctx).Error("rpc failed", zap.Error(err))
```
