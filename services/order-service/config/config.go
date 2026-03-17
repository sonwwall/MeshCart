package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	MySQL        MySQLConfig         `mapstructure:"mysql"`
	Migration    MigrationConfig     `mapstructure:"migration"`
	Snowflake    SnowflakeConfig     `mapstructure:"snowflake"`
	Timeout      TimeoutConfig       `mapstructure:"timeout"`
	ProductRPC   DownstreamRPCConfig `mapstructure:"product_rpc"`
	InventoryRPC DownstreamRPCConfig `mapstructure:"inventory_rpc"`
}

type MySQLConfig struct {
	Address   string `mapstructure:"address"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	Database  string `mapstructure:"database"`
	Charset   string `mapstructure:"charset"`
	ParseTime bool   `mapstructure:"parse_time"`
	Loc       string `mapstructure:"loc"`
}

type MigrationConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Source  string `mapstructure:"source"`
}

type SnowflakeConfig struct {
	Node int64 `mapstructure:"node"`
}

type TimeoutConfig struct {
	DBQueryMS int `mapstructure:"db_query_ms"`
}

type DownstreamRPCConfig struct {
	ServiceName      string `mapstructure:"service_name"`
	HostPort         string `mapstructure:"host_port"`
	DiscoveryType    string `mapstructure:"discovery_type"`
	ConsulAddress    string `mapstructure:"consul_address"`
	ConnectTimeoutMS int    `mapstructure:"connect_timeout_ms"`
	RPCTimeoutMS     int    `mapstructure:"rpc_timeout_ms"`
}

type ApolloLoader interface {
	Load() (Config, error)
}

var apolloLoader ApolloLoader

func RegisterApolloLoader(loader ApolloLoader) {
	apolloLoader = loader
}

func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s&parseTime=%t&loc=%s",
		m.Username, m.Password, m.Address, m.Database, m.Charset, m.ParseTime, m.Loc)
}

func Load() (Config, error) {
	source := strings.ToLower(getEnv("ORDER_SERVICE_CONFIG_SOURCE", "file"))
	if source == "apollo" {
		return loadFromApollo()
	}

	path := getEnv("ORDER_SERVICE_CONFIG", "services/order-service/config/order-service.local.yaml")

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("ORDER_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("mysql.address", "127.0.0.1:3306")
	v.SetDefault("mysql.username", "root")
	v.SetDefault("mysql.password", "root")
	v.SetDefault("mysql.database", "meshcart_order")
	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.parse_time", true)
	v.SetDefault("mysql.loc", "Local")
	v.SetDefault("migration.enabled", true)
	v.SetDefault("migration.source", "file://services/order-service/migrations")
	v.SetDefault("snowflake.node", 5)
	v.SetDefault("timeout.db_query_ms", 1500)
	v.SetDefault("product_rpc.service_name", "meshcart.product")
	v.SetDefault("product_rpc.host_port", "127.0.0.1:8890")
	v.SetDefault("product_rpc.discovery_type", "consul")
	v.SetDefault("product_rpc.consul_address", "127.0.0.1:8500")
	v.SetDefault("product_rpc.connect_timeout_ms", 500)
	v.SetDefault("product_rpc.rpc_timeout_ms", 2000)
	v.SetDefault("inventory_rpc.service_name", "meshcart.inventory")
	v.SetDefault("inventory_rpc.host_port", "127.0.0.1:8891")
	v.SetDefault("inventory_rpc.discovery_type", "consul")
	v.SetDefault("inventory_rpc.consul_address", "127.0.0.1:8500")
	v.SetDefault("inventory_rpc.connect_timeout_ms", 500)
	v.SetDefault("inventory_rpc.rpc_timeout_ms", 2000)

	if err := v.ReadInConfig(); err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadFromApollo() (Config, error) {
	if apolloLoader == nil {
		return Config{}, errors.New("apollo config source selected but no apollo loader registered")
	}
	return apolloLoader.Load()
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
