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

func (s *ProductServiceImpl) GetProductDetail(ctx context.Context, request *productpb.GetProductDetailRequest) (resp *productpb.GetProductDetailResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "get_product_detail", code, time.Since(start))
	}()

	productDetail, bizErr := s.svc.GetProductDetail(ctx, request.ProductId)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("get product detail failed", zap.Int64("product_id", request.ProductId), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.GetProductDetailResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.GetProductDetailResponse{
		Product: productDetail,
		Base:    &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
