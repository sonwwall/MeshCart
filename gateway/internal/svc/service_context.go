package svc

import (
	"meshcart/gateway/config"
	userrpc "meshcart/gateway/rpc/user"
)

type ServiceContext struct {
	Config     config.Config
	UserClient userrpc.Client
}

func NewServiceContext(cfg config.Config) *ServiceContext {
	userClient, err := userrpc.NewClient(cfg.UserRPC.ServiceName, cfg.UserRPC.Address)
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:     cfg,
		UserClient: userClient,
	}
}
