package main

import (
	"meshcart/gateway/config"
	"meshcart/gateway/internal/handler"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func main() {
	cfg := config.Load()
	svcCtx := svc.NewServiceContext(cfg)

	h := server.Default(server.WithHostPorts(cfg.Server.Addr))
	handler.Register(h, svcCtx)
	h.Spin()
}
