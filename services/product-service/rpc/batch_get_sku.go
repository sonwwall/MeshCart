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

func (s *ProductServiceImpl) BatchGetSku(ctx context.Context, request *productpb.BatchGetSkuRequest) (resp *productpb.BatchGetSkuResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "batch_get_sku", code, time.Since(start))
	}()

	skus, bizErr := s.svc.BatchGetSKU(ctx, request.SkuIds)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("batch get sku failed", zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.BatchGetSkuResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.BatchGetSkuResponse{
		Skus: skus,
		Base: &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
