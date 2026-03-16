package handler

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"

	"go.uber.org/zap"
)

func (s *InventoryServiceImpl) CompensateInitSkuStocksSaga(ctx context.Context, request *inventorypb.CompensateInitSkuStocksSagaRequest) (resp *inventorypb.CompensateInitSkuStocksSagaResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("inventory-service", "compensate_init_sku_stocks_saga", code, time.Since(start))
	}()

	bizErr := s.svc.CompensateInitSkuStocksSaga(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("compensate init sku stocks saga failed", zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &inventorypb.CompensateInitSkuStocksSagaResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &inventorypb.CompensateInitSkuStocksSagaResponse{
		Base: &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
