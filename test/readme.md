test/integration

集成测试：验证“多个模块一起工作”是否正常。
例如：gateway -> order-service -> inventory-service 下单链路是否通。
通常会连测试库/测试 Redis/MQ，关注接口协作和数据流。
test/e2e

端到端测试：从用户入口走完整流程，尽量接近真实环境。
例如：登录 -> 加购物车 -> 下单 -> 支付回调 -> 订单状态更新。
关注业务结果，不关心内部实现细节。
简化理解：integration 测模块协作，e2e 测用户全链路。


