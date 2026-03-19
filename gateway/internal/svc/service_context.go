package svc

import (
	"meshcart/gateway/config"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/tx"
	cartrpc "meshcart/gateway/rpc/cart"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	orderrpc "meshcart/gateway/rpc/order"
	paymentrpc "meshcart/gateway/rpc/payment"
	productrpc "meshcart/gateway/rpc/product"
	userrpc "meshcart/gateway/rpc/user"
	"sync/atomic"

	jwtmw "github.com/hertz-contrib/jwt"
)

type ServiceContext struct {
	Config                   config.Config
	UserClient               userrpc.Client
	CartClient               cartrpc.Client
	OrderClient              orderrpc.Client
	PaymentClient            paymentrpc.Client
	ProductClient            productrpc.Client
	InventoryClient          inventoryrpc.Client
	ProductCreateCoordinator tx.ProductCreateCoordinator
	AccessControl            *authz.AccessController
	JWT                      *jwtmw.HertzJWTMiddleware
	RateLimiter              *middleware.RateLimitStore
	Draining                 *atomic.Bool
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

	orderClient, err := orderrpc.NewClient(
		cfg.OrderRPC.ServiceName,
		cfg.OrderRPC.Address,
		cfg.OrderRPC.DiscoveryType,
		cfg.OrderRPC.ConsulAddress,
		cfg.OrderRPC.ConnectTimeout,
		cfg.OrderRPC.RPCTimeout,
	)
	if err != nil {
		panic(err)
	}

	paymentClient, err := paymentrpc.NewClient(
		cfg.PaymentRPC.ServiceName,
		cfg.PaymentRPC.Address,
		cfg.PaymentRPC.DiscoveryType,
		cfg.PaymentRPC.ConsulAddress,
		cfg.PaymentRPC.ConnectTimeout,
		cfg.PaymentRPC.RPCTimeout,
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

	inventoryClient, err := inventoryrpc.NewClient(
		cfg.InventoryRPC.ServiceName,
		cfg.InventoryRPC.Address,
		cfg.InventoryRPC.DiscoveryType,
		cfg.InventoryRPC.ConsulAddress,
		cfg.InventoryRPC.ConnectTimeout,
		cfg.InventoryRPC.RPCTimeout,
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
		Config:                   cfg,
		UserClient:               userClient,
		CartClient:               cartClient,
		OrderClient:              orderClient,
		PaymentClient:            paymentClient,
		ProductClient:            productClient,
		InventoryClient:          inventoryClient,
		ProductCreateCoordinator: tx.NewProductCreateCoordinator(cfg.DTM, productClient, inventoryClient),
		AccessControl:            accessController,
		JWT:                      jwtMiddleware,
		RateLimiter:              middleware.NewRateLimitStore(cfg.RateLimit),
		Draining:                 &atomic.Bool{},
	}
}
