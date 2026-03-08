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
	source := strings.ToLower(getEnv("USER_SERVICE_CONFIG_SOURCE", "file"))
	if source == "apollo" {
		return loadFromApollo()
	}

	path := getEnv("USER_SERVICE_CONFIG", "services/user-service/config/user-service.local.yaml")

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetEnvPrefix("USER_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("mysql.address", "127.0.0.1:3306")
	v.SetDefault("mysql.username", "root")
	v.SetDefault("mysql.password", "root")
	v.SetDefault("mysql.database", "meshcart_user")
	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.parse_time", true)
	v.SetDefault("mysql.loc", "Local")
	v.SetDefault("migration.enabled", true)
	v.SetDefault("migration.source", "file://services/user-service/migrations")
	v.SetDefault("snowflake.node", 1)

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

	cfg, err := apolloLoader.Load()
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
