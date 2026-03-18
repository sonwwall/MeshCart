# 微服务治理规划

## 1. 目的

本文档用于单独说明 MeshCart 在当前阶段应该补哪些微服务治理能力，以及哪些能力应当留到后续服务规模扩大后再建设。

这里的“治理”重点不是追求完整平台能力，而是解决当前最容易影响开发效率、稳定性和排障效率的问题。

## 2. 当前判断

MeshCart 目前已经具备以下基础：

- `gateway + user-service + product-service` 的微服务骨架
- `gateway -> user-service/product-service` 的 Kitex RPC 调用
- 基于 Consul 的基础服务发现
- 基础日志、指标、链路追踪能力
- 基于 JWT 的登录态
- 基于 Casbin 的网关侧授权控制

当前阶段的核心问题不是“服务能不能继续拆”，而是：

- 故障时的排障入口还不够清晰
- 服务虽然已经拆开，但治理能力还主要停留在“能注册、能调用”
- 超时治理虽然已经补齐，但仍需持续用业务链路验证预算是否合理
- 当前开发方式仍以宿主机直跑业务服务为主，调试效率优先于部署形态统一

当前环境约束：

- MySQL 与 Redis 主要使用远程环境，不以本机容器资源为前提
- `docker compose` 当前主要承担 Consul 与可观测性依赖，不承担业务服务主调试入口
- 业务服务如果过早切到 Compose，会提高断点调试、日志查看和快速迭代成本

因此，当前更适合建设“最小治理基线”，而不是一次性引入完整治理体系。

## 3. 已完成的治理能力

### 3.1 超时治理

超时治理已经完成第一阶段落地，当前统一了四层 timeout：

- HTTP 入口连接与 I/O 超时
- HTTP request-level 总超时
- `gateway -> rpc` 调用超时
- 服务内数据库访问超时

治理目标：

- 避免请求无限等待
- 避免上游超时预算和下游超时预算互相冲突
- 避免链路卡死后只能靠日志猜问题

当前已完成：

- `gateway` 的 Hertz 启动层已显式配置 HTTP read timeout、write timeout 与 idle timeout
- `gateway` 已补一层 request-level 的协作式总超时中间件
- `gateway -> user-service` 与 `gateway -> product-service` 的 Kitex Client 已显式配置 connect timeout 和 rpc timeout
- `user-service` 与 `product-service` 的 repository 已统一套上数据库查询超时
- timeout 已经进入配置层，而不是继续写死在调用点

设计落点：

- `gateway/config/config.go`
  - `GATEWAY_READ_TIMEOUT_MS`
  - `GATEWAY_WRITE_TIMEOUT_MS`
  - `GATEWAY_IDLE_TIMEOUT_MS`
  - `GATEWAY_REQUEST_TIMEOUT_MS`
  - `USER_RPC_CONNECT_TIMEOUT_MS`
  - `USER_RPC_TIMEOUT_MS`
  - `PRODUCT_RPC_CONNECT_TIMEOUT_MS`
  - `PRODUCT_RPC_TIMEOUT_MS`
- `gateway/internal/component/server.go`
  - 通过 `server.WithReadTimeout(...)`、`server.WithWriteTimeout(...)` 与 `server.WithIdleTimeout(...)` 显式配置网关 HTTP 入口超时
- `gateway/internal/middleware/timeout.go`
  - 通过 `context.WithTimeout(...)` 为整条请求链增加 request-level deadline
  - 当下游链路尊重 `ctx.Done()` 且未写响应时，统一返回“服务繁忙，请稍后重试”
- `gateway/rpc/user/client.go`
  - 通过 `client.WithConnectTimeout(...)` 和 `client.WithRPCTimeout(...)` 显式配置下游用户服务调用超时
- `gateway/rpc/product/client.go`
  - 通过 `client.WithConnectTimeout(...)` 和 `client.WithRPCTimeout(...)` 显式配置下游商品服务调用超时
- `services/user-service/config/config.go`
  - `timeout.db_query_ms`
- `services/product-service/config/config.go`
  - `timeout.db_query_ms`
- `services/user-service/biz/repository/repository.go`
  - 所有 repository 读写操作统一套 `context.WithTimeout(...)`
- `services/product-service/biz/repository/repository.go`
  - 所有 repository 查询、更新、事务写入统一套 `context.WithTimeout(...)`

预算口径：

- HTTP read timeout：默认 `5000ms`
- HTTP write timeout：默认 `5000ms`
- HTTP idle timeout：默认 `60000ms`
- HTTP request timeout：默认 `3000ms`
- RPC connect timeout：默认 `500ms`
- RPC timeout：默认 `2000ms`
- DB query timeout：默认 `1500ms`

设计原则：

- 把 `gateway -> rpc -> db` 这条链路的超时预算先明确下来
- 把最外层 HTTP 入口也纳入预算控制，避免连接与请求读取长期悬挂
- 保证 DB timeout 小于整体 RPC timeout，避免下游数据库还在等待时，上游先完全失控
- 先把最容易出问题的链路显式收口，再决定后续是否继续细分读写预算

当前边界：

- 这次在 `gateway` 落地的是 Hertz 的 HTTP read/write/idle timeout
- 它主要约束的是连接与 I/O 层超时，不完全等于“整个 handler 执行超时”
- 当前又补了一层 request-level 的协作式 timeout middleware，用来约束整条请求链的总预算
- 这层 timeout 能覆盖 `gateway -> rpc -> db` 这类会尊重 `ctx.Done()` 的调用链
- 它不会强行打断一个完全不检查 `ctx` 的纯阻塞 handler，因此仍属于“协作式总超时”

### 3.1.1 如何观测

当前超时主要通过以下方式观测：

- 日志
  - `gateway` 的 logic 层会记录下游 RPC 技术错误
  - `user-service` / `product-service` 的业务层和 repository 层在底层错误时会留下日志
- trace
  - `gateway` 与下游服务之间已经接入 tracing
  - 当 RPC 超时时，span 会记录 error，并能在链路中看到失败节点
- metrics
  - 当前 RPC handler 已经有方法级耗时与状态观测
  - 超时会体现在对应 RPC 方法的错误数和耗时分布上
  - HTTP 入口读写超时会体现在网关 HTTP 请求层的异常响应与连接关闭上
  - request-level timeout 会体现在网关接口耗时上限附近的失败响应

观测重点：

- 是否是 connect timeout
- 是否是 rpc timeout
- 是否是 service 内部 DB query timeout
- 是单点偶发，还是持续性故障

观测注意点：

- `gateway` 的 HTTP read/write timeout 发生在 Hertz 框架层，可能不会进入业务 `handler -> logic` 的统一返回链路
- 因此它更适合作为入口保护和排障信号，而不是业务错误表达层
- `gateway` 的 request-level timeout 依赖下游逻辑尊重 `ctx.Done()`，因此观测时要区分“deadline 已下发”和“代码是否真正提前退出”
- `ConnectTimeout` 在 Kitex 下可能会叠加其默认重试策略，因此最终总耗时不一定严格等于单次 connect timeout
- 观察时要区分“单次建连超时”和“整体调用在重试后失败”这两个层次

### 3.1.2 超时后的返回策略

当前不会直接把底层错误原文返回给前端，而是按超时发生层次分别处理。

当前返回规则：

- HTTP read timeout / write timeout
  - 当前由 Hertz 框架层直接返回超时响应
  - 当前已验证的请求读取超时场景，会在日志中出现 `connection read timeout`
  - 对客户端的默认返回为 Hertz 的框架级错误响应：`400 Bad Request`，body 为 `Error when parsing request`
  - 这类超时通常不会走 `common.Fail(...)` 的统一 JSON 包装
- HTTP request timeout
  - 当下游链路尊重 `ctx.Done()` 且超时后未自行写响应时，由网关中间件统一返回
  - 对外返回：`服务繁忙，请稍后重试`
  - 当前仍沿用网关业务错误的统一 JSON 包装和 `HTTP 200`
- timeout / deadline exceeded
  - 对外返回：`服务繁忙，请稍后重试`
- connection refused / no available / dial tcp / broken pipe 等连接阶段错误
  - 对外返回：`下游服务暂不可用，请稍后重试`
- 其他技术错误
  - 对外返回：`系统内部错误`

实现位置：

- `gateway/internal/logic/logicutil/rpc_error.go`
- `app/common/errors.go`

设计原因：

- 避免把底层网络错误和内部实现细节直接暴露给前端
- 让前端拿到更稳定、可理解的错误口径
- 给后续更细的技术错误分类预留空间

### 3.1.3 测试覆盖

当前围绕超时治理已经补了五层测试。

第一层：HTTP 入口超时测试

- `gateway/internal/component/server_test.go`

覆盖内容：

- `NewGatewayServer(...)` 是否真的把 HTTP read/write/idle timeout 接进 Hertz
- Hertz 的 read timeout 是否会在真实 TCP 请求读取过程中触发

测试方式：

- 构造带有 HTTP timeout 的 Hertz server
- 使用原始 TCP 连接发送不完整 HTTP 请求头
- 故意等待超过 `ReadTimeout`
- 断言服务端返回 Hertz 默认的框架级错误响应，而不是业务 JSON 包装

第二层：HTTP request-level timeout 测试

- `gateway/internal/middleware/timeout_test.go`

覆盖内容：

- request timeout 是否会在下游 handler 尊重 `ctx.Done()` 时真实触发
- 中间件是否会在超时且未写响应时统一返回“服务繁忙，请稍后重试”
- 下游如果已经写了自己的响应，中间件是否会保留现有返回而不是覆盖

测试方式：

- 起本地 Hertz server 并挂载 request timeout middleware
- 构造一个等待 `ctx.Done()` 的慢 handler
- 断言请求在超时后返回统一 JSON 错误
- 再构造一个在 `ctx.Done()` 后自行写响应的 handler，断言中间件不会覆盖该响应

第三层：真实 RPC timeout 行为测试

- `gateway/rpc/user/client_test.go`
- `gateway/rpc/product/client_test.go`

覆盖内容：

- 下游服务处理慢时，`RPCTimeout` 会真实触发
- 建连阶段阻塞时，`ConnectTimeout` 会真实触发

测试方式：

- 本地起一个测试用 Kitex server
- 对 `RPCTimeout` 场景，服务端 handler 故意 `sleep`
- 对 `ConnectTimeout` 场景，测试注入一个可控的 `Dialer`，在建连阶段故意阻塞到超过 connect timeout
- 断言错误确实属于 timeout / deadline exceeded 类错误

第四层：真实 DB timeout 行为测试

- `services/user-service/biz/repository/repository_test.go`
- `services/product-service/biz/repository/repository_test.go`

测试方式：

- 使用内存 sqlite 启动 GORM
- 在 query callback 中故意阻塞到 `ctx.Done()`
- 调用真实 repository 方法
- 断言最终返回 `context deadline exceeded`

第五层：错误映射测试

- `gateway/internal/logic/logicutil/rpc_error_test.go`
- `gateway/internal/logic/user/login_test.go`
- `gateway/internal/logic/user/register_test.go`

覆盖内容：

- timeout 是否会映射成“服务繁忙，请稍后重试”
- 连接失败是否会映射成“下游服务暂不可用，请稍后重试”
- `user` 业务链路是否真正使用了这套映射

测试结论：

- gateway HTTP read timeout：已有真实行为测试
- gateway HTTP timeout 配置接线：已有组件层测试
- gateway HTTP request timeout：已有真实行为测试
- `RPCTimeout`：已有真实行为测试
- `ConnectTimeout`：已有真实行为测试
- repository DB timeout：已有真实行为测试
- 超时后的 gateway 对外返回：已有映射测试

这意味着当前第一阶段已落地的 timeout 能力，已经具备基本可回归的测试闭环。

当前仍未落地：

- 强制型的 HTTP 整体处理超时
- 当前已配置的是 Hertz 连接与 I/O 层 timeout，加上一层协作式 request timeout，还不是“能强杀任意阻塞 handler”的最终形态

### 3.2 网关入口限流

网关入口限流已经完成第一阶段落地，当前采用“全局宽松兜底 + 重点接口严格限流”的两层策略。

当前已完成：

- `/api/v1/*`
  - 按 IP 维度做全局宽松限流
  - 用于给所有网关接口提供第一层兜底保护
- `POST /api/v1/user/login`
  - 按 IP + 路由维度限流
- `POST /api/v1/user/register`
  - 按 IP + 路由维度限流
- 商品管理写接口
  - 按用户 + 路由维度限流
  - 按接口 + 路由维度限流

设计落点：

- `gateway/config/config.go`
  - `GATEWAY_RATE_LIMIT_ENABLED`
  - `GATEWAY_RATE_LIMIT_ENTRY_TTL_MS`
  - `GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL_MS`
  - `GATEWAY_GLOBAL_IP_RATE_LIMIT_RPS`
  - `GATEWAY_GLOBAL_IP_RATE_LIMIT_BURST`
  - `GATEWAY_LOGIN_IP_RATE_LIMIT_RPS`
  - `GATEWAY_LOGIN_IP_RATE_LIMIT_BURST`
  - `GATEWAY_REGISTER_IP_RATE_LIMIT_RPS`
  - `GATEWAY_REGISTER_IP_RATE_LIMIT_BURST`
  - `GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_RPS`
  - `GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_BURST`
  - `GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_RPS`
  - `GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_BURST`
- `gateway/internal/middleware/ratelimit.go`
  - 提供通用限流中间件
  - 提供按 IP、按 IP + 路由、按用户、按接口的 key 生成策略
  - 使用进程内存维护 limiter store，并按 TTL 做过期清理
- `gateway/internal/handler/routes.go`
  - 在 `/api/v1` 分组上接入全局 IP 限流
- `gateway/internal/handler/user/routes.go`
  - 在 `login/register` 路由级接入限流
- `gateway/internal/handler/product/routes.go`
  - 在商品管理写接口分组上接入用户限流和接口限流

字段说明：

- `GATEWAY_RATE_LIMIT_ENABLED`
  - 是否开启网关限流总开关
  - 关闭后所有第一阶段限流规则都不生效
- `GATEWAY_RATE_LIMIT_ENTRY_TTL_MS`
  - 单个限流 key 在内存 store 中的存活时间
  - 超过该时间未再访问的 IP / 用户 / 路由桶会被视为过期并清理
- `GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL_MS`
  - 后台执行过期清理的最小间隔
  - 用于避免每次请求都遍历整个 limiter map
- `GATEWAY_GLOBAL_IP_RATE_LIMIT_RPS`
  - 所有 `/api/v1` 接口共享的单个 IP 每秒放行速率
  - 用于给整个网关入口提供宽松兜底保护
- `GATEWAY_GLOBAL_IP_RATE_LIMIT_BURST`
  - 所有 `/api/v1` 接口共享的单个 IP 瞬时突发容量
  - 用于容忍单个 IP 的短时访问峰值
- `GATEWAY_LOGIN_IP_RATE_LIMIT_RPS`
  - 登录接口单个 IP 的每秒放行速率
  - 用于控制 `POST /api/v1/user/login` 的持续请求速度
- `GATEWAY_LOGIN_IP_RATE_LIMIT_BURST`
  - 登录接口单个 IP 的瞬时突发容量
  - 允许短时间内超过 RPS 的小波峰流量
- `GATEWAY_REGISTER_IP_RATE_LIMIT_RPS`
  - 注册接口单个 IP 的每秒放行速率
  - 通常应比登录更严格
- `GATEWAY_REGISTER_IP_RATE_LIMIT_BURST`
  - 注册接口单个 IP 的瞬时突发容量
  - 控制注册接口短时间爆发流量
- `GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_RPS`
  - 商品管理写接口单个用户的每秒放行速率
  - 用于限制同一管理员连续高频写操作
- `GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_BURST`
  - 商品管理写接口单个用户的瞬时突发容量
  - 允许管理员在短时间内执行少量连续写请求
- `GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_RPS`
  - 商品管理写接口单个路由的每秒放行速率
  - 用于保护某个写接口被整体打爆
- `GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_BURST`
  - 商品管理写接口单个路由的瞬时突发容量
  - 用于给单个写接口保留有限的短时峰值承载能力

口径说明：

- `RPS`
  - 表示 steady-state 的持续通过速率
- `Burst`
  - 表示桶容量，也就是允许短时间透支的请求数
- `Entry TTL`
  - 只影响 limiter 状态在内存里的保留时间，不影响单次请求的超时或缓存语义

当前设计原则：

- 只在 `gateway` 做第一层入口保护，不在 RPC 层同步引入第二套限流
- 所有 `/api/v1` 先挂一层宽松的全局 IP 限流，再对重点接口叠加更严格规则
- 先做单机内存版，不在当前阶段引入 Redis 分布式限流
- 限流 key 使用稳定路由模板，而不是原始 URL，避免路径参数把桶打散
- 对外继续沿用统一 JSON 错误包装，命中限流时返回“请求过于频繁，请稍后再试”

### 3.2.1 测试覆盖

- `gateway/internal/middleware/ratelimit_test.go`

覆盖内容：

- 同一个 key 在短时间内超过桶容量后会被拒绝
- 不同 key 的限流桶互不影响
- 全局 IP 限流会在不同路由之间共享同一个 IP 桶
- 内存 store 会清理长时间不再访问的 limiter，避免 map 持续增长

### 3.2.2 以后要做的限流

当前第一阶段已经把网关入口的基础护栏补上，但后续限流仍有几项值得逐步推进。

后续可考虑：

- 分布式限流
  - 当 `gateway` 进入多实例部署后，把当前内存 store 替换为 Redis 等共享存储
  - 目标是让同一个 IP / 用户在不同网关实例之间共享同一份配额
- 更细粒度的用户限流
  - 当前主要保护登录、注册和商品管理写接口
  - 后续可扩展到 `refresh_token`、用户管理写接口、订单相关写接口
- 热点接口专项限流
  - 对访问量明显偏高的单个接口设置独立配额
  - 避免全局限流过松而热点接口仍被打爆
- 按租户 / 角色 / 业务动作限流
  - 当系统出现租户隔离、多角色运营、不同业务动作优先级时再引入
  - 例如管理员写操作、普通用户下单、后台批量导入可使用不同配额
- RPC 层自保护限流
  - 当前不在 RPC 层重复做一套限流
  - 后续当订单、库存、支付等核心服务出现明显资源争抢时，再在服务侧补自保护限流
- 观测与告警
  - 后续应补“限流命中次数、命中接口、命中 key 类型”的 metrics 和日志口径
  - 让限流从“只能拦请求”变成“可观测、可调参”

当前不急着做这些能力的原因：

- 现阶段服务数量和实例规模还不大
- 当前更需要先验证现有限流阈值是否合理
- 过早引入分布式限流和复杂维度规则，会明显增加配置与排障复杂度

### 3.3 运行与排障手册

运行与排障手册已经完成第一阶段落地，当前落点：

- [docs/runbook.md](/Users/ruitong/GolandProjects/MeshCart/docs/runbook.md)

当前已覆盖：

- 本地启动顺序
- Docker 依赖启动
- `gateway` / `user-service` / `product-service` 启动说明
- 最小功能验证命令
- Consul / metrics / trace / 日志查看入口
- 常见问题排查路径

这项能力虽然不是“治理代码”，但它已经属于治理基线的一部分。

原因：

- 当前服务已经不是单体结构
- 故障定位成本已经明显高于单体阶段
- 没有统一 runbook 时，启动、联调、排障口径会持续发散

### 3.4 最小验收测试

最小验收测试已经完成第一阶段落地。

当前落点：

- `gateway/integration/gateway_acceptance_test.go`
- `gateway/integration/consul_acceptance_test.go`

当前已完成：

- 登录链路验收测试
- 商品列表链路验收测试
- 全局限流接线验收测试
- 超时响应关键路径验收测试
- 下游不可用错误映射验收测试
- Consul 注册发现验收测试

当前这批测试验证的是：

- `gateway -> user-service` 登录链路在网关真实路由接线下可用
- `gateway -> product-service` 商品列表链路在网关真实路由接线下可用
- `/api/v1` 全局限流已经真实挂到路由分组上，而不是只有中间件单测
- request timeout 和下游不可用错误在网关真实 handler 链路下仍会返回统一对外文案
- 临时 `user-service` 实例可以真实注册到 Consul，并被 `gateway/rpc/user` 的 Consul 模式 client 发现和调用

#### 3.4.1 设计目标

最小验收测试的目标不是把所有接口测完，而是先证明以下事情是真的：

- 关键业务链路能跑通
- 已落地治理能力在真实接线后仍然生效
- 服务发现不是“配置存在”，而是实际可用

换句话说，它验证的是“整条主干链路是活的”，而不是某个局部函数单独正确。

#### 3.4.2 设计边界

当前最小验收测试不追求：

- 全量接口覆盖
- 全量业务分支覆盖
- 所有异常场景一次补齐
- 重型端到端环境编排

当前只优先覆盖：

- 用户主链路
- 商品主链路
- 限流关键接线
- 超时与异常返回关键接线
- Consul 注册发现最小闭环

#### 3.4.3 分层方式

当前验收测试和已有单测、组件测试刻意分层：

- 单元/组件测试
  - 继续放在原有模块目录
  - 验证 timeout、ratelimit、rpc client、repository 等局部行为
- 验收测试
  - 放在 `gateway/integration`
  - 验证真实网关路由接线下的关键业务链路和治理链路

这样做的原因：

- 避免把验收测试和纯单测混在一起
- 后续继续补 Consul、生命周期、更多业务链路时，有统一目录可扩展
- 让“局部正确”和“整链路可用”这两类测试职责更清晰

#### 3.4.4 当前实现方式

当前验收测试主要采用两种方式：

- 网关验收测试
  - 起一个真实的 Hertz 网关路由服务
  - 把下游 `UserClient` / `ProductClient` 替换成测试桩
  - 验证 HTTP 入口、middleware、handler、logic、统一响应这条链路
- Consul 验收测试
  - 起一个临时 Kitex `user-service`
  - 把它真实注册到 Consul
  - 再用 `gateway/rpc/user` 的 Consul 模式 client 去发现并调用

这意味着当前验收测试不是纯 mock，也不是重型全环境 e2e，而是介于两者之间的“最小整链路闭环”。

#### 3.4.5 执行策略

当前执行策略分两类：

- 日常可直接跑的验收测试
  - `gateway_acceptance_test.go`
- 依赖外部环境的验收测试
  - `consul_acceptance_test.go`
  - 当本地 Consul 不可用时自动跳过，不阻塞日常 `go test`

这样做的原因：

- 保证大多数验收测试可以稳定进入日常回归
- 保留真实 Consul 闭环验证能力
- 避免把所有开发机都变成“必须先起完整环境才能测试”

### 3.5 读链路重试治理

读链路有限重试已经完成当前阶段落地，当前采用“只给幂等读接口加一次有限重试，写接口保持不自动重放”的策略。

治理目标：

- 吸收瞬时网络抖动和短暂连接异常
- 降低读请求因为单次 transport error 直接失败的概率
- 不让写链路因为透明重放而放大状态一致性风险

当前已完成：

- 新增共享读 RPC 重试策略构造：
  - `app/common/rpc_retry.go`
- `gateway` 下游读接口已启用一次有限重试：
  - `gateway/rpc/user/client.go`
    - `GetUser`
  - `gateway/rpc/product/client.go`
    - `GetProductDetail`
    - `ListProducts`
    - `BatchGetSKU`
  - `gateway/rpc/inventory/client.go`
    - `GetSkuStock`
    - `BatchGetSkuStock`
    - `CheckSaleableStock`
  - `gateway/rpc/cart/client.go`
    - `GetCart`
  - `gateway/rpc/order/client.go`
    - `GetOrder`
    - `ListOrders`
- `order-service` 下游商品读接口已启用一次有限重试：
  - `services/order-service/rpcclient/product/client.go`
    - `GetProductDetail`
    - `BatchGetSKU`
- 混合读写的 RPC wrapper 已拆成 `readCli/writeCli` 双 client，避免把写接口一起带上重试

当前策略：

- 最大重试次数：`1`
- 固定退避：`30ms`
- 仅对技术错误重试：transport error、连接失败、临时网络错误
- 不对业务错误码重试
- 写接口仍然依赖业务幂等和显式补偿，不走 client 透明重放

设计落点：

- `app/common/rpc_retry.go`
- `gateway/rpc/user/client.go`
- `gateway/rpc/product/client.go`
- `gateway/rpc/inventory/client.go`
- `gateway/rpc/cart/client.go`
- `gateway/rpc/order/client.go`
- `services/order-service/rpcclient/product/client.go`

测试覆盖：

- `app/common/rpc_retry_test.go`
  - 覆盖策略参数
- `gateway/rpc/product/client_test.go`
  - 覆盖读接口在临时 transport error 下的自动重试成功
- `gateway/rpc/order/client_test.go`
- `gateway/rpc/inventory/client_test.go`
  - 覆盖读写分离后的 wrapper 映射行为仍然正确

## 4. 已完成的服务生命周期治理

### 4.1 服务生命周期治理

这项能力已经完成当前阶段的最小治理基线建设，并已进入可复用状态。

当前第一阶段已经完成：

- `gateway` 已提供 `/healthz`、`/readyz`
- `user-service` / `product-service` 已在 admin 端口统一暴露 `/metrics`、`/healthz`、`/readyz`
- `gateway`、`user-service`、`product-service` 已接入基于信号的优雅停机
- 宿主机启动脚本已统一补充 shutdown timeout 与探针入口提示

当前第二阶段已完成的部分：

- `gateway` 在 Consul 发现模式下，启动前会先检查 `CONSUL_ADDR` TCP 连通性
- `user-service` / `product-service` 启动前会先检查 MySQL TCP 连通性
- `user-service` / `product-service` 在 Consul 注册模式下，还会先检查 `CONSUL_ADDR` TCP 连通性
- 三个主服务都已提供统一的 preflight timeout 配置
- 启动失败时会直接给出具体依赖和目标地址，而不是进入半启动状态后再报含糊错误

当前第三阶段已完成的部分：

- `gateway`、`user-service`、`product-service` 在停机时会先进入 draining
- 三个主服务都已增加 drain timeout 配置
- draining 期间 `readyz` 会先失败，再进入真正的 shutdown / stop
- 已补 Consul 验收测试，验证 `server.Stop()` 后实例会从健康服务列表中消失

生命周期治理当前主要包含四块：

1. 探针与基础可观测入口
   - `gateway` 主端口提供 `/healthz`、`/readyz`
   - RPC 服务在 admin 端口统一暴露 `/metrics`、`/healthz`、`/readyz`
2. 优雅停机
   - 服务收到 `SIGINT` / `SIGTERM` 后不直接退出
   - 先进入 draining，再执行 shutdown / stop
3. 启动前自检
   - `gateway` 在 Consul 发现模式下检查 `CONSUL_ADDR`
   - `user-service` / `product-service` 检查 MySQL 连通性，使用 Consul 注册时再检查 `CONSUL_ADDR`
4. 模板化复用
   - 新服务最小治理模板已沉淀到 [service-development-spec.md](./service-development-spec.md)

当前统一设计意图：

- 用 `/healthz` 判断“进程是否存活”
- 用 `/readyz` 判断“实例当前是否仍适合接新流量”
- 用 preflight 尽早暴露远程依赖问题
- 用 draining + drain timeout 明确“先摘流，再停服务”的顺序

当前边界：

- 现在的 readiness 判断还比较保守，主要覆盖进程状态、draining 状态和关键依赖可用性
- 当前摘流仍以 `readyz` 和显式 drain timeout 为主，还没有更复杂的发布编排
- 当前阶段已经足够支撑宿主机调试和最小多服务联调

说明：

- 本文档只保留治理目标、阶段结论和边界
- 新服务如何接入这些能力，统一见 [service-development-spec.md](./service-development-spec.md)
- 具体启动、检查和排障步骤，统一见 [runbook.md](./runbook.md)

## 5. 当前可以先不做的

以下能力不是没价值，而是当前阶段收益还不够高：

- 业务服务 Compose 化
- MySQL / Redis 本地容器化
- 熔断
- 复杂降级编排
- 隔离舱壁
- 动态路由
- 灰度发布
- 服务网格
- 分布式限流
- 自定义负载均衡策略

当前不建议优先做这些能力的原因：

- 当前本机资源和开发效率约束都不支持把部署形态治理放在最前面
- 远程 MySQL / Redis 已经能满足当前联调，不是最先卡开发效率的瓶颈
- 服务数量和调用深度还不够大
- 真实故障样本还不够多
- 过早引入会明显增加配置、排障和理解成本
- 很多策略只有在订单、库存等链路真正接入后，才知道该怎么调才合理

## 6. 以后再做的治理能力

在当前服务生命周期治理完成后，后续更适合放到“以后再做”的治理项包括：

- Redis 接入主链路后的 preflight 收口
- 更细粒度的 readiness 判定
- 更明确的依赖级别错误分类
- 更复杂的摘流与发布编排
- 新服务自动化脚手架

### 6.1 熔断与快速失败

当下游持续超时或错误率升高时，再补这项能力更合适。

适用场景：

- 某个服务持续不可用
- 某个依赖抖动导致上游请求大量堆积
- 需要快速失败，阻止故障级联

前提：

- 已经有较稳定的 timeout 和指标口径
- 已经知道哪些接口属于关键路径

### 6.2 服务隔离

后续如果引入订单、库存、支付等服务，隔离的重要性会明显上升。

可考虑的方向：

- 不同下游服务连接池隔离
- 慢接口与热点接口隔离
- 后台任务与在线流量隔离
- 核心链路与非核心链路隔离

### 6.3 更细粒度的限流

当前先做网关入口限流即可，后续再考虑：

- 分布式限流
- 热点接口专项限流
- 下游服务自保护限流
- 按租户、角色、业务动作维度限流

### 6.4 路由与发布治理

当服务实例数增多、版本并存需求出现时，再推进：

- 灰度发布
- 动态路由
- 按用户/请求头/租户路由
- 权重路由
- locality 路由

## 7. 与下一步业务建设的关系

当前建议不是“暂停业务，先把治理全做完”，而是：

1. 先基于当前模板推进下一个核心业务模块
2. 再用真实业务链路验证后续治理需求

这样做的原因：

- 当前治理缺口已经足够影响后续开发效率
- 当前服务生命周期治理已经形成可复用基线
- 这时继续优先做治理的收益开始下降
- 更适合切回业务开发，再让真实链路暴露下一批治理问题

## 8. 推荐推进顺序

建议按以下顺序推进：

1. 基于当前模板推进下一个核心业务模块
2. 补测试与最小 CI 门槛
3. 根据真实调用链问题，再决定是否补重试、熔断、隔离
4. 项目基本完成后，再推进业务服务容器化与 Compose 化

## 9. 对下一步业务的建议

如果要在“继续治理”与“继续业务”之间取平衡，我建议：

- 当前先切回业务推进
- 新服务默认按现有治理模板接入

原因：

- 当前项目已经完成服务生命周期治理的最小闭环
- 更合适的是让后续业务服务直接复用这套模板
- 再优先选择 `cart` 或 `inventory` 这类更容易复用现有链路的业务服务
- 订单与支付更适合放到治理能力和服务模板更稳定之后推进

换句话说，当前阶段最合理的路线不是“继续堆治理”，而是“先用已完成的治理基线去推进业务，再让真实业务反哺后续治理”。

## 10. 当前结论

已完成的：

- 超时治理第一阶段

当前应该先做的：

- 基于现有治理模板推进下一个业务模块
- 测试与最小 CI 门槛

当前暂时不急着做的：

- 业务服务 Compose 化
- MySQL / Redis 本地容器化
- 熔断
- 全链路重试
- 复杂降级
- 灰度发布
- 动态路由
- 服务网格

当前整体策略：

- 不停下业务开发
- 不提前做重平台治理
- 先补宿主机调试优先的最小治理基线
- 再用下一阶段核心业务继续验证治理方向
