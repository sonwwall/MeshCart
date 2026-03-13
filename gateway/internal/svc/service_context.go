package svc

import (
	"meshcart/gateway/config"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	cartrpc "meshcart/gateway/rpc/cart"
	productrpc "meshcart/gateway/rpc/product"
	userrpc "meshcart/gateway/rpc/user"
	"sync/atomic"

	jwtmw "github.com/hertz-contrib/jwt"
)

type ServiceContext struct {
	Config        config.Config
	UserClient    userrpc.Client
	CartClient    cartrpc.Client
	ProductClient productrpc.Client
	AccessControl *authz.AccessController
	JWT           *jwtmw.HertzJWTMiddleware
	RateLimiter   *middleware.RateLimitStore
	Draining      *atomic.Bool
}

func NewServiceContext(cfg config.Config) *ServiceContext {
	userClient, err := userrpc.NewClient(
		cfg.UserRPC.ServiceName,
		cfg.UserRPC.Address,
		cfg.UserRPC.DiscoveryType,
		cfg.UserRPC.ConsulAddress,
		cfg.UserRPC.ConnectTimeout,
		cfg.UserRPC.RPCTimeout,
	)
	if err != nil {
		panic(err)
	}

	cartClient, err := cartrpc.NewClient(
		cfg.CartRPC.ServiceName,
		cfg.CartRPC.Address,
		cfg.CartRPC.DiscoveryType,
		cfg.CartRPC.ConsulAddress,
		cfg.CartRPC.ConnectTimeout,
		cfg.CartRPC.RPCTimeout,
	)
	if err != nil {
		panic(err)
	}

	productClient, err := productrpc.NewClient(
		cfg.ProductRPC.ServiceName,
		cfg.ProductRPC.Address,
		cfg.ProductRPC.DiscoveryType,
		cfg.ProductRPC.ConsulAddress,
		cfg.ProductRPC.ConnectTimeout,
		cfg.ProductRPC.RPCTimeout,
	)
	if err != nil {
		panic(err)
	}

	jwtMiddleware, err := middleware.NewJWT(cfg.JWT)
	if err != nil {
		panic(err)
	}

	accessController, err := authz.NewAccessController()
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:        cfg,
		UserClient:    userClient,
		CartClient:    cartClient,
		ProductClient: productClient,
		AccessControl: accessController,
		JWT:           jwtMiddleware,
		RateLimiter:   middleware.NewRateLimitStore(cfg.RateLimit),
		Draining:      &atomic.Bool{},
	}
}
