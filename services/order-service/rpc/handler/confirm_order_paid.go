package handler

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	orderpb "meshcart/kitex_gen/meshcart/order"

	"go.uber.org/zap"
)

func (s *OrderServiceImpl) ConfirmOrderPaid(ctx context.Context, request *orderpb.ConfirmOrderPaidRequest) (resp *orderpb.ConfirmOrderPaidResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("order-service", "confirm_order_paid", code, time.Since(start))
	}()

	order, bizErr := s.svc.ConfirmOrderPaid(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("confirm order paid failed", zap.Int64("order_id", request.GetOrderId()), zap.String("payment_id", request.GetPaymentId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &orderpb.ConfirmOrderPaidResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &orderpb.ConfirmOrderPaidResponse{
		Order: order,
		Base:  &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
