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

func (s *ProductServiceImpl) ChangeProductStatus(ctx context.Context, request *productpb.ChangeProductStatusRequest) (resp *productpb.ChangeProductStatusResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "change_product_status", code, time.Since(start))
	}()

	bizErr := s.svc.ChangeProductStatus(ctx, request.ProductId, request.Status)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("change product status failed", zap.Int64("product_id", request.ProductId), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.ChangeProductStatusResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.ChangeProductStatusResponse{Base: &base.BaseResponse{Code: 0, Message: "成功"}}, nil
}
