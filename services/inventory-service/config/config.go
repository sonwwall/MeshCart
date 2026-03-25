package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Migration MigrationConfig `mapstructure:"migration"`
	Snowflake SnowflakeConfig `mapstructure:"snowflake"`
	DBPool    DBPoolConfig    `mapstructure:"db_pool"`
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

type DBPoolConfig struct {
	MaxOpenConns    int `mapstructure:"max_open_conns"`
	MaxIdleConns    int `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int `mapstructure:"conn_max_lifetime_minutes"`
	StatsIntervalMS int `mapstructure:"stats_interval_ms"`
}

type TimeoutConfig struct {
	DBQueryMS int `mapstructure:"db_query_ms"`
}

func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s&parseTime=%t&loc=%s",
		m.Username, m.Password, m.Address, m.Database, m.Charset, m.ParseTime, m.Loc)
}

func Load() (Config, error) {
	path := getEnv("INVENTORY_SERVICE_CONFIG", "services/inventory-service/config/inventory-service.local.yaml")

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("INVENTORY_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("mysql.address", "127.0.0.1:3306")
	v.SetDefault("mysql.username", "root")
	v.SetDefault("mysql.password", "root")
	v.SetDefault("mysql.database", "meshcart_inventory")
	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.parse_time", true)
	v.SetDefault("mysql.loc", "Local")
	v.SetDefault("migration.enabled", true)
	v.SetDefault("migration.source", "file://services/inventory-service/migrations")
	v.SetDefault("snowflake.node", 4)
	v.SetDefault("db_pool.max_open_conns", 60)
	v.SetDefault("db_pool.max_idle_conns", 20)
	v.SetDefault("db_pool.conn_max_lifetime_minutes", 30)
	v.SetDefault("db_pool.stats_interval_ms", 5000)
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

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
