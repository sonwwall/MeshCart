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

func (s *InventoryServiceImpl) BatchGetSkuStock(ctx context.Context, request *inventorypb.BatchGetSkuStockRequest) (resp *inventorypb.BatchGetSkuStockResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("inventory-service", "batch_get_sku_stock", code, time.Since(start))
	}()

	stocks, bizErr := s.svc.BatchGetSkuStock(ctx, request.GetSkuIds())
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("batch get sku stock failed", zap.Int("sku_count", len(request.GetSkuIds())), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &inventorypb.BatchGetSkuStockResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}

	return &inventorypb.BatchGetSkuStockResponse{
		Stocks: stocks,
		Base:   &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
