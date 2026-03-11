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

- 调用链超时策略还没有显式收口
- 限流还没有落地
- 故障时的排障入口还不够清晰
- 服务虽然已经拆开，但治理能力还主要停留在“能注册、能调用”

因此，当前更适合建设“最小治理基线”，而不是一次性引入完整治理体系。

## 3. 当前应该先做的

### 3.1 超时治理

这是当前优先级最高的一项。

建议先统一三层 timeout：

- HTTP 请求超时
- `gateway -> rpc` 调用超时
- 服务内数据库访问超时

目标：

- 避免请求无限等待
- 避免上游超时预算和下游超时预算互相冲突
- 避免链路卡死后只能靠日志猜问题

当前阶段建议：

- 先把超时配置显式化，不要继续依赖框架默认值
- 先由配置统一管理 timeout，而不是散落在业务代码里
- 读请求和写请求可以先使用不同预算，但不需要一开始就做得很细

当前已落地：

- `gateway -> user-service` 与 `gateway -> product-service` 的 Kitex Client 已显式配置 connect timeout 和 rpc timeout
- `user-service` 与 `product-service` 的 repository 已统一套上数据库查询超时
- timeout 已经进入配置层，而不是继续写死在调用点

当前设计落点：

- `gateway/config/config.go`
  - `USER_RPC_CONNECT_TIMEOUT_MS`
  - `USER_RPC_TIMEOUT_MS`
  - `PRODUCT_RPC_CONNECT_TIMEOUT_MS`
  - `PRODUCT_RPC_TIMEOUT_MS`
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

当前预算口径：

- RPC connect timeout：默认 `500ms`
- RPC timeout：默认 `2000ms`
- DB query timeout：默认 `1500ms`

这样设计的目的：

- 把 `gateway -> rpc -> db` 这条链路的超时预算先明确下来
- 保证 DB timeout 小于整体 RPC timeout，避免下游数据库还在等待时，上游先完全失控
- 先把最容易出问题的链路显式收口，再决定后续是否继续细分读写预算

### 3.1.1 如何观测超时

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

当前观测重点：

- 是否是 connect timeout
- 是否是 rpc timeout
- 是否是 service 内部 DB query timeout
- 是单点偶发，还是持续性故障

一个实际注意点：

- `ConnectTimeout` 在 Kitex 下可能会叠加其默认重试策略，因此最终总耗时不一定严格等于单次 connect timeout
- 观察时要区分“单次建连超时”和“整体调用在重试后失败”这两个层次

### 3.1.2 超时后如何返回

当前超时不是直接把底层错误原文返回给前端，而是先在 `gateway` 做技术错误映射。

当前映射规则：

- timeout / deadline exceeded
  - 对外返回：`服务繁忙，请稍后重试`
- connection refused / no available / dial tcp / broken pipe 等连接阶段错误
  - 对外返回：`下游服务暂不可用，请稍后重试`
- 其他技术错误
  - 对外返回：`系统内部错误`

当前实现位置：

- `gateway/internal/logic/logicutil/rpc_error.go`
- `app/common/errors.go`

这样做的原因：

- 避免把底层网络错误和内部实现细节直接暴露给前端
- 让前端拿到更稳定、可理解的错误口径
- 给后续更细的技术错误分类预留空间

### 3.1.3 当前怎么测试

当前围绕超时治理已经补了三层测试。

第一层：真实 RPC timeout 行为测试

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

第二层：真实 DB timeout 行为测试

- `services/user-service/biz/repository/repository_test.go`
- `services/product-service/biz/repository/repository_test.go`

测试方式：

- 使用内存 sqlite 启动 GORM
- 在 query callback 中故意阻塞到 `ctx.Done()`
- 调用真实 repository 方法
- 断言最终返回 `context deadline exceeded`

第三层：错误映射测试

- `gateway/internal/logic/logicutil/rpc_error_test.go`
- `gateway/internal/logic/user/login_test.go`
- `gateway/internal/logic/user/register_test.go`

覆盖内容：

- timeout 是否会映射成“服务繁忙，请稍后重试”
- 连接失败是否会映射成“下游服务暂不可用，请稍后重试”
- `user` 业务链路是否真正使用了这套映射

当前测试结论：

- `RPCTimeout`：已有真实行为测试
- `ConnectTimeout`：已有真实行为测试
- repository DB timeout：已有真实行为测试
- 超时后的 gateway 对外返回：已有映射测试

这意味着当前文档里“已落地”的 timeout 能力，已经具备基本可回归的测试闭环。

当前暂未落地：

- `gateway` HTTP 入口超时还没有在 Hertz 启动层显式配置
- 这部分先保留为下一步补齐项，避免在不确定框架接入方式时引入额外噪音

### 3.2 网关入口限流

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

### 3.3 运行与排障手册

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

### 3.4 最小验收测试

严格来说这属于稳定性建设，但应该和治理基线一起推进。

建议至少补齐：

- Consul 注册发现的最小验收测试
- `gateway -> user-service` 的最小联调测试
- `gateway -> product-service` 的最小联调测试
- 超时和异常响应的关键路径测试

目标不是一次性把测试做满，而是保证治理相关能力不是“只写了配置，没有回归验证”。

## 4. 当前可以先不做的

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

## 5. 以后再做的治理能力

### 5.1 重试策略

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

### 5.2 熔断与快速失败

当下游持续超时或错误率升高时，再补这项能力更合适。

适用场景：

- 某个服务持续不可用
- 某个依赖抖动导致上游请求大量堆积
- 需要快速失败，阻止故障级联

前提：

- 已经有较稳定的 timeout 和指标口径
- 已经知道哪些接口属于关键路径

### 5.3 服务隔离

后续如果引入订单、库存、支付等服务，隔离的重要性会明显上升。

可考虑的方向：

- 不同下游服务连接池隔离
- 慢接口与热点接口隔离
- 后台任务与在线流量隔离
- 核心链路与非核心链路隔离

### 5.4 更细粒度的限流

当前先做网关入口限流即可，后续再考虑：

- 分布式限流
- 热点接口专项限流
- 下游服务自保护限流
- 按租户、角色、业务动作维度限流

### 5.5 路由与发布治理

当服务实例数增多、版本并存需求出现时，再推进：

- 灰度发布
- 动态路由
- 按用户/请求头/租户路由
- 权重路由
- locality 路由

## 6. 与下一步业务建设的关系

当前建议不是“暂停业务，先把治理全做完”，而是：

1. 先补最小治理基线
2. 再继续推进下一个核心业务模块

这样做的原因：

- 当前治理缺口已经足够影响后续开发效率
- 但又没有大到需要停下所有业务开发先做平台化改造
- 先补最小护栏，再加业务，性价比最高

## 7. 推荐推进顺序

建议按以下顺序推进：

1. 明确 timeout 策略并落配置
2. 增加网关基础限流
3. 补全运行与排障手册
4. 增加最小联调与治理验收测试
5. 继续推进下一个核心业务模块
6. 根据真实调用链问题，再决定是否补重试、熔断、隔离

## 8. 对下一步业务的建议

如果要在“继续治理”与“继续业务”之间取平衡，我建议：

- 当前先做最小治理基线
- 然后优先推进 `order` 相关业务

原因：

- 订单链路会真正把商品、用户、库存等服务串起来
- 到那时才会暴露出更真实的治理需求
- 比如哪些链路需要超时更短，哪些地方需要隔离，哪些接口能不能重试

换句话说，当前阶段最合理的路线不是“先做完整治理”，而是“先做治理底线，再用订单链路验证治理需求”。

## 9. 当前结论

当前应该先做的：

- timeout 显式配置
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
