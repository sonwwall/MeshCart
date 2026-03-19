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

func (s *PaymentServiceImpl) ConfirmPaymentSuccess(ctx context.Context, request *paymentpb.ConfirmPaymentSuccessRequest) (resp *paymentpb.ConfirmPaymentSuccessResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("payment-service", "confirm_payment_success", code, time.Since(start))
	}()

	payment, bizErr := s.svc.ConfirmPaymentSuccess(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("confirm payment success failed",
			zap.Int64("payment_id", request.GetPaymentId()),
			zap.String("payment_method", request.GetPaymentMethod()),
			zap.String("payment_trade_no", request.GetPaymentTradeNo()),
			zap.String("request_id", request.GetRequestId()),
			zap.Int64("paid_at", request.GetPaidAt()),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &paymentpb.ConfirmPaymentSuccessResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &paymentpb.ConfirmPaymentSuccessResponse{
		Payment: payment,
		Base:    &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
