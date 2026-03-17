package service

import (
	"github.com/bwmarrin/snowflake"

	"meshcart/services/order-service/biz/repository"
)

const (
	OrderStatusPending   int32 = 1
	OrderStatusReserved  int32 = 2
	OrderStatusPaid      int32 = 3
	OrderStatusCancelled int32 = 4
	OrderStatusClosed    int32 = 5
)

type OrderService struct {
	repo repository.OrderRepository
	node *snowflake.Node
}

func NewOrderService(repo repository.OrderRepository, node *snowflake.Node) *OrderService {
	return &OrderService{repo: repo, node: node}
}
