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

func (s *ProductServiceImpl) CompensateCreateProductSaga(ctx context.Context, request *productpb.CompensateCreateProductSagaRequest) (resp *productpb.CompensateCreateProductSagaResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("product-service", "compensate_create_product_saga", code, time.Since(start))
	}()

	bizErr := s.svc.CompensateCreateProductSaga(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("compensate create product saga failed", zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &productpb.CompensateCreateProductSagaResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &productpb.CompensateCreateProductSagaResponse{
		Base: &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
