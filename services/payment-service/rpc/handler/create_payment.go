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

func (s *PaymentServiceImpl) CreatePayment(ctx context.Context, request *paymentpb.CreatePaymentRequest) (resp *paymentpb.CreatePaymentResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("payment-service", "create_payment", code, time.Since(start))
	}()

	payment, bizErr := s.svc.CreatePayment(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("create payment failed", zap.Int64("order_id", request.GetOrderId()), zap.Int64("user_id", request.GetUserId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &paymentpb.CreatePaymentResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &paymentpb.CreatePaymentResponse{
		Payment: payment,
		Base:    &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
