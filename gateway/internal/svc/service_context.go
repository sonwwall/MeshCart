package svc

import (
	"meshcart/gateway/config"
	"meshcart/gateway/internal/middleware"
	userrpc "meshcart/gateway/rpc/user"

	jwtmw "github.com/hertz-contrib/jwt"
)

type ServiceContext struct {
	Config     config.Config
	UserClient userrpc.Client
	JWT        *jwtmw.HertzJWTMiddleware
}

func NewServiceContext(cfg config.Config) *ServiceContext {
	userClient, err := userrpc.NewClient(
		cfg.UserRPC.ServiceName,
		cfg.UserRPC.Address,
		cfg.UserRPC.DiscoveryType,
		cfg.UserRPC.ConsulAddress,
	)
	if err != nil {
		panic(err)
	}

	jwtMiddleware, err := middleware.NewJWT(cfg.JWT)
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:     cfg,
		UserClient: userClient,
		JWT:        jwtMiddleware,
	}
}
