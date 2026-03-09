package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server  ServerConfig
	UserRPC UserRPCConfig
	JWT     JWTConfig
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

type JWTConfig struct {
	Secret            string
	Issuer            string
	TimeoutMinutes    int
	MaxRefreshMinutes int
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Addr: getEnv("GATEWAY_ADDR", ":8080"),
		},
		UserRPC: UserRPCConfig{
			ServiceName:   getEnv("USER_RPC_SERVICE", "meshcart.user"),
			Address:       getEnv("USER_RPC_ADDR", "127.0.0.1:8888"),
			DiscoveryType: getEnv("USER_RPC_DISCOVERY", "direct"),
			ConsulAddress: getEnv("CONSUL_ADDR", "127.0.0.1:8500"),
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
