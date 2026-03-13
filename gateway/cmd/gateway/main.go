package main

import (
	"context"

	logx "meshcart/app/log"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/component"
	"meshcart/gateway/internal/svc"

	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	component.InitLogger(cfg)
	defer logx.Sync()

	otel := component.InitOpenTelemetry(cfg)
	defer func() { _ = otel.Shutdown(context.Background()) }()

	if err := component.RunPreflight(cfg); err != nil {
		logx.L(nil).Fatal("gateway preflight failed", zap.Error(err))
	}

	svcCtx := svc.NewServiceContext(cfg)
	h := component.NewGatewayServer(cfg, svcCtx)
	component.StartServer(h, cfg, svcCtx)
}
