package inventory

import (
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	adminGroup := api.Group("/admin/inventory")
	adminGroup.Use(svcCtx.JWT.MiddlewareFunc())
	adminGroup.Use(
		middleware.RateLimit(svcCtx.RateLimiter, middleware.NewRule(svcCtx.Config.RateLimit.AdminWriteUser), middleware.UserRouteKey),
		middleware.RateLimit(svcCtx.RateLimiter, middleware.NewRule(svcCtx.Config.RateLimit.AdminWriteRoute), middleware.RouteKey),
	)
	adminGroup.GET("/skus/:sku_id", GetSkuStock(svcCtx))
	adminGroup.POST("/skus/batch_get", BatchGetSkuStock(svcCtx))
	adminGroup.PUT("/skus/:sku_id/stock", AdjustStock(svcCtx))
}
