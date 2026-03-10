package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App        AppConfig
	Log        LogConfig
	Telemetry  TelemetryConfig
	Metrics    MetricsConfig
	Server     ServerConfig
	UserRPC    UserRPCConfig
	ProductRPC ProductRPCConfig
	Admin      AdminConfig
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
	ServiceName   string
	Address       string
	DiscoveryType string
	ConsulAddress string
}

type ProductRPCConfig struct {
	ServiceName   string
	Address       string
	DiscoveryType string
	ConsulAddress string
}

type AdminConfig struct {
	UserIDs []int64
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
			ServiceName:   getEnv("USER_RPC_SERVICE", "meshcart.user"),
			Address:       getEnv("USER_RPC_ADDR", "127.0.0.1:8888"),
			DiscoveryType: getEnv("USER_RPC_DISCOVERY", "direct"),
			ConsulAddress: getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
		},
		ProductRPC: ProductRPCConfig{
			ServiceName:   getEnv("PRODUCT_RPC_SERVICE", "meshcart.product"),
			Address:       getEnv("PRODUCT_RPC_ADDR", "127.0.0.1:8889"),
			DiscoveryType: getEnv("PRODUCT_RPC_DISCOVERY", "direct"),
			ConsulAddress: getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
		},
		Admin: AdminConfig{
			UserIDs: getEnvAsInt64Slice("ADMIN_USER_IDS"),
		},
		JWT: JWTConfig{
			Secret:            getEnv("JWT_SECRET", "meshcart-dev-secret-change-me"),
			Issuer:            getEnv("JWT_ISSUER", "meshcart.gateway"),
			TimeoutMinutes:    getEnvAsInt("JWT_TIMEOUT_MINUTES", 120),
			MaxRefreshMinutes: getEnvAsInt("JWT_MAX_REFRESH_MINUTES", 720),
		},
	}
}

func getEnvAsInt64Slice(key string) []int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			continue
		}
		result = append(result, parsed)
	}
	return result
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
