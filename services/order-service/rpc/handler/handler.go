package handler

import "meshcart/services/order-service/biz/service"

type OrderServiceImpl struct {
	svc *service.OrderService
}

func NewOrderServiceImpl(svc *service.OrderService) *OrderServiceImpl {
	return &OrderServiceImpl{svc: svc}
}
