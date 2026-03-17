package order

import (
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	orderGroup := api.Group("/orders")
	orderGroup.Use(svcCtx.JWT.MiddlewareFunc())
	orderGroup.GET("", ListOrders(svcCtx))
	orderGroup.POST("", CreateOrder(svcCtx))
	orderGroup.GET("/:order_id", GetOrder(svcCtx))
	orderGroup.POST("/:order_id/cancel", CancelOrder(svcCtx))
}
