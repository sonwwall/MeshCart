# Architecture

MeshCart 当前采用 Gateway + RPC 微服务架构。

系统边界：

- `gateway`
  - 对外提供 HTTP 接口
  - 负责路由、参数校验、调用下游 RPC、统一返回
  - 接入 Hertz 生态可观测性组件
- `user-service`
  - 对内提供 Kitex RPC 接口
  - 负责用户域业务逻辑与数据访问
  - 接入 Kitex 生态可观测性组件

依赖关系：

- 客户端 -> `gateway`
- `gateway` -> `user-service`
- `gateway` / `user-service` -> MySQL
- `gateway` / `user-service` -> OTel Collector
- `Prometheus` 抓取 `gateway` / `user-service` metrics
- `Promtail` 采集业务日志并写入 `Loki`
- `Grafana` 统一展示 `Prometheus` / `Loki` / `Jaeger`

See:

- [Gateway 分层设计](./gateway-design.md)
- [错误码规范](./error-code.md)
- [日志与链路追踪](./logging-tracing.md)

可观测性面板与入口：

- Grafana：`http://localhost:3000`
- Prometheus：`http://localhost:9090`
- Jaeger：`http://localhost:16686`
- Loki API：`http://localhost:3100`
- Gateway metrics：`http://localhost:9092/metrics`
- User-service metrics：`http://localhost:9091/metrics`

详细使用方式见：

- [日志与链路追踪](./logging-tracing.md)
