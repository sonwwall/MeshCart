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

func (s *ProductServiceImpl) BatchGetProducts(ctx context.Context, request *productpb.BatchGetProductsRequest) (resp *productpb.BatchGetProductsResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "batch_get_products", code, time.Since(start))
	}()

	products, bizErr := s.svc.BatchGetProducts(ctx, request.GetProductIds())
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("batch get products failed", zap.Int64s("product_ids", request.GetProductIds()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.BatchGetProductsResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.BatchGetProductsResponse{
		Products: products,
		Base:     &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
