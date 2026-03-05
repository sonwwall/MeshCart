package user

import (
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	userGroup := api.Group("/user")
	userGroup.POST("/login", Login(svcCtx))
}
