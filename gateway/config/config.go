package config

import "os"

type Config struct {
	Server  ServerConfig
	UserRPC UserRPCConfig
}

type ServerConfig struct {
	Addr string
}

type UserRPCConfig struct {
	Address string
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Addr: getEnv("GATEWAY_ADDR", ":8080"),
		},
		UserRPC: UserRPCConfig{
			Address: getEnv("USER_RPC_ADDR", "127.0.0.1:8888"),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
