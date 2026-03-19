package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"

	"go.uber.org/zap"
)

func (s *InventoryService) FreezeSkuStocks(ctx context.Context, req *inventorypb.FreezeSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	if req == nil || len(req.GetSkuIds()) == 0 {
		count := 0
		if req != nil {
			count = len(req.GetSkuIds())
		}
		logx.L(ctx).Warn("freeze sku stocks rejected by invalid request", zap.Int("sku_count", count))
		return nil, common.ErrInvalidParam
	}
	for _, skuID := range req.GetSkuIds() {
		if skuID <= 0 {
			logx.L(ctx).Warn("freeze sku stocks rejected by invalid sku id", zap.Int64("sku_id", skuID))
			return nil, common.ErrInvalidParam
		}
	}
	logx.L(ctx).Info("freeze sku stocks start",
		zap.Int("sku_count", len(req.GetSkuIds())),
		zap.Int64s("sku_ids", req.GetSkuIds()),
	)

	stocks, err := s.repo.FreezeBySKUIDs(ctx, req.GetSkuIds())
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Warn("freeze sku stocks failed",
			zap.Error(err),
			zap.Int64s("sku_ids", req.GetSkuIds()),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
	}
	logx.L(ctx).Info("freeze sku stocks completed",
		zap.Int("stock_count", len(stocks)),
		zap.Int64s("sku_ids", req.GetSkuIds()),
	)
	return toRPCSkuStocks(stocks), nil
}
