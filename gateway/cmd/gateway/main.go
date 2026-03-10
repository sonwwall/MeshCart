package main

import (
	"context"

	logx "meshcart/app/log"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/component"
	"meshcart/gateway/internal/svc"
)

func main() {
	cfg := config.Load()
	component.InitLogger(cfg)
	defer logx.Sync()

	otel := component.InitOpenTelemetry(cfg)
	defer func() { _ = otel.Shutdown(context.Background()) }()

	svcCtx := svc.NewServiceContext(cfg)
	h := component.NewGatewayServer(cfg, svcCtx)
	component.StartServer(h, cfg)
}
