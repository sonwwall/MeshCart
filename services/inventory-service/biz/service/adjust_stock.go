package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"

	"go.uber.org/zap"
)

func (s *InventoryService) AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*inventorypb.SkuStock, *common.BizError) {
	if req == nil || req.GetSkuId() <= 0 || req.GetTotalStock() < 0 {
		skuID := int64(0)
		totalStock := int64(0)
		if req != nil {
			skuID = req.GetSkuId()
			totalStock = req.GetTotalStock()
		}
		logx.L(ctx).Warn("adjust inventory stock rejected by invalid request",
			zap.Int64("sku_id", skuID),
			zap.Int64("total_stock", totalStock),
		)
		return nil, common.ErrInvalidParam
	}
	logx.L(ctx).Info("adjust inventory stock start",
		zap.Int64("sku_id", req.GetSkuId()),
		zap.Int64("total_stock", req.GetTotalStock()),
	)

	stock, err := s.repo.AdjustTotalStock(ctx, req.GetSkuId(), req.GetTotalStock())
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Warn("adjust inventory stock failed",
			zap.Error(err),
			zap.Int64("sku_id", req.GetSkuId()),
			zap.Int64("total_stock", req.GetTotalStock()),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
	}
	logx.L(ctx).Info("adjust inventory stock completed",
		zap.Int64("sku_id", stock.SKUID),
		zap.Int64("total_stock", stock.TotalStock),
		zap.Int64("available_stock", stock.AvailableStock),
		zap.Int64("reserved_stock", stock.ReservedStock),
	)
	return toRPCSkuStock(stock), nil
}
