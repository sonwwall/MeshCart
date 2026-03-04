gateway 这几个目录通常这样用：

biz/middleware

写“网关通用横切能力”：
鉴权（JWT/Token 解析）
请求日志（含 trace_id）
Recover（panic 兜底）
CORS
限流（用户/IP/接口级）
超时控制
幂等（可选，针对下单等）
灰度/AB 标记透传（可选）
biz/router

建议按模块拆分路由，不要全写一个文件。
例如：user.go、product.go、order.go，每个文件注册对应路由组。
最外层再有一个 register.go 统一调用各模块注册函数。
config

存网关配置（本地文件 + 环境变量）：
服务监听端口、超时、环境标识
各下游微服务地址/服务名（用于服务发现）
鉴权密钥/JWKS 地址
限流阈值、熔断参数
日志级别、trace 上报地址
CORS 白名单等
建议再放配置结构体和加载逻辑（如 config.go + config.yaml）
rpc

放调用下游 kitex client 的封装层：
client 初始化
每个服务的调用方法封装
统一超时/重试/错误转换
请求上下文透传（trace、user_id）
script

放脚本和自动化工具：
代码生成（hz/kitex 命令）
本地启动脚本
检查脚本（lint、api 校验）
打包/发布辅助脚本
你这套项目里，router 按模块拆分是非常推荐的；middleware 则按“能力类型”拆，不按业务模块拆。


