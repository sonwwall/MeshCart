package handler

import (
	producthandler "meshcart/gateway/internal/handler/product"
	userhandler "meshcart/gateway/internal/handler/user"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(h *server.Hertz, svcCtx *svc.ServiceContext) {
	apiV1 := h.Group("/api/v1")
	producthandler.RegisterRoutes(apiV1, svcCtx)
	userhandler.RegisterRoutes(apiV1, svcCtx)
}
