package component

import (
	"context"

	"meshcart/app/lifecycle"
	logx "meshcart/app/log"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/handler"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
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
	registerLifecycleRoutes(h, svcCtx)
	handler.Register(h, svcCtx)
	return h
}

func StartServer(h *server.Hertz, cfg config.Config, svcCtx *svc.ServiceContext) {
	logx.L(nil).Info("gateway starting", zap.String("addr", cfg.Server.Addr))
	err := lifecycle.RunUntilSignal(
		h.Run,
		func(ctx context.Context) error {
			if svcCtx != nil && svcCtx.Draining != nil {
				svcCtx.Draining.Store(true)
			}
			if err := lifecycle.WaitForDrainWindow(ctx, cfg.Server.DrainTimeout); err != nil {
				return err
			}
			logx.L(nil).Info("gateway shutting down", zap.Duration("timeout", cfg.Server.ShutdownTimeout))
			return h.Shutdown(ctx)
		},
		cfg.Server.ShutdownTimeout,
	)
	if err != nil {
		logx.L(nil).Error("gateway stopped with error", zap.Error(err))
	}
}

func registerLifecycleRoutes(h *server.Hertz, svcCtx *svc.ServiceContext) {
	h.GET("/healthz", func(_ context.Context, c *app.RequestContext) {
		c.String(200, "ok service=gateway\n")
	})
	h.GET("/readyz", func(_ context.Context, c *app.RequestContext) {
		if svcCtx != nil && svcCtx.Draining != nil && svcCtx.Draining.Load() {
			c.String(503, "not ready service=gateway err=service is draining\n")
			return
		}
		c.String(200, "ready service=gateway\n")
	})
}
