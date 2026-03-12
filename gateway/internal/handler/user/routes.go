package user

import (
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	userGroup := api.Group("/user")
	userGroup.POST(
		"/login",
		middleware.RateLimit(svcCtx.RateLimiter, middleware.NewRule(svcCtx.Config.RateLimit.LoginIP), middleware.IPRouteKey),
		Login(svcCtx),
	)
	userGroup.POST(
		"/register",
		middleware.RateLimit(svcCtx.RateLimiter, middleware.NewRule(svcCtx.Config.RateLimit.RegisterIP), middleware.IPRouteKey),
		Register(svcCtx),
	)
	userGroup.GET("/refresh_token", svcCtx.JWT.RefreshHandler)

	authGroup := userGroup.Group("")
	authGroup.Use(svcCtx.JWT.MiddlewareFunc())
	authGroup.GET("/me", Me(svcCtx))

	adminGroup := api.Group("/admin/users")
	adminGroup.Use(svcCtx.JWT.MiddlewareFunc())
	adminGroup.PUT("/:user_id/role", UpdateUserRole(svcCtx))
}
