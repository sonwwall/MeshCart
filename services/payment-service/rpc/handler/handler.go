package handler

import "meshcart/services/payment-service/biz/service"

type PaymentServiceImpl struct {
	svc *service.PaymentService
}

func NewPaymentServiceImpl(svc *service.PaymentService) *PaymentServiceImpl {
	return &PaymentServiceImpl{svc: svc}
}
