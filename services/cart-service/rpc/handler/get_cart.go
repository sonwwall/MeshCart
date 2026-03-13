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

func (s *CartServiceImpl) GetCart(ctx context.Context, request *cartpb.GetCartRequest) (resp *cartpb.GetCartResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("cart-service", "get_cart", code, time.Since(start))
	}()

	items, bizErr := s.svc.GetCart(ctx, request.GetUserId())
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("get cart failed", zap.Int64("user_id", request.GetUserId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &cartpb.GetCartResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &cartpb.GetCartResponse{
		Items: items,
		Base:  &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
