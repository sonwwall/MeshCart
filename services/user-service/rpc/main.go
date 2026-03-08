package main

import (
	"context"
	"net/http"
	"os"

	"github.com/bwmarrin/snowflake"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	otelprovider "github.com/kitex-contrib/obs-opentelemetry/provider"
	kitextrace "github.com/kitex-contrib/obs-opentelemetry/tracing"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
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
		LogDir:  getEnv("LOG_DIR", "logs"),
	}); err != nil {
		panic(err)
	}
	defer logx.Sync()

	otel := otelprovider.NewOpenTelemetryProvider(
		otelprovider.WithServiceName("user-service"),
		otelprovider.WithDeploymentEnvironment(getEnv("APP_ENV", "dev")),
		otelprovider.WithExportEndpoint(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4319")),
		otelprovider.WithInsecure(),
	)
	defer func() { _ = otel.Shutdown(context.Background()) }()

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
	node, err := snowflake.NewNode(cfg.Snowflake.Node)
	if err != nil {
		logx.L(nil).Fatal("init snowflake node failed", zap.Error(err), zap.Int64("node", cfg.Snowflake.Node))
	}
	svc := bizservice.NewUserService(repo, node)

	metricsAddr := getEnv("USER_METRICS_ADDR", ":9091")
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsx.PromHandler())
		logx.L(nil).Info("user-service metrics server starting", zap.String("addr", metricsAddr))
		if err := http.ListenAndServe(metricsAddr, mux); err != nil {
			logx.L(nil).Error("user-service metrics server stopped with error", zap.Error(err))
		}
	}()

	svr := userservice.NewServer(
		NewUserServiceImpl(svc),
		server.WithSuite(kitextrace.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "user-service"}),
	)
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
