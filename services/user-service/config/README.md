# User Service 配置接入说明

## 1. 当前配置加载方式

`user-service` 当前支持两种配置来源：

- `file`：本地配置文件（Viper）
- `apollo`：Apollo 配置中心（预留接口，待实现）

配置入口代码在：`services/user-service/config/config.go`

## 2. 配置来源切换

通过环境变量 `USER_SERVICE_CONFIG_SOURCE` 控制：

- `file`（默认）：读取本地 YAML 文件
- `apollo`：调用已注册的 Apollo Loader

示例：

```bash
export USER_SERVICE_CONFIG_SOURCE=file
```

```bash
export USER_SERVICE_CONFIG_SOURCE=apollo
```

## 3. file 模式

默认读取：

- `services/user-service/config/user-service.local.yaml`

也可通过 `USER_SERVICE_CONFIG` 覆盖路径：

```bash
export USER_SERVICE_CONFIG=services/user-service/config/user-service.local.yaml
```

## 4. Apollo 预留接口

已预留以下接口：

```go
type ApolloLoader interface {
    Load() (Config, error)
}

func RegisterApolloLoader(loader ApolloLoader)
```

当 `USER_SERVICE_CONFIG_SOURCE=apollo` 时，`Load()` 会调用已注册的 `ApolloLoader`。

如果未注册，会返回错误。

占位实现文件：

- `services/user-service/config/apollo_stub.go`

## 5. 后续接入 Apollo 的步骤

1. 新建 Apollo 实现文件（示例：`apollo_loader.go`）
2. 在该文件中实现 `ApolloLoader` 接口
3. 在启动时（`rpc/main.go`）注册 Apollo Loader
4. 设置环境变量 `USER_SERVICE_CONFIG_SOURCE=apollo`
5. 启动服务并验证读取结果

## 6. Apollo 接入代码示例

```go
// apollo_loader.go
package config

type RealApolloLoader struct {
    // apollo client fields
}

func NewRealApolloLoader(/* params */) *RealApolloLoader {
    return &RealApolloLoader{}
}

func (l *RealApolloLoader) Load() (Config, error) {
    // 1. 调 Apollo SDK 拉取配置
    // 2. 解析为 Config 结构
    // 3. 返回 Config
    return Config{}, nil
}
```

```go
// rpc/main.go
cfgLoader := config.NewRealApolloLoader(/* ... */)
config.RegisterApolloLoader(cfgLoader)

cfg, err := config.Load()
if err != nil {
    log.Fatalf("load config failed: %v", err)
}
```

## 7. 配置字段说明

`Config` 结构中的 MySQL 字段：

- `mysql.address`
- `mysql.username`
- `mysql.password`
- `mysql.database`
- `mysql.charset`
- `mysql.parse_time`
- `mysql.loc`

建议 Apollo 中保持与本地 YAML 相同的字段命名，减少解析转换成本。
