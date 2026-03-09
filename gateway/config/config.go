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
	ServiceName   string
	Address       string
	DiscoveryType string
	ConsulAddress string
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
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
