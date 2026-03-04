package handler

import (
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(h *server.Hertz, svcCtx *svc.ServiceContext) {
	v1 := h.Group("/api/v1")
	userGroup := v1.Group("/user")
	userGroup.POST("/login", UserLogin(svcCtx))
}
