package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	App          AppConfig
	Log          LogConfig
	Telemetry    TelemetryConfig
	Metrics      MetricsConfig
	Server       ServerConfig
	DTM          DTMConfig
	RateLimit    RateLimitConfig
	UserRPC      UserRPCConfig
	CartRPC      CartRPCConfig
	OrderRPC     OrderRPCConfig
	ProductRPC   ProductRPCConfig
	InventoryRPC InventoryRPCConfig
	JWT          JWTConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type LogConfig struct {
	Level  string
	LogDir string
}

type TelemetryConfig struct {
	Endpoint string
	Insecure bool
}

type MetricsConfig struct {
	Addr string
	Path string
}

type ServerConfig struct {
	Addr             string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	RequestTimeout   time.Duration
	ShutdownTimeout  time.Duration
	PreflightTimeout time.Duration
	DrainTimeout     time.Duration
}

type DTMConfig struct {
	Server              string
	WorkflowCallbackURL string
}

type RateLimitConfig struct {
	Enabled         bool
	EntryTTL        time.Duration
	CleanupInterval time.Duration
	GlobalIP        RateLimitRuleConfig
	LoginIP         RateLimitRuleConfig
	RegisterIP      RateLimitRuleConfig
	AdminWriteUser  RateLimitRuleConfig
	AdminWriteRoute RateLimitRuleConfig
}

type RateLimitRuleConfig struct {
	RatePerSecond int
	Burst         int
}

type UserRPCConfig struct {
	ServiceName    string
	Address        string
	DiscoveryType  string
	ConsulAddress  string
	ConnectTimeout time.Duration
	RPCTimeout     time.Duration
}

type ProductRPCConfig struct {
	ServiceName    string
	Address        string
	DiscoveryType  string
	ConsulAddress  string
	ConnectTimeout time.Duration
	RPCTimeout     time.Duration
}

type CartRPCConfig struct {
	ServiceName    string
	Address        string
	DiscoveryType  string
	ConsulAddress  string
	ConnectTimeout time.Duration
	RPCTimeout     time.Duration
}

type OrderRPCConfig struct {
	ServiceName    string
	Address        string
	DiscoveryType  string
	ConsulAddress  string
	ConnectTimeout time.Duration
	RPCTimeout     time.Duration
}

type InventoryRPCConfig struct {
	ServiceName    string
	Address        string
	DiscoveryType  string
	ConsulAddress  string
	ConnectTimeout time.Duration
	RPCTimeout     time.Duration
}

type JWTConfig struct {
	Secret            string
	Issuer            string
	TimeoutMinutes    int
	MaxRefreshMinutes int
}

func Load() Config {
	return Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "gateway"),
			Env:  getEnv("APP_ENV", "dev"),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			LogDir: getEnv("LOG_DIR", "logs"),
		},
		Telemetry: TelemetryConfig{
			Endpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4319"),
			Insecure: getEnvAsBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		},
		Metrics: MetricsConfig{
			Addr: getEnv("GATEWAY_PROM_ADDR", ":9092"),
			Path: getEnv("GATEWAY_PROM_PATH", "/metrics"),
		},
		Server: ServerConfig{
			Addr:             getEnv("GATEWAY_ADDR", ":8080"),
			ReadTimeout:      getEnvAsDuration("GATEWAY_READ_TIMEOUT_MS", 5*time.Second),
			WriteTimeout:     getEnvAsDuration("GATEWAY_WRITE_TIMEOUT_MS", 5*time.Second),
			IdleTimeout:      getEnvAsDuration("GATEWAY_IDLE_TIMEOUT_MS", 60*time.Second),
			RequestTimeout:   getEnvAsDuration("GATEWAY_REQUEST_TIMEOUT_MS", 3*time.Second),
			ShutdownTimeout:  getEnvAsDuration("GATEWAY_SHUTDOWN_TIMEOUT_MS", 5*time.Second),
			PreflightTimeout: getEnvAsDuration("GATEWAY_PREFLIGHT_TIMEOUT_MS", 1500*time.Millisecond),
			DrainTimeout:     getEnvAsDuration("GATEWAY_DRAIN_TIMEOUT_MS", 500*time.Millisecond),
		},
		DTM: DTMConfig{
			Server:              getEnv("DTM_SERVER", "http://127.0.0.1:36789/api/dtmsvr"),
			WorkflowCallbackURL: getEnv("DTM_WORKFLOW_CALLBACK_URL", "http://127.0.0.1:8080/api/internal/dtm/workflow"),
		},
		RateLimit: RateLimitConfig{
			Enabled:         getEnvAsBool("GATEWAY_RATE_LIMIT_ENABLED", true),
			EntryTTL:        getEnvAsDuration("GATEWAY_RATE_LIMIT_ENTRY_TTL_MS", 10*time.Minute),
			CleanupInterval: getEnvAsDuration("GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL_MS", time.Minute),
			GlobalIP: RateLimitRuleConfig{
				RatePerSecond: getEnvAsInt("GATEWAY_GLOBAL_IP_RATE_LIMIT_RPS", 50),
				Burst:         getEnvAsInt("GATEWAY_GLOBAL_IP_RATE_LIMIT_BURST", 100),
			},
			LoginIP: RateLimitRuleConfig{
				RatePerSecond: getEnvAsInt("GATEWAY_LOGIN_IP_RATE_LIMIT_RPS", 5),
				Burst:         getEnvAsInt("GATEWAY_LOGIN_IP_RATE_LIMIT_BURST", 10),
			},
			RegisterIP: RateLimitRuleConfig{
				RatePerSecond: getEnvAsInt("GATEWAY_REGISTER_IP_RATE_LIMIT_RPS", 2),
				Burst:         getEnvAsInt("GATEWAY_REGISTER_IP_RATE_LIMIT_BURST", 5),
			},
			AdminWriteUser: RateLimitRuleConfig{
				RatePerSecond: getEnvAsInt("GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_RPS", 3),
				Burst:         getEnvAsInt("GATEWAY_ADMIN_WRITE_USER_RATE_LIMIT_BURST", 6),
			},
			AdminWriteRoute: RateLimitRuleConfig{
				RatePerSecond: getEnvAsInt("GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_RPS", 20),
				Burst:         getEnvAsInt("GATEWAY_ADMIN_WRITE_ROUTE_RATE_LIMIT_BURST", 40),
			},
		},
		UserRPC: UserRPCConfig{
			ServiceName:    getEnv("USER_RPC_SERVICE", "meshcart.user"),
			Address:        getEnv("USER_RPC_ADDR", "127.0.0.1:8888"),
			DiscoveryType:  getEnv("USER_RPC_DISCOVERY", "direct"),
			ConsulAddress:  getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			ConnectTimeout: getEnvAsDuration("USER_RPC_CONNECT_TIMEOUT_MS", 500*time.Millisecond),
			RPCTimeout:     getEnvAsDuration("USER_RPC_TIMEOUT_MS", 2*time.Second),
		},
		CartRPC: CartRPCConfig{
			ServiceName:    getEnv("CART_RPC_SERVICE", "meshcart.cart"),
			Address:        getEnv("CART_RPC_ADDR", "127.0.0.1:8890"),
			DiscoveryType:  getEnv("CART_RPC_DISCOVERY", "direct"),
			ConsulAddress:  getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			ConnectTimeout: getEnvAsDuration("CART_RPC_CONNECT_TIMEOUT_MS", 500*time.Millisecond),
			RPCTimeout:     getEnvAsDuration("CART_RPC_TIMEOUT_MS", 2*time.Second),
		},
		OrderRPC: OrderRPCConfig{
			ServiceName:    getEnv("ORDER_RPC_SERVICE", "meshcart.order"),
			Address:        getEnv("ORDER_RPC_ADDR", "127.0.0.1:8892"),
			DiscoveryType:  getEnv("ORDER_RPC_DISCOVERY", "direct"),
			ConsulAddress:  getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			ConnectTimeout: getEnvAsDuration("ORDER_RPC_CONNECT_TIMEOUT_MS", 500*time.Millisecond),
			RPCTimeout:     getEnvAsDuration("ORDER_RPC_TIMEOUT_MS", 2*time.Second),
		},
		ProductRPC: ProductRPCConfig{
			ServiceName:    getEnv("PRODUCT_RPC_SERVICE", "meshcart.product"),
			Address:        getEnv("PRODUCT_RPC_ADDR", "127.0.0.1:8889"),
			DiscoveryType:  getEnv("PRODUCT_RPC_DISCOVERY", "direct"),
			ConsulAddress:  getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			ConnectTimeout: getEnvAsDuration("PRODUCT_RPC_CONNECT_TIMEOUT_MS", 500*time.Millisecond),
			RPCTimeout:     getEnvAsDuration("PRODUCT_RPC_TIMEOUT_MS", 2*time.Second),
		},
		InventoryRPC: InventoryRPCConfig{
			ServiceName:    getEnv("INVENTORY_RPC_SERVICE", "meshcart.inventory"),
			Address:        getEnv("INVENTORY_RPC_ADDR", "127.0.0.1:8891"),
			DiscoveryType:  getEnv("INVENTORY_RPC_DISCOVERY", "direct"),
			ConsulAddress:  getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			ConnectTimeout: getEnvAsDuration("INVENTORY_RPC_CONNECT_TIMEOUT_MS", 500*time.Millisecond),
			RPCTimeout:     getEnvAsDuration("INVENTORY_RPC_TIMEOUT_MS", 2*time.Second),
		},
		JWT: JWTConfig{
			Secret:            getEnv("JWT_SECRET", "meshcart-dev-secret-change-me"),
			Issuer:            getEnv("JWT_ISSUER", "meshcart.gateway"),
			TimeoutMinutes:    getEnvAsInt("JWT_TIMEOUT_MINUTES", 120),
			MaxRefreshMinutes: getEnvAsInt("JWT_MAX_REFRESH_MINUTES", 720),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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

func getEnvAsBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Millisecond
}
