package svc

import (
	"meshcart/gateway/config"
	"meshcart/gateway/internal/middleware"
	productrpc "meshcart/gateway/rpc/product"
	userrpc "meshcart/gateway/rpc/user"

	jwtmw "github.com/hertz-contrib/jwt"
)

type ServiceContext struct {
	Config        config.Config
	UserClient    userrpc.Client
	ProductClient productrpc.Client
	JWT           *jwtmw.HertzJWTMiddleware
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

	productClient, err := productrpc.NewClient(
		cfg.ProductRPC.ServiceName,
		cfg.ProductRPC.Address,
		cfg.ProductRPC.DiscoveryType,
		cfg.ProductRPC.ConsulAddress,
	)
	if err != nil {
		panic(err)
	}

	jwtMiddleware, err := middleware.NewJWT(cfg.JWT)
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:        cfg,
		UserClient:    userClient,
		ProductClient: productClient,
		JWT:           jwtMiddleware,
	}
}
