package cart

import (
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	cartGroup := api.Group("/cart")
	cartGroup.Use(svcCtx.JWT.MiddlewareFunc())
	cartGroup.GET("", GetCart(svcCtx))
	cartGroup.POST("/items", AddCartItem(svcCtx))
	cartGroup.PUT("/items/:item_id", UpdateCartItem(svcCtx))
	cartGroup.DELETE("/items/:item_id", RemoveCartItem(svcCtx))
	cartGroup.DELETE("", ClearCart(svcCtx))
}
