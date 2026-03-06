package main

import (
	"context"
	"os"

	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/handler"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
	"go.uber.org/zap"
)

func main() {
	if err := logx.Init(logx.Config{
		Service: "gateway",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
	}); err != nil {
		panic(err)
	}
	defer logx.Sync()

	traceShutdown, err := tracex.Init(context.Background(), tracex.Config{
		ServiceName: "gateway",
		Environment: getEnv("APP_ENV", "dev"),
		Endpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4319"),
		Insecure:    true,
	})
	if err != nil {
		panic(err)
	}
	defer func() { _ = traceShutdown(context.Background()) }()

	cfg := config.Load()
	svcCtx := svc.NewServiceContext(cfg)

	logx.L(nil).Info("gateway starting", zap.String("addr", cfg.Server.Addr))
	h := server.Default(server.WithHostPorts(cfg.Server.Addr))
	handler.Register(h, svcCtx)
	h.Spin()
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
