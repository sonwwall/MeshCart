package service

import (
	"context"

	"meshcart/app/common"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
)

func (s *PaymentService) GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*paymentpb.Payment, *common.BizError) {
	if req == nil || req.GetPaymentId() <= 0 || req.GetUserId() <= 0 {
		return nil, common.ErrInvalidParam
	}
	payment, err := s.repo.GetByPaymentIDUser(ctx, req.GetPaymentId(), req.GetUserId())
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCPayment(payment), nil
}
