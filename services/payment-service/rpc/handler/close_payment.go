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

func (s *PaymentServiceImpl) ClosePayment(ctx context.Context, request *paymentpb.ClosePaymentRequest) (resp *paymentpb.ClosePaymentResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("payment-service", "close_payment", code, time.Since(start))
	}()

	payment, bizErr := s.svc.ClosePayment(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("close payment failed",
			zap.Int64("payment_id", request.GetPaymentId()),
			zap.Int64("user_id", request.GetUserId()),
			zap.String("reason", request.GetReason()),
			zap.String("request_id", request.GetRequestId()),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &paymentpb.ClosePaymentResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &paymentpb.ClosePaymentResponse{
		Payment: payment,
		Base:    &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
