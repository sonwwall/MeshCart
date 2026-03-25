package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	consulapi "github.com/hashicorp/consul/api"
	otelprovider "github.com/kitex-contrib/obs-opentelemetry/provider"
	kitextrace "github.com/kitex-contrib/obs-opentelemetry/tracing"
	consul "github.com/kitex-contrib/registry-consul"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"meshcart/app/lifecycle"
	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	orderservice "meshcart/kitex_gen/meshcart/order/orderservice"
	"meshcart/services/order-service/biz/repository"
	bizservice "meshcart/services/order-service/biz/service"
	"meshcart/services/order-service/config"
	"meshcart/services/order-service/dal/db"
	rpchandler "meshcart/services/order-service/rpc/handler"
	inventoryrpc "meshcart/services/order-service/rpcclient/inventory"
	productrpc "meshcart/services/order-service/rpcclient/product"
)

func Run() {
	initLogger()
	defer logx.Sync()

	defer initOpenTelemetry()()

	cfg := loadConfig()
	runPreflight(cfg)
	runMigrations(cfg)
	mysqlDB := initMySQL(cfg)
	sqlDB, err := mysqlDB.DB()
	if err != nil {
		logx.L(nil).Fatal("get mysql sql db failed", zap.Error(err))
	}
	defer sqlDB.Close()

	var draining atomic.Bool
	adminServer := startAdminServer(sqlDB, &draining, time.Duration(cfg.DBPool.StatsIntervalMS)*time.Millisecond)

	svc := initService(mysqlDB, cfg)
	svr := newServer(cfg, svc)

	logx.L(nil).Info("order-service starting")
	if err := lifecycle.RunUntilSignal(
		svr.Run,
		func(ctx context.Context) error {
			draining.Store(true)
			if err := lifecycle.WaitForDrainWindow(ctx, drainTimeout("ORDER_SERVICE_DRAIN_TIMEOUT_MS")); err != nil {
				return err
			}
			logx.L(nil).Info("order-service shutting down", zap.Duration("timeout", shutdownTimeout()))
			if adminServer != nil {
				if err := adminServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
			}
			return svr.Stop()
		},
		shutdownTimeout(),
	); err != nil {
		logx.L(nil).Error("order-service stopped with error", zap.Error(err))
	}
}

func initLogger() {
	if err := logx.Init(logx.Config{
		Service: "order-service",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
		LogDir:  getEnv("LOG_DIR", "logs"),
	}); err != nil {
		panic(err)
	}
}

func initOpenTelemetry() func() {
	otel := otelprovider.NewOpenTelemetryProvider(
		otelprovider.WithServiceName("order-service"),
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

func runPreflight(cfg config.Config) {
	timeout := lifecycle.TimeoutFromMS(getEnvAsInt("ORDER_SERVICE_PREFLIGHT_TIMEOUT_MS", 1500), 1500*time.Millisecond)
	checks := []lifecycle.PreflightCheck{
		{Name: "mysql", Address: cfg.MySQL.Address},
	}
	if strings.EqualFold(getEnv("ORDER_SERVICE_REGISTRY", "consul"), "consul") {
		checks = append(checks, lifecycle.PreflightCheck{Name: "consul", Address: getEnv("CONSUL_ADDR", "127.0.0.1:8500")})
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := lifecycle.RunTCPPreflight(ctx, checks...); err != nil {
		logx.L(nil).Fatal("order-service preflight failed", zap.Error(err), zap.Duration("timeout", timeout))
	}
	logx.L(nil).Info("order-service preflight passed", zap.Duration("timeout", timeout))
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
	mysqlDB, err := db.NewMySQL(cfg.MySQL.DSN(), db.PoolConfig{
		MaxOpenConns:    cfg.DBPool.MaxOpenConns,
		MaxIdleConns:    cfg.DBPool.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.DBPool.ConnMaxLifetime) * time.Minute,
	})
	if err != nil {
		logx.L(nil).Fatal("init mysql failed", zap.Error(err))
	}
	return mysqlDB
}

func initService(mysqlDB *gorm.DB, cfg config.Config) *bizservice.OrderService {
	repo := repository.NewMySQLOrderRepository(mysqlDB, time.Duration(cfg.Timeout.DBQueryMS)*time.Millisecond)
	node, err := snowflake.NewNode(cfg.Snowflake.Node)
	if err != nil {
		logx.L(nil).Fatal("init snowflake node failed", zap.Error(err), zap.Int64("node", cfg.Snowflake.Node))
	}
	productClient, err := productrpc.NewClient(
		cfg.ProductRPC.ServiceName,
		cfg.ProductRPC.HostPort,
		cfg.ProductRPC.DiscoveryType,
		cfg.ProductRPC.ConsulAddress,
		time.Duration(cfg.ProductRPC.ConnectTimeoutMS)*time.Millisecond,
		time.Duration(cfg.ProductRPC.RPCTimeoutMS)*time.Millisecond,
	)
	if err != nil {
		logx.L(nil).Fatal("init product rpc client failed", zap.Error(err))
	}
	inventoryClient, err := inventoryrpc.NewClient(
		cfg.InventoryRPC.ServiceName,
		cfg.InventoryRPC.HostPort,
		cfg.InventoryRPC.DiscoveryType,
		cfg.InventoryRPC.ConsulAddress,
		time.Duration(cfg.InventoryRPC.ConnectTimeoutMS)*time.Millisecond,
		time.Duration(cfg.InventoryRPC.RPCTimeoutMS)*time.Millisecond,
	)
	if err != nil {
		logx.L(nil).Fatal("init inventory rpc client failed", zap.Error(err))
	}
	return bizservice.NewOrderService(repo, node, productClient, inventoryClient)
}

func startAdminServer(sqlDB *sql.DB, draining *atomic.Bool, statsInterval time.Duration) *http.Server {
	metricsAddr := getEnv("ORDER_METRICS_ADDR", ":9096")
	stopCollectDBStats := metricsx.StartDBStatsCollector("order-service", sqlDB, statsInterval)
	srv := &http.Server{
		Addr: metricsAddr,
		Handler: lifecycle.NewHTTPMux("order-service", metricsx.PromHandler(), func(ctx context.Context) error {
			if draining != nil && draining.Load() {
				return errors.New("service is draining")
			}
			pingCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			return sqlDB.PingContext(pingCtx)
		}),
	}

	go func() {
		defer stopCollectDBStats()
		logx.L(nil).Info("order-service admin server starting", zap.String("addr", metricsAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.L(nil).Error("order-service admin server stopped with error", zap.Error(err))
		}
	}()
	return srv
}

func newServer(_ config.Config, svc *bizservice.OrderService) server.Server {
	serviceName := getEnv("ORDER_RPC_SERVICE", "meshcart.order")
	serviceAddr, err := mustResolveTCPAddr(getEnv("ORDER_SERVICE_ADDR", "127.0.0.1:8892"))
	if err != nil {
		logx.L(nil).Fatal("resolve order-service addr failed", zap.Error(err))
	}

	opts := []server.Option{
		server.WithSuite(kitextrace.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
		server.WithServiceAddr(serviceAddr),
	}
	if strings.EqualFold(getEnv("ORDER_SERVICE_REGISTRY", "consul"), "consul") {
		consulRegistry, err := consul.NewConsulRegister(
			getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			consul.WithCheck(buildConsulCheck(serviceName, serviceAddr)),
		)
		if err != nil {
			logx.L(nil).Fatal("init consul registry failed", zap.Error(err))
		}
		opts = append(opts, server.WithRegistry(consulRegistry))
	}

	return orderservice.NewServer(rpchandler.NewOrderServiceImpl(svc), opts...)
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
	if strings.EqualFold(getEnv("ORDER_SERVICE_CONSUL_TCP_CHECK", "false"), "true") {
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

func shutdownTimeout() time.Duration {
	return time.Duration(getEnvAsInt("ORDER_SERVICE_SHUTDOWN_TIMEOUT_MS", 5000)) * time.Millisecond
}

func drainTimeout(key string) time.Duration {
	return time.Duration(getEnvAsInt(key, 500)) * time.Millisecond
}

func getEnvAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
