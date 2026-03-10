package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	consulapi "github.com/hashicorp/consul/api"
	otelprovider "github.com/kitex-contrib/obs-opentelemetry/provider"
	kitextrace "github.com/kitex-contrib/obs-opentelemetry/tracing"
	consul "github.com/kitex-contrib/registry-consul"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	productservice "meshcart/kitex_gen/meshcart/product/productservice"
	"meshcart/services/product-service/biz/repository"
	bizservice "meshcart/services/product-service/biz/service"
	"meshcart/services/product-service/config"
	"meshcart/services/product-service/dal/db"

	"go.uber.org/zap"
)

func main() {
	if err := logx.Init(logx.Config{
		Service: "product-service",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
		LogDir:  getEnv("LOG_DIR", "logs"),
	}); err != nil {
		panic(err)
	}
	defer logx.Sync()

	otel := otelprovider.NewOpenTelemetryProvider(
		otelprovider.WithServiceName("product-service"),
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

	repo := repository.NewMySQLProductRepository(mysqlDB)
	node, err := snowflake.NewNode(cfg.Snowflake.Node)
	if err != nil {
		logx.L(nil).Fatal("init snowflake node failed", zap.Error(err), zap.Int64("node", cfg.Snowflake.Node))
	}
	svc := bizservice.NewProductService(repo, node)

	metricsAddr := getEnv("PRODUCT_METRICS_ADDR", ":9093")
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsx.PromHandler())
		logx.L(nil).Info("product-service metrics server starting", zap.String("addr", metricsAddr))
		if err := http.ListenAndServe(metricsAddr, mux); err != nil {
			logx.L(nil).Error("product-service metrics server stopped with error", zap.Error(err))
		}
	}()

	serviceName := getEnv("PRODUCT_RPC_SERVICE", "meshcart.product")
	serviceAddr, err := mustResolveTCPAddr(getEnv("PRODUCT_SERVICE_ADDR", "127.0.0.1:8889"))
	if err != nil {
		logx.L(nil).Fatal("resolve product-service addr failed", zap.Error(err))
	}

	opts := []server.Option{
		server.WithSuite(kitextrace.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
		server.WithServiceAddr(serviceAddr),
	}
	if strings.EqualFold(getEnv("PRODUCT_SERVICE_REGISTRY", "consul"), "consul") {
		consulRegistry, err := consul.NewConsulRegister(
			getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			consul.WithCheck(buildConsulCheck(serviceName, serviceAddr)),
		)
		if err != nil {
			logx.L(nil).Fatal("init consul registry failed", zap.Error(err))
		}
		opts = append(opts, server.WithRegistry(consulRegistry))
	}

	svr := productservice.NewServer(NewProductServiceImpl(svc), opts...)
	logx.L(nil).Info("product-service starting")
	if err := svr.Run(); err != nil {
		logx.L(nil).Error("product-service stopped with error", zap.Error(err))
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustResolveTCPAddr(addr string) (*net.TCPAddr, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve tcp addr %q: %w", addr, err)
	}
	return tcpAddr, nil
}

func buildConsulCheck(serviceName string, serviceAddr *net.TCPAddr) *consulapi.AgentServiceCheck {
	checkID := fmt.Sprintf("service:%s:%s", serviceName, serviceAddr.String())
	if strings.EqualFold(getEnv("PRODUCT_SERVICE_CONSUL_TCP_CHECK", "false"), "true") {
		return &consulapi.AgentServiceCheck{
			CheckID:                        checkID,
			TCP:                            serviceAddr.String(),
			Interval:                       "5s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "1m",
		}
	}

	return &consulapi.AgentServiceCheck{
		CheckID:                        checkID,
		TTL:                            "10s",
		DeregisterCriticalServiceAfter: "1m",
	}
}
