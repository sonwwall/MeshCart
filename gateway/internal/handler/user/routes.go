package user

import (
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	userGroup := api.Group("/user")
	userGroup.POST("/login", Login(svcCtx))
	userGroup.POST("/register", Register(svcCtx))
	userGroup.GET("/refresh_token", svcCtx.JWT.RefreshHandler)

	authGroup := userGroup.Group("")
	authGroup.Use(svcCtx.JWT.MiddlewareFunc())
	authGroup.GET("/me", Me(svcCtx))
}
