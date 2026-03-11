package component

import (
	logx "meshcart/app/log"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/handler"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
	hzprom "github.com/hertz-contrib/monitor-prometheus"
	hztrace "github.com/hertz-contrib/obs-opentelemetry/tracing"
	"go.uber.org/zap"
)

func NewGatewayServer(cfg config.Config, svcCtx *svc.ServiceContext) *server.Hertz {
	serverTracer, traceCfg := hztrace.NewServerTracer()
	promTracer := hzprom.NewServerTracer(cfg.Metrics.Addr, cfg.Metrics.Path)

	h := server.Default(
		server.WithHostPorts(cfg.Server.Addr),
		server.WithReadTimeout(cfg.Server.ReadTimeout),
		server.WithWriteTimeout(cfg.Server.WriteTimeout),
		server.WithIdleTimeout(cfg.Server.IdleTimeout),
		serverTracer,
		server.WithTracer(promTracer),
	)
	h.Use(
		hztrace.ServerMiddleware(traceCfg),
		middleware.RequestTimeout(cfg.Server.RequestTimeout),
	)
	handler.Register(h, svcCtx)
	return h
}

func StartServer(h *server.Hertz, cfg config.Config) {
	logx.L(nil).Info("gateway starting", zap.String("addr", cfg.Server.Addr))
	h.Spin()
}
