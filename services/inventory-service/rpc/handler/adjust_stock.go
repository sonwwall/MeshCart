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

func (s *InventoryServiceImpl) AdjustStock(ctx context.Context, request *inventorypb.AdjustStockRequest) (resp *inventorypb.AdjustStockResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("inventory-service", "adjust_stock", code, time.Since(start))
	}()

	stock, bizErr := s.svc.AdjustStock(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("adjust stock failed", zap.Int64("sku_id", request.GetSkuId()), zap.Int64("total_stock", request.GetTotalStock()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &inventorypb.AdjustStockResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &inventorypb.AdjustStockResponse{
		Stock: stock,
		Base:  &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
