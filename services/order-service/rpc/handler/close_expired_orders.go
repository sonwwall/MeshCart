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

func (s *OrderServiceImpl) CloseExpiredOrders(ctx context.Context, request *orderpb.CloseExpiredOrdersRequest) (resp *orderpb.CloseExpiredOrdersResponse, err error) {
	start := time.Now()
	code := int32(0)
	limit := int32(0)
	if request != nil {
		limit = request.GetLimit()
	}
	defer func() {
		metricsx.ObserveRPC("order-service", "close_expired_orders", code, time.Since(start))
	}()

	orderIDs, bizErr := s.svc.CloseExpiredOrders(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("close expired orders failed", zap.Int32("limit", limit), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &orderpb.CloseExpiredOrdersResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &orderpb.CloseExpiredOrdersResponse{
		ClosedCount: int32(len(orderIDs)),
		OrderIds:    orderIDs,
		Base:        &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
