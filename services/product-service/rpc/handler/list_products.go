package handler

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.uber.org/zap"
)

func (s *ProductServiceImpl) ListProducts(ctx context.Context, request *productpb.ListProductsRequest) (resp *productpb.ListProductsResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "list_products", code, time.Since(start))
	}()

	items, total, bizErr := s.svc.ListProducts(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("list products failed", zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.ListProductsResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.ListProductsResponse{
		Products: items,
		Total:    total,
		Base:     &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
