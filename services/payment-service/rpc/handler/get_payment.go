package handler

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	paymentpb "meshcart/kitex_gen/meshcart/payment"

	"go.uber.org/zap"
)

func (s *PaymentServiceImpl) GetPayment(ctx context.Context, request *paymentpb.GetPaymentRequest) (resp *paymentpb.GetPaymentResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("payment-service", "get_payment", code, time.Since(start))
	}()

	payment, bizErr := s.svc.GetPayment(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("get payment failed", zap.Int64("payment_id", request.GetPaymentId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &paymentpb.GetPaymentResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &paymentpb.GetPaymentResponse{
		Payment: payment,
		Base:    &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
