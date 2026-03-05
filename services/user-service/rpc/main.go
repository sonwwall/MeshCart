package main

import (
	"log"

	userservice "meshcart/kitex_gen/meshcart/user/userservice"
	"meshcart/services/user-service/biz/repository"
	bizservice "meshcart/services/user-service/biz/service"
	"meshcart/services/user-service/config"
	"meshcart/services/user-service/dal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	mysqlDB, err := db.NewMySQL(cfg.MySQL.DSN())
	if err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	defer mysqlDB.Close()

	repo := repository.NewMySQLUserRepository(mysqlDB)
	svc := bizservice.NewUserService(repo)

	svr := userservice.NewServer(NewUserServiceImpl(svc))
	if err := svr.Run(); err != nil {
		log.Println(err.Error())
	}
}
