package main

import (
	"os"

	logx "meshcart/app/log"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
	"meshcart/services/user-service/biz/repository"
	bizservice "meshcart/services/user-service/biz/service"
	"meshcart/services/user-service/config"
	"meshcart/services/user-service/dal/db"

	"go.uber.org/zap"
)

func main() {
	if err := logx.Init(logx.Config{
		Service: "user-service",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
	}); err != nil {
		panic(err)
	}
	defer logx.Sync()

	cfg, err := config.Load()
	if err != nil {
		logx.L(nil).Fatal("load config failed", zap.Error(err))
	}

	mysqlDB, err := db.NewMySQL(cfg.MySQL.DSN())
	if err != nil {
		logx.L(nil).Fatal("init mysql failed", zap.Error(err))
	}
	defer mysqlDB.Close()

	repo := repository.NewMySQLUserRepository(mysqlDB)
	svc := bizservice.NewUserService(repo)

	svr := userservice.NewServer(NewUserServiceImpl(svc))
	logx.L(nil).Info("user-service starting")
	if err := svr.Run(); err != nil {
		logx.L(nil).Error("user-service stopped with error", zap.Error(err))
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
