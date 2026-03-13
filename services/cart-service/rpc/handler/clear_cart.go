package handler

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	cartpb "meshcart/kitex_gen/meshcart/cart"

	"go.uber.org/zap"
)

func (s *CartServiceImpl) ClearCart(ctx context.Context, request *cartpb.ClearCartRequest) (resp *cartpb.ClearCartResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("cart-service", "clear_cart", code, time.Since(start))
	}()

	bizErr := s.svc.ClearCart(ctx, request.GetUserId())
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("clear cart failed", zap.Int64("user_id", request.GetUserId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &cartpb.ClearCartResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &cartpb.ClearCartResponse{Base: &base.BaseResponse{Code: 0, Message: "成功"}}, nil
}
