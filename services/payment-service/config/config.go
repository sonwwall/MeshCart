package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	MySQL     MySQLConfig         `mapstructure:"mysql"`
	Migration MigrationConfig     `mapstructure:"migration"`
	Snowflake SnowflakeConfig     `mapstructure:"snowflake"`
	Timeout   TimeoutConfig       `mapstructure:"timeout"`
	OrderRPC  DownstreamRPCConfig `mapstructure:"order_rpc"`
	MQ        MQConfig            `mapstructure:"mq"`
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

type MQConfig struct {
	Enabled                       bool     `mapstructure:"enabled"`
	Brokers                       []string `mapstructure:"brokers"`
	PaymentSucceededTopic         string   `mapstructure:"payment_succeeded_topic"`
	PaymentSucceededConsumerGroup string   `mapstructure:"payment_succeeded_consumer_group"`
	DispatcherIntervalMS          int      `mapstructure:"dispatcher_interval_ms"`
	DispatcherBatchSize           int      `mapstructure:"dispatcher_batch_size"`
}

type DownstreamRPCConfig struct {
	ServiceName      string `mapstructure:"service_name"`
	HostPort         string `mapstructure:"host_port"`
	DiscoveryType    string `mapstructure:"discovery_type"`
	ConsulAddress    string `mapstructure:"consul_address"`
	ConnectTimeoutMS int    `mapstructure:"connect_timeout_ms"`
	RPCTimeoutMS     int    `mapstructure:"rpc_timeout_ms"`
}

func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s&parseTime=%t&loc=%s",
		m.Username, m.Password, m.Address, m.Database, m.Charset, m.ParseTime, m.Loc)
}

func Load() (Config, error) {
	path := getEnv("PAYMENT_SERVICE_CONFIG", "services/payment-service/config/payment-service.local.yaml")

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("PAYMENT_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("mysql.address", "127.0.0.1:3306")
	v.SetDefault("mysql.username", "root")
	v.SetDefault("mysql.password", "root")
	v.SetDefault("mysql.database", "meshcart_payment")
	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.parse_time", true)
	v.SetDefault("mysql.loc", "Local")
	v.SetDefault("migration.enabled", true)
	v.SetDefault("migration.source", "file://services/payment-service/migrations")
	v.SetDefault("snowflake.node", 6)
	v.SetDefault("timeout.db_query_ms", 1500)
	v.SetDefault("order_rpc.service_name", "meshcart.order")
	v.SetDefault("order_rpc.host_port", "127.0.0.1:8892")
	v.SetDefault("order_rpc.discovery_type", "consul")
	v.SetDefault("order_rpc.consul_address", "127.0.0.1:8500")
	v.SetDefault("order_rpc.connect_timeout_ms", 500)
	v.SetDefault("order_rpc.rpc_timeout_ms", 2000)
	v.SetDefault("mq.enabled", false)
	v.SetDefault("mq.brokers", []string{"127.0.0.1:9092"})
	v.SetDefault("mq.payment_succeeded_topic", "payment.events")
	v.SetDefault("mq.payment_succeeded_consumer_group", "meshcart.payment.succeeded.smoke")
	v.SetDefault("mq.dispatcher_interval_ms", 1000)
	v.SetDefault("mq.dispatcher_batch_size", 100)

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
