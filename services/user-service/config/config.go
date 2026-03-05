package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MySQL MySQLConfig `yaml:"mysql"`
}

type MySQLConfig struct {
	Address   string `yaml:"address"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Database  string `yaml:"database"`
	Charset   string `yaml:"charset"`
	ParseTime bool   `yaml:"parse_time"`
	Loc       string `yaml:"loc"`
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
	path := getEnv("USER_SERVICE_CONFIG", "services/user-service/config/user-service.local.yaml")

	cfg := Config{
		MySQL: MySQLConfig{
			Address:   "127.0.0.1:3306",
			Username:  "root",
			Password:  "root",
			Database:  "meshcart_user",
			Charset:   "utf8mb4",
			ParseTime: true,
			Loc:       "Local",
		},
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
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
