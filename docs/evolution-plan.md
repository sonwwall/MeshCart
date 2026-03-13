# 后续演进规划

## 1. 目的

本文档基于当前仓库已经落地的能力，更新 MeshCart 下一阶段值得继续推进的演进方向、优先级和边界。

这份文档不再重复记录“还没开始的设想”，而是先明确当前基线，再规划真正剩余的工作。

## 2. 当前基线

截至当前代码状态，项目已经完成的基础能力包括：

- 架构基线
  - `gateway + user-service + product-service + cart-service` 已形成下一阶段业务主链路
  - `order-service`、`inventory-service`、`payment-service` 已有目录骨架，但仍未进入完整联调阶段
- 服务发现
  - `gateway -> user-service`
  - `gateway -> cart-service`
  - `gateway -> product-service`
  - 已支持 `Consul` 发现与 `direct` 直连回退
  - 已补充最小 Consul 验收测试
- 认证与授权
  - 网关已接入 JWT 登录态
  - 已提供 `refresh_token` 刷新接口，但仍属于单 JWT 刷新窗口方案，不是服务端会话制双 token
  - 已落地 Casbin 鉴权
  - 已支持 `guest / user / admin / superadmin`
  - 已支持商品归属控制和用户角色治理
- 流量治理与稳定性
  - 网关已接入请求总超时
  - 网关已接入第一阶段限流
  - 已区分全局、登录、注册、管理写接口等限流规则
  - RPC 客户端已显式配置 timeout，并在测试中约束默认不盲目重试
- 服务生命周期
  - `gateway` 已提供 `/healthz`、`/readyz`
  - `user-service`、`product-service` 已在 admin 端口暴露 `/metrics`、`/healthz`、`/readyz`
  - `gateway`、`user-service`、`product-service` 已接入信号驱动的优雅停机
  - 已补远程依赖场景下的启动前 TCP 连通性自检
  - 已补 draining 窗口与停机前 `readyz` 摘流顺序
  - 已补 Consul 停机后实例摘注册验收验证
- 可观测性
  - 日志：Zap 结构化日志
  - 指标：Prometheus
  - 链路：OpenTelemetry + Jaeger
  - 日志采集：Promtail + Loki
  - 看板：Grafana 预置 dashboard
- 运行与文档
  - `runbook` 已补齐本地启动与排障步骤
  - 已补齐网关、用户、商品、Consul、可观测性等专题设计文档
- 测试
  - 已有 gateway 逻辑单测
  - 已有 user-service / product-service 仓储与服务层单测
  - 已补 cart-service service 层与 gateway cart logic 单测
  - 已有 gateway 验收测试
  - 已有 Consul 服务发现验收测试

结论：

- 当前阶段已经不是“只有用户登录骨架”的状态
- 下一阶段重点应从“补最基础能力”转向“把现有能力做完整、可持续、可交付”

## 3. 演进原则

后续规划建议遵循以下原则：

1. 先补齐可持续交付能力，再继续横向铺业务服务
2. 先把已经接入的能力做实，再引入更复杂的治理组件
3. 设计优先围绕真实链路问题，而不是为了“技术名词覆盖率”堆功能
4. 每新增一类基础设施能力，都应同步补配置、文档、测试和排障路径

## 4. 优先级建议

建议按以下顺序推进：

1. 服务生命周期治理继续收口
2. 认证体系从单 JWT 刷新升级为服务端会话制
3. 稳定性治理第二阶段
4. 测试体系与 CI 能力
5. 配置、安全与发布治理
6. 新业务服务实装与统一治理复用
7. 项目基本完成后，再推进业务服务容器化与 Compose 化

补充判断：

- `cart-service` 已经可以作为“治理模板复用”的第一条业务线继续完善
- 下一条更合适的业务线仍然是 `inventory-service`，而不是直接进入 `order-service / payment-service`

## 5. P0：服务生命周期治理收口

### 5.1 当前状态

当前第一阶段已经落地：

- `gateway` 主端口提供 `/healthz`、`/readyz`
- `user-service` / `product-service` 在 admin 端口统一暴露 `/metrics`、`/healthz`、`/readyz`
- 三个主服务都支持信号驱动的优雅停机
- 宿主机启动脚本已统一补充 shutdown timeout 和探针入口提示
- `gateway`、`user-service`、`product-service` 已补 preflight timeout 与远程依赖 TCP 连通性自检

但还没有完全收口的部分包括：

- 新服务复用同一套生命周期模板
- 更细的 readiness 判定标准
- Redis 接入主链路后的 preflight 收口

### 5.2 下一步建议

优先补齐：

- 宿主机启动契约的继续统一
- 远程依赖检查与排障闭环的继续细化
- 服务停机流程的 runbook 固化
- 新服务复用同一套健康检查与 shutdown 模板

建议目标：

- 保持宿主机直跑服务的调试体验
- 让新服务默认按同一套生命周期契约接入
- 让“启动成功、可接流、停止接流、优雅退出”都有统一判断口径

### 5.3 完成标准

- 启动、健康、就绪、停机入口在三个主服务上保持一致
- runbook 能覆盖远程依赖、探针、优雅停机的排障步骤
- 后续新服务不再各自实现一套生命周期逻辑

## 6. P1：认证体系升级

### 6.1 当前状态

当前认证能力已经比旧规划更完整：

- 登录由 `gateway` 签发 JWT
- 已支持 `GET /api/v1/user/refresh_token`
- JWT 中已包含 `user_id`、`username`、`role` 等身份快照

但当前仍然属于单 token 演进方案，存在以下边界：

- 无法真正做到服务端失效控制
- 不支持 `logout` / `logout_all`
- 角色变更后旧 token 仍依赖重新登录或刷新来收敛
- 不支持设备维度、多端会话治理

### 6.2 下一步建议

当项目开始进入多人联调、后台治理或更长期会话管理阶段时，建议升级为双 token：

- `access_token`
  - 继续使用 JWT
  - 有效期缩短到 `15` 到 `30` 分钟
- `refresh_token`
  - 使用高熵随机串
  - 服务端存储白名单或会话表
  - 每次刷新执行 rotation

推荐先补：

- Redis 会话白名单
- `POST /api/v1/user/refresh_token`
- `POST /api/v1/user/logout`
- `POST /api/v1/user/logout_all`
- `session_id` / `device_id` 字段模型

### 6.3 配套事项

认证升级不要只改接口，还需要同时补齐：

- token 失效策略
- 角色变更后的会话收敛策略
- Redis 故障时的降级行为
- 安全审计与操作日志
- runbook 中的排障步骤

## 7. P1：稳定性治理第二阶段

### 7.1 已完成部分

当前已经具备：

- HTTP 层请求超时
- RPC timeout
- 网关第一阶段限流
- 统一错误码与基础 trace/metrics

因此下一阶段不需要重复讨论“是否要做超时、限流”，而是应该继续做第二阶段治理。

### 7.2 超时治理

后续建议继续统一超时预算：

- HTTP 总超时
- Gateway -> RPC 超时
- DB / Redis 访问超时
- 后续消息队列投递或消费超时

重点不是单独配置一个值，而是形成入口到下游的预算分配规则。

### 7.3 限流治理

当前限流还是单机内存态，下一步建议按顺序推进：

1. 补齐限流命中指标与日志口径
2. 区分匿名用户、登录用户、管理写接口的治理阈值
3. 视业务增长再评估 Redis 分布式限流

当前不建议过早做复杂分布式限流，除非已经出现多实例网关下的真实绕限问题。

### 7.4 重试、熔断、隔离

当前仓库还没有真正落地以下能力：

- 熔断
- 失败快速返回
- 并发隔离 / 舱壁
- 明确的降级策略

建议演进顺序：

1. 先梳理哪些 RPC 是读操作、哪些错误允许有限重试
2. 再给关键下游引入熔断与失败快速返回
3. 最后再做热点接口隔离、慢接口隔离、核心链路隔离

原则：

- 登录、注册、角色变更、商品写操作默认不要盲目重试
- 所有治理动作都要有指标可观测
- 不要引入“高重试 + 短超时”这类会放大故障的组合

## 8. P1：测试体系与 CI

### 8.1 当前状态

当前测试基础已经存在，但还不够覆盖“变更可回归”：

- 已有部分单测
- 已有 gateway 验收测试
- 已有 Consul 验收测试

缺口仍然明显存在于：

- JWT / 刷新 / 角色快照相关链路
- Casbin 授权回归
- 商品管理写链路
- migration 回归
- 多服务联调回归

### 8.2 下一步建议

建议把测试建设分成三层：

1. 单测
   - `gateway/internal/logic`
   - `gateway/internal/authz`
   - `user-service` / `product-service` service 与 repository
2. 集成测试
   - MySQL migration
   - JWT 登录、刷新、角色变更
   - 商品创建、更新、上下架、权限拦截
3. 验收测试
   - `gateway + user-service + product-service + consul`

### 8.3 CI 建议

当前仓库里还没有形成明确的 CI 入口约束，建议补齐：

- `go test` 分层执行策略
- 需要依赖外部环境的测试标签与跳过规则
- 文档化的推荐校验命令
- PR 最低通过门槛

## 9. P2：配置、安全与发布治理

### 9.1 配置治理

当前配置仍偏开发态，后续建议继续完善：

- 明确环境变量与配置文件优先级
- 区分开发、测试、生产配置口径
- 把敏感配置从默认值演进为显式注入
- 为 Redis、MySQL、Consul、OTel 等依赖定义统一配置约定

### 9.2 安全治理

建议优先补齐：

- 生产环境禁止弱 `JWT_SECRET`
- 默认账号密码与本地开发配置的边界说明
- 管理操作审计日志
- 角色变更审计
- 关键接口安全基线检查

### 9.3 发布治理

当前还没有成型的灰度或动态路由能力。

后续如果服务数量和实例数继续增长，可在网关层逐步演进：

- 按 Header / 用户 / 测试账号灰度
- 按版本路由
- 指标驱动回滚

这部分优先级低于会话治理、容器化和测试体系。

## 10. P2：新业务服务实装

### 10.1 当前状态

仓库已经存在：

- `order-service`
- `inventory-service`
- `payment-service`
- `cart-service`

但当前真正完成可联调、可治理复用的仍主要是：

- `user-service`
- `product-service`

### 10.2 下一步建议

后续继续扩业务服务时，不建议再走“先把接口写出来再补治理”的路线，而应复用现有基线：

- 统一服务名规范
- 统一启动脚本与配置约定
- 统一 metrics / trace / log
- 统一 timeout / 错误码 / RPC 封装
- 统一 migration、repository、service 分层规范

推荐优先级：

1. `cart-service`
2. `inventory-service`
3. `order-service`
4. `payment-service`

原因：

- 购物车和库存更容易复用现有商品与用户链路
- 订单与支付对事务性、幂等性和状态机要求更高，应放在治理能力更成熟后推进

## 11. 当前建议汇总

短期建议先做：

- 补齐业务服务与 MySQL 的 Compose 化
- 统一本地默认启动方式
- 设计并落地 Redis 会话白名单
- 把 JWT 从单 token 刷新窗口升级到双 token
- 补齐角色治理、商品管理相关回归测试

中期按需推进：

- 熔断 / 快速失败 / 隔离
- 分布式限流
- 配置与密钥治理
- CI 门禁
- 新业务服务接入同一套治理基线

长期再考虑：

- 灰度发布
- 动态路由
- 更复杂的多角色、多资源授权模型

## 12. 待确认事项

以下事项建议后续拍板：

- 是否把 MySQL 纳入默认 `docker compose` 开发环境
- 是否优先引入 Redis，为双 token 与分布式限流做准备
- JWT 默认有效期是否从 `120` 分钟下调
- 下一阶段首先实装哪个业务服务
- 是否需要把 CI 门禁纳入当前迭代目标
