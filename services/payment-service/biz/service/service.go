package service

import (
	"time"

	"github.com/bwmarrin/snowflake"

	mqx "meshcart/app/mq"
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
	mqTopic     string
	dispatcher  *mqx.Dispatcher
	nowFunc     func() time.Time
}

func NewPaymentService(repo repository.PaymentRepository, node *snowflake.Node, orderClient orderrpc.Client, mqTopic string) *PaymentService {
	return &PaymentService{
		repo:        repo,
		node:        node,
		orderClient: orderClient,
		mqTopic:     mqTopic,
		nowFunc:     time.Now,
	}
}
