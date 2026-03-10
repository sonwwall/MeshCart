package product

import (
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	api.GET("/products", ListProducts(svcCtx))
	api.GET("/products/detail/:product_id", GetProductDetail(svcCtx))

	adminGroup := api.Group("/admin/products")
	adminGroup.Use(svcCtx.JWT.MiddlewareFunc())
	adminGroup.POST("", CreateProduct(svcCtx))
	adminGroup.PUT("/:product_id", UpdateProduct(svcCtx))
	adminGroup.POST("/:product_id/status", ChangeProductStatus(svcCtx))
}
