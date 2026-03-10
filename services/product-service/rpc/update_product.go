package main

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.uber.org/zap"
)

func (s *ProductServiceImpl) UpdateProduct(ctx context.Context, request *productpb.UpdateProductRequest) (resp *productpb.UpdateProductResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "update_product", code, time.Since(start))
	}()

	bizErr := s.svc.UpdateProduct(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("update product failed", zap.Int64("product_id", request.ProductId), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.UpdateProductResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.UpdateProductResponse{Base: &base.BaseResponse{Code: 0, Message: "成功"}}, nil
}
