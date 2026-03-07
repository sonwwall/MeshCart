package main

import (
	"context"
	"os"

	logx "meshcart/app/log"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/handler"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
	hzprom "github.com/hertz-contrib/monitor-prometheus"
	otelprovider "github.com/hertz-contrib/obs-opentelemetry/provider"
	hztrace "github.com/hertz-contrib/obs-opentelemetry/tracing"
	"go.uber.org/zap"
)

func main() {

	//日志初始化
	if err := logx.Init(logx.Config{
		Service: "gateway",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
		LogDir:  getEnv("LOG_DIR", "logs"),
	}); err != nil {
		panic(err)
	}
	defer logx.Sync()

	// 使用 hertz-contrib/provider 初始化 OTel Provider。
	otel := otelprovider.NewOpenTelemetryProvider(
		otelprovider.WithServiceName("gateway"),
		otelprovider.WithDeploymentEnvironment(getEnv("APP_ENV", "dev")),
		otelprovider.WithExportEndpoint(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4319")),
		otelprovider.WithInsecure(),
	)
	defer func() { _ = otel.Shutdown(context.Background()) }()

	cfg := config.Load()
	svcCtx := svc.NewServiceContext(cfg)

	// hertz-contrib tracing: 生成服务追踪器与中间件配置。
	serverTracer, traceCfg := hztrace.NewServerTracer()

	// hertz-contrib monitor-prometheus: 采集 Hertz 服务端基础指标。
	promTracer := hzprom.NewServerTracer(
		getEnv("GATEWAY_PROM_ADDR", ":9092"),
		getEnv("GATEWAY_PROM_PATH", "/metrics"),
	)

	logx.L(nil).Info("gateway starting", zap.String("addr", cfg.Server.Addr))
	h := server.Default(
		server.WithHostPorts(cfg.Server.Addr),
		serverTracer,
		server.WithTracer(promTracer),
	)
	h.Use(hztrace.ServerMiddleware(traceCfg))
	handler.Register(h, svcCtx)
	h.Spin()
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
