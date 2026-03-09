# Consul 服务发现设计

## 1. 目标

为 `gateway -> user-service` 的 Kitex RPC 调用引入基于 Consul 的服务发现，替代固定 `host:port` 直连。

本次设计目标：

- 默认通过 Consul 做服务发现
- 保留 `direct` 直连回退能力，便于排障
- 在当前仓库内只落地最小可用链路，不引入配置中心职责

## 2. 范围

本次覆盖：

- `gateway` 作为服务消费者，通过 Consul 查找 `user-service`
- `user-service` 启动时注册到 Consul
- `docker-compose.yml` 提供本地开发用单节点 Consul

本次不覆盖：

- Consul KV 配置中心
- ACL、TLS、多数据中心
- 其它业务服务的批量迁移

## 3. 改造前现状

改造前：

- `gateway` 通过 `USER_RPC_ADDR` 和 `client.WithHostPorts(...)` 直连 `user-service`
- `user-service` 未接入注册中心
- 服务名存在多套命名：`UserService`、`user-service`、`meshcart.user`

这会带来两个问题：

- 实例扩缩容时需要手工改网关配置
- 服务名不统一，接入注册中心后容易出现发现失败

## 4. 设计决策

统一使用 `meshcart.user` 作为 RPC 服务名。

发现模式保留双态：

- `consul`：默认模式，使用 Consul resolver 发现实例
- `direct`：回退模式，使用固定 `host:port`

这样做的原因：

- 日常开发和联调默认走真实服务发现链路
- 出现注册中心问题时，仍然可以快速切回直连定位问题

## 5. 库选型与使用方式

当前项目没有直接使用官方 `github.com/hashicorp/consul` 仓库里的完整实现，而是分两层使用：

- `github.com/hashicorp/consul/api`
  - 这是 Consul 官方提供的 Go API 客户端
  - 负责和 Consul Agent/HTTP API 通信
  - 当前项目里只在自定义健康检查配置时直接使用了它的 `AgentServiceCheck`
- `github.com/kitex-contrib/registry-consul`
  - 这是 Kitex 生态的 Consul 适配层
  - 内部基于官方 `consul/api`
  - 对外实现了 Kitex 所需的 `registry.Registry` 和 `resolver.Resolver`

在本项目中的用法：

- `gateway` 使用 `consul.NewConsulResolver(...)` 创建 resolver，并通过 `client.WithResolver(...)` 挂到 Kitex client 上
- `user-service` 使用 `consul.NewConsulRegister(...)` 创建 registry，并通过 `server.WithRegistry(...)` 挂到 Kitex server 上
- `user-service` 额外通过 `consulapi.AgentServiceCheck` 定制了 Consul 健康检查

对应代码：

- [gateway/rpc/user/client.go](/Users/ruitong/GolandProjects/MeshCart/gateway/rpc/user/client.go)
- [services/user-service/rpc/main.go](/Users/ruitong/GolandProjects/MeshCart/services/user-service/rpc/main.go)

## 6. 运行时设计

### 6.1 Gateway

`gateway` 相关配置：

- `USER_RPC_SERVICE`
- `USER_RPC_DISCOVERY`
- `USER_RPC_ADDR`
- `CONSUL_ADDR`

行为：

- 当 `USER_RPC_DISCOVERY=consul` 时，`gateway` 使用 `meshcart.user` 从 Consul 获取实例列表
- 当 `USER_RPC_DISCOVERY=direct` 时，`gateway` 使用 `USER_RPC_ADDR` 直连

### 6.2 User Service

`user-service` 相关配置：

- `USER_RPC_SERVICE`
- `USER_SERVICE_ADDR`
- `USER_SERVICE_REGISTRY`
- `CONSUL_ADDR`
- `USER_SERVICE_CONSUL_TCP_CHECK`

行为：

- 启动时显式绑定 `USER_SERVICE_ADDR`
- 当 `USER_SERVICE_REGISTRY=consul` 时，将自身注册到 Consul
- 默认使用 `TTL` 心跳健康检查
- 当 `USER_SERVICE_CONSUL_TCP_CHECK=true` 时，改用 Consul `TCP` 健康检查

### 6.3 Docker Compose

`docker-compose.yml` 中新增了单节点 `consul`：

- UI 地址：`http://localhost:8500`
- 使用命名卷 `consul_data` 持久化数据
- 仅面向本地开发和测试

## 7. 健康检查设计

这是本次落地里最容易踩坑的地方。

### 7.1 默认方案

当前默认使用 `TTL` 健康检查，而不是 `TCP` 检查。

原因：

- 你的当前运行方式是 `consul` 在 Docker 容器里，`user-service` 在宿主机
- 如果把 `127.0.0.1:8888` 注册给 Consul，并启用 `TCP` 检查，Consul 容器会去探测它自己的回环地址，而不是宿主机上的 `user-service`
- 这会导致实例一直是 `critical`，`gateway` 拿不到健康实例

`TTL` 模式下，由 `registry-consul` 在服务注册后自动上报心跳，避免了容器对宿主机回环地址的误探测。

### 7.2 什么时候可以开启 TCP 检查

以下条件满足时，可以把 `USER_SERVICE_CONSUL_TCP_CHECK=true`：

- `user-service` 也运行在容器网络里
- 注册到 Consul 的地址不是宿主机 `127.0.0.1`
- Consul 可以从自身网络命名空间访问到这个服务地址

例如：

```bash
export USER_SERVICE_ADDR=user-service:8888
export USER_SERVICE_CONSUL_TCP_CHECK=true
```

### 7.3 三方库注意事项

`github.com/kitex-contrib/registry-consul@v0.2.0` 有一个实现缺陷：

- 它的注释写着可以通过 `WithCheck(nil)` 禁用健康检查
- 但实际代码里会直接访问 `c.opts.check.TTL`
- 因此传 `nil` 会触发空指针 panic

所以当前项目没有使用 `nil` check，而是显式配置了 `TTL` check 作为默认值。

## 8. 配置示例

### 8.1 Gateway

启动脚本默认使用 Consul：

```bash
export USER_RPC_SERVICE=meshcart.user
export USER_RPC_DISCOVERY=consul
export CONSUL_ADDR=127.0.0.1:8500
```

回退直连模式：

```bash
export USER_RPC_SERVICE=meshcart.user
export USER_RPC_DISCOVERY=direct
export USER_RPC_ADDR=127.0.0.1:8888
```

### 8.2 User Service

宿主机运行：

```bash
export USER_RPC_SERVICE=meshcart.user
export USER_SERVICE_ADDR=127.0.0.1:8888
export USER_SERVICE_REGISTRY=consul
export CONSUL_ADDR=127.0.0.1:8500
export USER_SERVICE_CONSUL_TCP_CHECK=false
```

容器网络运行：

```bash
export USER_RPC_SERVICE=meshcart.user
export USER_SERVICE_ADDR=user-service:8888
export USER_SERVICE_REGISTRY=consul
export CONSUL_ADDR=consul:8500
export USER_SERVICE_CONSUL_TCP_CHECK=true
```

## 9. 启动流程

1. 启动 `docker compose up -d consul`
2. 启动 `user-service`，注册 `meshcart.user`
3. 启动 `gateway`，通过 Consul resolver 查找 `meshcart.user`
4. `gateway` 选择可用实例并发起 Kitex RPC 请求

## 10. 风险与约束

- 当前 Compose 只部署了 Consul，业务服务还没有全部容器化
- 只覆盖服务发现，不包含配置动态下发
- 当前依赖的 `registry-consul` 是社区适配库，不是 HashiCorp 官方直接维护的 Kitex 集成
- 如果后续进入生产环境，需要进一步评估 ACL、TLS、健康检查策略和高可用部署方式

## 11. 后续建议

- 为其它 RPC 服务复用同一套服务名、注册地址和健康检查约定
- 在 `runbook` 中补充完整的启动与排障步骤
- 如果后续对 Consul 接入稳定性要求更高，可以评估是否需要基于官方 `consul/api` 封装一层更可控的内部适配
