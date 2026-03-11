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

- 限流还没有落地
- 故障时的排障入口还不够清晰
- 服务虽然已经拆开，但治理能力还主要停留在“能注册、能调用”
- 超时治理虽然已经补齐，但仍需持续用业务链路验证预算是否合理

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

## 4. 当前应该先做的

### 4.1 网关入口限流

限流是当前最值得尽快落地的第二项能力。

原因：

- `gateway` 是统一对外入口
- 登录、商品管理等接口已经具备被滥用或打爆的基础条件
- 相比在每个服务分别补限流，网关限流的收益更直接

建议先做：

- 按 IP 限流
- 按用户限流
- 按接口限流

建议先保护的接口：

- `POST /api/v1/user/login`
- `POST /api/v1/user/register`
- 商品管理写接口

当前阶段不必一开始就做分布式限流，本地令牌桶或固定窗口已经足够。

### 4.2 运行与排障手册

这项能力看起来不像“治理代码”，但实际上是当前最缺的治理基础设施之一。

建议补齐：

- 本地启动顺序
- Consul 注册与发现排查
- RPC 调用失败排查
- migration 失败排查
- metrics / trace / log 的查看入口
- 常见超时与连接失败问题定位方式

原因：

- 当前服务已经不是单体结构
- 没有统一 runbook 时，故障定位成本会明显上升
- 后面继续加服务时，排障复杂度会继续放大

### 4.3 最小验收测试

严格来说这属于稳定性建设，但应该和治理基线一起推进。

建议至少补齐：

- Consul 注册发现的最小验收测试
- `gateway -> user-service` 的最小联调测试
- `gateway -> product-service` 的最小联调测试
- 超时和异常响应的关键路径测试

目标不是一次性把测试做满，而是保证治理相关能力不是“只写了配置，没有回归验证”。

## 5. 当前可以先不做的

以下能力不是没价值，而是当前阶段收益还不够高：

- 全链路默认重试
- 熔断
- 复杂降级编排
- 隔离舱壁
- 动态路由
- 灰度发布
- 服务网格
- 分布式限流
- 自定义负载均衡策略

当前不建议优先做这些能力的原因：

- 服务数量和调用深度还不够大
- 真实故障样本还不够多
- 过早引入会明显增加配置、排障和理解成本
- 很多策略只有在订单、库存等链路真正接入后，才知道该怎么调才合理

## 6. 以后再做的治理能力

### 6.1 重试策略

后续可以建设，但要有边界。

建议原则：

- 只对幂等读请求开放重试
- 只对明确的技术错误做有限次重试
- 重试必须和 timeout 一起设计

当前不建议对以下操作默认重试：

- 登录
- 创建商品
- 更新商品
- 修改商品状态
- 后续订单创建、支付等写操作

### 6.2 熔断与快速失败

当下游持续超时或错误率升高时，再补这项能力更合适。

适用场景：

- 某个服务持续不可用
- 某个依赖抖动导致上游请求大量堆积
- 需要快速失败，阻止故障级联

前提：

- 已经有较稳定的 timeout 和指标口径
- 已经知道哪些接口属于关键路径

### 6.3 服务隔离

后续如果引入订单、库存、支付等服务，隔离的重要性会明显上升。

可考虑的方向：

- 不同下游服务连接池隔离
- 慢接口与热点接口隔离
- 后台任务与在线流量隔离
- 核心链路与非核心链路隔离

### 6.4 更细粒度的限流

当前先做网关入口限流即可，后续再考虑：

- 分布式限流
- 热点接口专项限流
- 下游服务自保护限流
- 按租户、角色、业务动作维度限流

### 6.5 路由与发布治理

当服务实例数增多、版本并存需求出现时，再推进：

- 灰度发布
- 动态路由
- 按用户/请求头/租户路由
- 权重路由
- locality 路由

## 7. 与下一步业务建设的关系

当前建议不是“暂停业务，先把治理全做完”，而是：

1. 先补最小治理基线
2. 再继续推进下一个核心业务模块

这样做的原因：

- 当前治理缺口已经足够影响后续开发效率
- 但又没有大到需要停下所有业务开发先做平台化改造
- 先补最小护栏，再加业务，性价比最高

## 8. 推荐推进顺序

建议按以下顺序推进：

1. 增加网关基础限流
2. 补全运行与排障手册
3. 增加最小联调与治理验收测试
4. 继续推进下一个核心业务模块
5. 根据真实调用链问题，再决定是否补重试、熔断、隔离

## 9. 对下一步业务的建议

如果要在“继续治理”与“继续业务”之间取平衡，我建议：

- 当前先做最小治理基线
- 然后优先推进 `order` 相关业务

原因：

- 订单链路会真正把商品、用户、库存等服务串起来
- 到那时才会暴露出更真实的治理需求
- 比如哪些链路需要超时更短，哪些地方需要隔离，哪些接口能不能重试

换句话说，当前阶段最合理的路线不是“先做完整治理”，而是“先做治理底线，再用订单链路验证治理需求”。

## 10. 当前结论

已完成的：

- 超时治理第一阶段

当前应该先做的：

- 网关入口限流
- 运行与排障手册
- 最小验收测试

当前暂时不急着做的：

- 熔断
- 全链路重试
- 复杂降级
- 灰度发布
- 动态路由
- 服务网格

当前整体策略：

- 不停下业务开发
- 不提前做重平台治理
- 先补最小治理基线
- 再用下一阶段核心业务继续验证治理方向
