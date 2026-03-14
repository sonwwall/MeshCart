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

func (s *InventoryServiceImpl) InitSkuStocks(ctx context.Context, request *inventorypb.InitSkuStocksRequest) (resp *inventorypb.InitSkuStocksResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("inventory-service", "init_sku_stocks", code, time.Since(start))
	}()

	stocks, bizErr := s.svc.InitSkuStocks(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("init sku stocks failed", zap.Int("sku_count", len(request.GetStocks())), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &inventorypb.InitSkuStocksResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &inventorypb.InitSkuStocksResponse{
		Stocks: stocks,
		Base:   &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
