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

func (s *InventoryServiceImpl) ReleaseReservedSkuStocks(ctx context.Context, request *inventorypb.ReleaseReservedSkuStocksRequest) (resp *inventorypb.ReleaseReservedSkuStocksResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("inventory-service", "release_reserved_sku_stocks", code, time.Since(start))
	}()

	stocks, bizErr := s.svc.ReleaseReservedSkuStocks(ctx, request)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("release reserved sku stocks failed", zap.String("biz_type", request.GetBizType()), zap.String("biz_id", request.GetBizId()), zap.Int("item_count", len(request.GetItems())), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
		return &inventorypb.ReleaseReservedSkuStocksResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &inventorypb.ReleaseReservedSkuStocksResponse{
		Stocks: stocks,
		Base:   &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}
