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

func (s *OrderServiceImpl) GetOrder(ctx context.Context, request *orderpb.GetOrderRequest) (resp *orderpb.GetOrderResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("order-service", "get_order", code, time.Since(start))
	}()

	order, bizErr := s.svc.GetOrder(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("get order failed", zap.Int64("user_id", request.GetUserId()), zap.Int64("order_id", request.GetOrderId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &orderpb.GetOrderResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &orderpb.GetOrderResponse{
		Order: order,
		Base:  &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
