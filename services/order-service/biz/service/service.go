package service

import (
	"time"

	"github.com/bwmarrin/snowflake"

	"meshcart/services/order-service/biz/repository"
	inventoryrpc "meshcart/services/order-service/rpcclient/inventory"
	productrpc "meshcart/services/order-service/rpcclient/product"
)

const (
	OrderStatusPending   int32 = 1
	OrderStatusReserved  int32 = 2
	OrderStatusPaid      int32 = 3
	OrderStatusCancelled int32 = 4
	OrderStatusClosed    int32 = 5
)

type OrderService struct {
	repo            repository.OrderRepository
	node            *snowflake.Node
	productClient   productrpc.Client
	inventoryClient inventoryrpc.Client
	nowFunc         func() time.Time
}

func NewOrderService(repo repository.OrderRepository, node *snowflake.Node, productClient productrpc.Client, inventoryClient inventoryrpc.Client) *OrderService {
	return &OrderService{
		repo:            repo,
		node:            node,
		productClient:   productClient,
		inventoryClient: inventoryClient,
		nowFunc:         time.Now,
	}
}
