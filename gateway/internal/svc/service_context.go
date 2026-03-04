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
	return &ServiceContext{
		Config:     cfg,
		UserClient: userrpc.NewMockClient(),
	}
}
