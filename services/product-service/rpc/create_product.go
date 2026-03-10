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

func (s *ProductServiceImpl) CreateProduct(ctx context.Context, request *productpb.CreateProductRequest) (resp *productpb.CreateProductResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "create_product", code, time.Since(start))
	}()

	productID, bizErr := s.svc.CreateProduct(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("create product failed", zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.CreateProductResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.CreateProductResponse{
		ProductId: productID,
		Base:      &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
