package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	consulapi "github.com/hashicorp/consul/api"
	otelprovider "github.com/kitex-contrib/obs-opentelemetry/provider"
	kitextrace "github.com/kitex-contrib/obs-opentelemetry/tracing"
	consul "github.com/kitex-contrib/registry-consul"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
	"meshcart/services/user-service/biz/repository"
	bizservice "meshcart/services/user-service/biz/service"
	"meshcart/services/user-service/config"
	"meshcart/services/user-service/dal/db"
	rpchandler "meshcart/services/user-service/rpc/handler"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Run() {
	initLogger()
	defer logx.Sync()

	defer initOpenTelemetry()()

	cfg := loadConfig()
	runMigrations(cfg)
	mysqlDB := initMySQL(cfg)
	sqlDB, err := mysqlDB.DB()
	if err != nil {
		logx.L(nil).Fatal("get mysql sql db failed", zap.Error(err))
	}
	defer sqlDB.Close()

	startMetricsServer()

	svc := initService(mysqlDB, cfg)
	svr := newServer(cfg, svc)

	logx.L(nil).Info("user-service starting")
	if err := svr.Run(); err != nil {
		logx.L(nil).Error("user-service stopped with error", zap.Error(err))
	}
}

func initLogger() {
	if err := logx.Init(logx.Config{
		Service: "user-service",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
		LogDir:  getEnv("LOG_DIR", "logs"),
	}); err != nil {
		panic(err)
	}
}

func initOpenTelemetry() func() {
	otel := otelprovider.NewOpenTelemetryProvider(
		otelprovider.WithServiceName("user-service"),
		otelprovider.WithDeploymentEnvironment(getEnv("APP_ENV", "dev")),
		otelprovider.WithExportEndpoint(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4319")),
		otelprovider.WithInsecure(),
	)
	return func() { _ = otel.Shutdown(context.Background()) }
}

func loadConfig() config.Config {
	cfg, err := config.Load()
	if err != nil {
		logx.L(nil).Fatal("load config failed", zap.Error(err))
	}
	return cfg
}

func runMigrations(cfg config.Config) {
	if !cfg.Migration.Enabled {
		return
	}
	if err := db.RunMigrations(cfg.MySQL.DSN(), cfg.Migration.Source); err != nil {
		logx.L(nil).Fatal("run migrations failed", zap.Error(err), zap.String("source", cfg.Migration.Source))
	}
	logx.L(nil).Info("database migrations applied", zap.String("source", cfg.Migration.Source))
}

func initMySQL(cfg config.Config) *gorm.DB {
	mysqlDB, err := db.NewMySQL(cfg.MySQL.DSN())
	if err != nil {
		logx.L(nil).Fatal("init mysql failed", zap.Error(err))
	}
	return mysqlDB
}

func initService(mysqlDB *gorm.DB, cfg config.Config) *bizservice.UserService {
	repo := repository.NewMySQLUserRepository(mysqlDB, time.Duration(cfg.Timeout.DBQueryMS)*time.Millisecond)
	node, err := snowflake.NewNode(cfg.Snowflake.Node)
	if err != nil {
		logx.L(nil).Fatal("init snowflake node failed", zap.Error(err), zap.Int64("node", cfg.Snowflake.Node))
	}
	return bizservice.NewUserService(repo, node)
}

func startMetricsServer() {
	metricsAddr := getEnv("USER_METRICS_ADDR", ":9091")
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsx.PromHandler())
		logx.L(nil).Info("user-service metrics server starting", zap.String("addr", metricsAddr))
		if err := http.ListenAndServe(metricsAddr, mux); err != nil {
			logx.L(nil).Error("user-service metrics server stopped with error", zap.Error(err))
		}
	}()
}

func newServer(cfg config.Config, svc *bizservice.UserService) server.Server {
	serviceName := getEnv("USER_RPC_SERVICE", "meshcart.user")
	serviceAddr, err := mustResolveTCPAddr(getEnv("USER_SERVICE_ADDR", "127.0.0.1:8888"))
	if err != nil {
		logx.L(nil).Fatal("resolve user-service addr failed", zap.Error(err))
	}

	opts := []server.Option{
		server.WithSuite(kitextrace.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
		server.WithServiceAddr(serviceAddr),
	}
	if strings.EqualFold(getEnv("USER_SERVICE_REGISTRY", "consul"), "consul") {
		consulRegistry, err := consul.NewConsulRegister(
			getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			consul.WithCheck(buildConsulCheck(serviceName, serviceAddr)),
		)
		if err != nil {
			logx.L(nil).Fatal("init consul registry failed", zap.Error(err))
		}
		opts = append(opts, server.WithRegistry(consulRegistry))
	}

	return userservice.NewServer(rpchandler.NewUserServiceImpl(svc), opts...)
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
	if strings.EqualFold(getEnv("USER_SERVICE_CONSUL_TCP_CHECK", "false"), "true") {
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
