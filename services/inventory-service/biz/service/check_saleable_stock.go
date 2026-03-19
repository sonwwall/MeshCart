package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	"meshcart/services/inventory-service/biz/errno"
	"meshcart/services/inventory-service/biz/repository"

	"go.uber.org/zap"
)

func (s *InventoryService) CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (bool, int64, *common.BizError) {
	if req == nil || req.GetSkuId() <= 0 || req.GetQuantity() <= 0 {
		skuID := int64(0)
		quantity := int32(0)
		if req != nil {
			skuID = req.GetSkuId()
			quantity = req.GetQuantity()
		}
		logx.L(ctx).Warn("check saleable stock rejected by invalid request",
			zap.Int64("sku_id", skuID),
			zap.Int32("quantity", quantity),
		)
		return false, 0, common.ErrInvalidParam
	}

	stock, err := s.repo.GetBySKUID(ctx, req.GetSkuId())
	if err != nil {
		if err == repository.ErrStockNotFound {
			logx.L(ctx).Warn("check saleable stock treated as insufficient because stock not found",
				zap.Int64("sku_id", req.GetSkuId()),
				zap.Int32("quantity", req.GetQuantity()),
			)
			return false, 0, errno.ErrInsufficientStock
		}
		logx.L(ctx).Error("check saleable stock repository failed",
			zap.Error(err),
			zap.Int64("sku_id", req.GetSkuId()),
			zap.Int32("quantity", req.GetQuantity()),
		)
		return false, 0, common.ErrInternalError
	}
	if stock.Status != StockStatusActive {
		logx.L(ctx).Warn("check saleable stock rejected by frozen stock",
			zap.Int64("sku_id", stock.SKUID),
			zap.Int32("quantity", req.GetQuantity()),
			zap.Int64("available_stock", stock.AvailableStock),
			zap.Int32("status", stock.Status),
		)
		return false, stock.AvailableStock, errno.ErrStockFrozen
	}
	if stock.AvailableStock < int64(req.GetQuantity()) {
		logx.L(ctx).Warn("check saleable stock rejected by insufficient stock",
			zap.Int64("sku_id", stock.SKUID),
			zap.Int32("quantity", req.GetQuantity()),
			zap.Int64("available_stock", stock.AvailableStock),
		)
		return false, stock.AvailableStock, errno.ErrInsufficientStock
	}
	logx.L(ctx).Info("check saleable stock succeeded",
		zap.Int64("sku_id", stock.SKUID),
		zap.Int32("quantity", req.GetQuantity()),
		zap.Int64("available_stock", stock.AvailableStock),
	)
	return true, stock.AvailableStock, nil
}
