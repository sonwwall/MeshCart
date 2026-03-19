package service

import (
	"context"

	"meshcart/app/common"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
)

func (s *PaymentService) ListPaymentsByOrder(ctx context.Context, req *paymentpb.ListPaymentsByOrderRequest) ([]*paymentpb.Payment, *common.BizError) {
	if req == nil || req.GetOrderId() <= 0 || req.GetUserId() <= 0 {
		return nil, common.ErrInvalidParam
	}
	payments, err := s.repo.ListByOrderID(ctx, req.GetOrderId(), req.GetUserId())
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCPayments(payments), nil
}
