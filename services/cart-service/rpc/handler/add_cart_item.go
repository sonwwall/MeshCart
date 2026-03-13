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

func (s *CartServiceImpl) AddCartItem(ctx context.Context, request *cartpb.AddCartItemRequest) (resp *cartpb.AddCartItemResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("cart-service", "add_cart_item", code, time.Since(start))
	}()

	item, bizErr := s.svc.AddCartItem(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("add cart item failed", zap.Int64("user_id", request.GetUserId()), zap.Int64("sku_id", request.GetSkuId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &cartpb.AddCartItemResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &cartpb.AddCartItemResponse{
		Item: item,
		Base: &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
