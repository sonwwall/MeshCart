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

func (s *PaymentServiceImpl) ListPaymentsByOrder(ctx context.Context, request *paymentpb.ListPaymentsByOrderRequest) (resp *paymentpb.ListPaymentsByOrderResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("payment-service", "list_payments_by_order", code, time.Since(start))
	}()

	payments, bizErr := s.svc.ListPaymentsByOrder(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("list payments by order failed", zap.Int64("order_id", request.GetOrderId()), zap.Int64("user_id", request.GetUserId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &paymentpb.ListPaymentsByOrderResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &paymentpb.ListPaymentsByOrderResponse{
		Payments: payments,
		Base:     &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
