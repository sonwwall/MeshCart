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

func (s *InventoryServiceImpl) FreezeSkuStocks(ctx context.Context, request *inventorypb.FreezeSkuStocksRequest) (resp *inventorypb.FreezeSkuStocksResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() { metricsx.ObserveRPC("inventory-service", "freeze_sku_stocks", code, time.Since(start)) }()

	stocks, bizErr := s.svc.FreezeSkuStocks(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("freeze sku stocks failed",
			zap.Int64s("sku_ids", request.GetSkuIds()),
			zap.Int64("operator_id", request.GetOperatorId()),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &inventorypb.FreezeSkuStocksResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &inventorypb.FreezeSkuStocksResponse{
		Stocks: stocks,
		Base:   &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
