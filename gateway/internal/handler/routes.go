package handler

import (
	userhandler "meshcart/gateway/internal/handler/user"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(h *server.Hertz, svcCtx *svc.ServiceContext) {
	apiV1 := h.Group("/api/v1")
	userhandler.RegisterRoutes(apiV1, svcCtx)
}
