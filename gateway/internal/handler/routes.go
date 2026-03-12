package handler

import (
	producthandler "meshcart/gateway/internal/handler/product"
	userhandler "meshcart/gateway/internal/handler/user"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(h *server.Hertz, svcCtx *svc.ServiceContext) {
	apiV1 := h.Group("/api/v1")
	apiV1.Use(
		middleware.RateLimit(
			svcCtx.RateLimiter,
			middleware.NewRule(svcCtx.Config.RateLimit.GlobalIP),
			middleware.IPKey,
		),
	)
	producthandler.RegisterRoutes(apiV1, svcCtx)
	userhandler.RegisterRoutes(apiV1, svcCtx)
}
