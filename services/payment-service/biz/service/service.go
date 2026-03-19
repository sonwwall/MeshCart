package service

import (
	"time"

	"github.com/bwmarrin/snowflake"

	"meshcart/services/payment-service/biz/repository"
	orderrpc "meshcart/services/payment-service/rpcclient/order"
)

const (
	PaymentStatusPending   int32 = 1
	PaymentStatusSucceeded int32 = 2
	PaymentStatusFailed    int32 = 3
	PaymentStatusClosed    int32 = 4
)

type PaymentService struct {
	repo        repository.PaymentRepository
	node        *snowflake.Node
	orderClient orderrpc.Client
	nowFunc     func() time.Time
}

func NewPaymentService(repo repository.PaymentRepository, node *snowflake.Node, orderClient orderrpc.Client) *PaymentService {
	return &PaymentService{
		repo:        repo,
		node:        node,
		orderClient: orderClient,
		nowFunc:     time.Now,
	}
}
