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

func (s *InventoryServiceImpl) GetSkuStock(ctx context.Context, request *inventorypb.GetSkuStockRequest) (resp *inventorypb.GetSkuStockResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("inventory-service", "get_sku_stock", code, time.Since(start))
	}()

	stock, bizErr := s.svc.GetSkuStock(ctx, request.GetSkuId())
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("get sku stock failed", zap.Int64("sku_id", request.GetSkuId()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &inventorypb.GetSkuStockResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}

	return &inventorypb.GetSkuStockResponse{
		Stock: stock,
		Base:  &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
