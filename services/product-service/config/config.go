package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Migration MigrationConfig `mapstructure:"migration"`
	Snowflake SnowflakeConfig `mapstructure:"snowflake"`
	Timeout   TimeoutConfig   `mapstructure:"timeout"`
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

type ApolloLoader interface {
	Load() (Config, error)
}

var apolloLoader ApolloLoader

func RegisterApolloLoader(loader ApolloLoader) {
	apolloLoader = loader
}

func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s&parseTime=%t&loc=%s",
		m.Username,
		m.Password,
		m.Address,
		m.Database,
		m.Charset,
		m.ParseTime,
		m.Loc,
	)
}

func Load() (Config, error) {
	source := strings.ToLower(getEnv("PRODUCT_SERVICE_CONFIG_SOURCE", "file"))
	if source == "apollo" {
		return loadFromApollo()
	}

	path := getEnv("PRODUCT_SERVICE_CONFIG", "services/product-service/config/product-service.local.yaml")

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetEnvPrefix("PRODUCT_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("mysql.address", "127.0.0.1:3306")
	v.SetDefault("mysql.username", "root")
	v.SetDefault("mysql.password", "root")
	v.SetDefault("mysql.database", "meshcart_product")
	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.parse_time", true)
	v.SetDefault("mysql.loc", "Local")
	v.SetDefault("migration.enabled", true)
	v.SetDefault("migration.source", "file://services/product-service/migrations")
	v.SetDefault("snowflake.node", 2)
	v.SetDefault("timeout.db_query_ms", 1500)

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
