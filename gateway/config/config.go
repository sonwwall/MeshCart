package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	App        AppConfig
	Log        LogConfig
	Telemetry  TelemetryConfig
	Metrics    MetricsConfig
	Server     ServerConfig
	UserRPC    UserRPCConfig
	ProductRPC ProductRPCConfig
	JWT        JWTConfig
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
	Addr string
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
			Addr: getEnv("GATEWAY_ADDR", ":8080"),
		},
		UserRPC: UserRPCConfig{
			ServiceName:    getEnv("USER_RPC_SERVICE", "meshcart.user"),
			Address:        getEnv("USER_RPC_ADDR", "127.0.0.1:8888"),
			DiscoveryType:  getEnv("USER_RPC_DISCOVERY", "direct"),
			ConsulAddress:  getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			ConnectTimeout: getEnvAsDuration("USER_RPC_CONNECT_TIMEOUT_MS", 500*time.Millisecond),
			RPCTimeout:     getEnvAsDuration("USER_RPC_TIMEOUT_MS", 2*time.Second),
		},
		ProductRPC: ProductRPCConfig{
			ServiceName:    getEnv("PRODUCT_RPC_SERVICE", "meshcart.product"),
			Address:        getEnv("PRODUCT_RPC_ADDR", "127.0.0.1:8889"),
			DiscoveryType:  getEnv("PRODUCT_RPC_DISCOVERY", "direct"),
			ConsulAddress:  getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
			ConnectTimeout: getEnvAsDuration("PRODUCT_RPC_CONNECT_TIMEOUT_MS", 500*time.Millisecond),
			RPCTimeout:     getEnvAsDuration("PRODUCT_RPC_TIMEOUT_MS", 2*time.Second),
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
