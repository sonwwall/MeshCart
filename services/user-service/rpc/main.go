package main

import (
	"context"
	"net/http"
	"os"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	tracex "meshcart/app/trace"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
	"meshcart/services/user-service/biz/repository"
	bizservice "meshcart/services/user-service/biz/service"
	"meshcart/services/user-service/config"
	"meshcart/services/user-service/dal/db"

	"go.uber.org/zap"
)

func main() {
	if err := logx.Init(logx.Config{
		Service: "user-service",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
	}); err != nil {
		panic(err)
	}
	defer logx.Sync()

	// 初始化 trace exporter：把 user-service 的 span 上报到 OTel Collector。
	traceShutdown, err := tracex.Init(context.Background(), tracex.Config{
		ServiceName: "user-service",
		Environment: getEnv("APP_ENV", "dev"),
		Endpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4319"),
		Insecure:    true,
	})
	if err != nil {
		panic(err)
	}
	defer func() { _ = traceShutdown(context.Background()) }()

	cfg, err := config.Load()
	if err != nil {
		logx.L(nil).Fatal("load config failed", zap.Error(err))
	}

	if cfg.Migration.Enabled {
		if err := db.RunMigrations(cfg.MySQL.DSN(), cfg.Migration.Source); err != nil {
			logx.L(nil).Fatal("run migrations failed", zap.Error(err), zap.String("source", cfg.Migration.Source))
		}
		logx.L(nil).Info("database migrations applied", zap.String("source", cfg.Migration.Source))
	}

	mysqlDB, err := db.NewMySQL(cfg.MySQL.DSN())
	if err != nil {
		logx.L(nil).Fatal("init mysql failed", zap.Error(err))
	}
	sqlDB, err := mysqlDB.DB()
	if err != nil {
		logx.L(nil).Fatal("get mysql sql db failed", zap.Error(err))
	}
	defer sqlDB.Close()

	repo := repository.NewMySQLUserRepository(mysqlDB)
	svc := bizservice.NewUserService(repo)

	metricsAddr := getEnv("USER_METRICS_ADDR", ":9091")
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsx.PromHandler())
		logx.L(nil).Info("user-service metrics server starting", zap.String("addr", metricsAddr))
		if err := http.ListenAndServe(metricsAddr, mux); err != nil {
			logx.L(nil).Error("user-service metrics server stopped with error", zap.Error(err))
		}
	}()

	svr := userservice.NewServer(NewUserServiceImpl(svc))
	logx.L(nil).Info("user-service starting")
	if err := svr.Run(); err != nil {
		logx.L(nil).Error("user-service stopped with error", zap.Error(err))
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
