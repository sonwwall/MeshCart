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
	paymentservice "meshcart/kitex_gen/meshcart/payment/paymentservice"
	"meshcart/services/payment-service/biz/repository"
	bizservice "meshcart/services/payment-service/biz/service"
	"meshcart/services/payment-service/config"
	"meshcart/services/payment-service/dal/db"
	rpchandler "meshcart/services/payment-service/rpc/handler"
	orderrpc "meshcart/services/payment-service/rpcclient/order"
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
	adminServer := startAdminServer(sqlDB, &draining)

	svc := initService(mysqlDB, cfg)
	svr := newServer(cfg, svc)

	logx.L(nil).Info("payment-service starting")
	if err := lifecycle.RunUntilSignal(
		svr.Run,
		func(ctx context.Context) error {
			draining.Store(true)
			if err := lifecycle.WaitForDrainWindow(ctx, drainTimeout("PAYMENT_SERVICE_DRAIN_TIMEOUT_MS")); err != nil {
				return err
			}
			logx.L(nil).Info("payment-service shutting down", zap.Duration("timeout", shutdownTimeout()))
			if adminServer != nil {
				if err := adminServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
			}
			return svr.Stop()
		},
		shutdownTimeout(),
	); err != nil {
		logx.L(nil).Error("payment-service stopped with error", zap.Error(err))
	}
}

func initLogger() {
	if err := logx.Init(logx.Config{
		Service: "payment-service",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
		LogDir:  getEnv("LOG_DIR", "logs"),
	}); err != nil {
		panic(err)
	}
}

func initOpenTelemetry() func() {
	otel := otelprovider.NewOpenTelemetryProvider(
		otelprovider.WithServiceName("payment-service"),
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
	timeout := lifecycle.TimeoutFromMS(getEnvAsInt("PAYMENT_SERVICE_PREFLIGHT_TIMEOUT_MS", 1500), 1500*time.Millisecond)
	checks := []lifecycle.PreflightCheck{
		{Name: "mysql", Address: cfg.MySQL.Address},
	}
	if strings.EqualFold(getEnv("PAYMENT_SERVICE_REGISTRY", "consul"), "consul") {
		checks = append(checks, lifecycle.PreflightCheck{Name: "consul", Address: getEnv("CONSUL_ADDR", "127.0.0.1:8500")})
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := lifecycle.RunTCPPreflight(ctx, checks...); err != nil {
		logx.L(nil).Fatal("payment-service preflight failed", zap.Error(err), zap.Duration("timeout", timeout))
	}
	logx.L(nil).Info("payment-service preflight passed", zap.Duration("timeout", timeout))
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

func initService(mysqlDB *gorm.DB, cfg config.Config) *bizservice.PaymentService {
	repo := repository.NewMySQLPaymentRepository(mysqlDB, time.Duration(cfg.Timeout.DBQueryMS)*time.Millisecond)
	node, err := snowflake.NewNode(cfg.Snowflake.Node)
	if err != nil {
		logx.L(nil).Fatal("init snowflake node failed", zap.Error(err), zap.Int64("node", cfg.Snowflake.Node))
	}
	orderClient, err := orderrpc.NewClient(
		cfg.OrderRPC.ServiceName,
		cfg.OrderRPC.HostPort,
		cfg.OrderRPC.DiscoveryType,
		cfg.OrderRPC.ConsulAddress,
		time.Duration(cfg.OrderRPC.ConnectTimeoutMS)*time.Millisecond,
		time.Duration(cfg.OrderRPC.RPCTimeoutMS)*time.Millisecond,
	)
	if err != nil {
		logx.L(nil).Fatal("init order rpc client failed", zap.Error(err))
	}
	return bizservice.NewPaymentService(repo, node, orderClient)
}

func startAdminServer(sqlDB *sql.DB, draining *atomic.Bool) *http.Server {
	metricsAddr := getEnv("PAYMENT_METRICS_ADDR", ":9097")
	srv := &http.Server{
		Addr: metricsAddr,
		Handler: lifecycle.NewHTTPMux("payment-service", metricsx.PromHandler(), func(ctx context.Context) error {
			if draining != nil && draining.Load() {
				return errors.New("service is draining")
			}
			pingCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			return sqlDB.PingContext(pingCtx)
		}),
	}

	go func() {
		logx.L(nil).Info("payment-service admin server starting", zap.String("addr", metricsAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.L(nil).Error("payment-service admin server stopped with error", zap.Error(err))
		}
	}()
	return srv
}

func newServer(_ config.Config, svc *bizservice.PaymentService) server.Server {
	serviceName := getEnv("PAYMENT_RPC_SERVICE", "meshcart.payment")
	serviceAddr, err := mustResolveTCPAddr(getEnv("PAYMENT_SERVICE_ADDR", "127.0.0.1:8893"))
	if err != nil {
		logx.L(nil).Fatal("resolve payment-service addr failed", zap.Error(err))
	}

	opts := []server.Option{
		server.WithSuite(kitextrace.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
		server.WithServiceAddr(serviceAddr),
	}
	if strings.EqualFold(getEnv("PAYMENT_SERVICE_REGISTRY", "consul"), "consul") {
		consulRegistry, err := consul.NewConsulRegister(
			getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			consul.WithCheck(buildConsulCheck(serviceName, serviceAddr)),
		)
		if err != nil {
			logx.L(nil).Fatal("init consul registry failed", zap.Error(err))
		}
		opts = append(opts, server.WithRegistry(consulRegistry))
	}

	return paymentservice.NewServer(rpchandler.NewPaymentServiceImpl(svc), opts...)
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
	if strings.EqualFold(getEnv("PAYMENT_SERVICE_CONSUL_TCP_CHECK", "false"), "true") {
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

func getEnvAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func shutdownTimeout() time.Duration {
	return lifecycle.TimeoutFromMS(getEnvAsInt("PAYMENT_SERVICE_SHUTDOWN_TIMEOUT_MS", 5000), 5*time.Second)
}

func drainTimeout(key string) time.Duration {
	return lifecycle.TimeoutFromMS(getEnvAsInt(key, 500), 500*time.Millisecond)
}
