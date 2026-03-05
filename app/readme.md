app/ 这层是“跨服务共享能力层”，放通用组件，避免每个微服务重复造轮子

app/common

放全局通用定义：错误码、通用响应结构、常量、基础类型。
例子：errors.go 里定义业务错误码和统一错误映射。
app/middleware

放可复用中间件（HTTP 和 RPC 都可以有各自实现）。
例子：鉴权、请求日志、限流、幂等校验、超时控制、trace 注入。
app/log

放统一日志封装，屏蔽具体日志库差异。
例子：初始化 logger、统一字段（trace_id/user_id/order_id）、按级别输出。
app/trace

放链路追踪初始化和工具函数。
例子：OpenTelemetry 初始化、span 创建、上下文透传、traceID 提取。
app/mq

放消息队列的统一封装。
例子：Producer/Consumer 初始化、消息序列化、重试、死信、幂等消费辅助。
app/xconfig

放配置加载与管理（x 一般表示扩展/统一封装）。
例子：读取本地配置+环境变量、按环境切换、配置结构体、热更新钩子。
