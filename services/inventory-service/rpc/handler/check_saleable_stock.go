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

func (s *InventoryServiceImpl) CheckSaleableStock(ctx context.Context, request *inventorypb.CheckSaleableStockRequest) (resp *inventorypb.CheckSaleableStockResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("inventory-service", "check_saleable_stock", code, time.Since(start))
	}()

	saleable, available, bizErr := s.svc.CheckSaleableStock(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("check saleable stock failed", zap.Int64("sku_id", request.GetSkuId()), zap.Int32("quantity", request.GetQuantity()), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &inventorypb.CheckSaleableStockResponse{
			Saleable:       saleable,
			AvailableStock: available,
			Base:           &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg},
		}, nil
	}

	return &inventorypb.CheckSaleableStockResponse{
		Saleable:       saleable,
		AvailableStock: available,
		Base:           &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
