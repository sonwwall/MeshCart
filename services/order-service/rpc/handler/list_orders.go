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

func (s *OrderServiceImpl) ListOrders(ctx context.Context, request *orderpb.ListOrdersRequest) (resp *orderpb.ListOrdersResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("order-service", "list_orders", code, time.Since(start))
	}()

	orders, total, bizErr := s.svc.ListOrders(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("list orders failed", zap.Int64("user_id", request.GetUserId()), zap.Int32("page", request.GetPage()), zap.Int32("page_size", request.GetPageSize()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &orderpb.ListOrdersResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &orderpb.ListOrdersResponse{
		Orders: orders,
		Total:  total,
		Base:   &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
