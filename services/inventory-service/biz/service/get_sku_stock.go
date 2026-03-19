package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"

	"go.uber.org/zap"
)

func (s *InventoryService) GetSkuStock(ctx context.Context, skuID int64) (*inventorypb.SkuStock, *common.BizError) {
	if skuID <= 0 {
		logx.L(ctx).Warn("get sku stock rejected by invalid request", zap.Int64("sku_id", skuID))
		return nil, common.ErrInvalidParam
	}

	stock, err := s.repo.GetBySKUID(ctx, skuID)
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Warn("get sku stock failed",
			zap.Error(err),
			zap.Int64("sku_id", skuID),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
	}
	logx.L(ctx).Info("get sku stock succeeded",
		zap.Int64("sku_id", stock.SKUID),
		zap.Int64("total_stock", stock.TotalStock),
		zap.Int64("available_stock", stock.AvailableStock),
		zap.Int64("reserved_stock", stock.ReservedStock),
	)
	return toRPCSkuStock(stock), nil
}
