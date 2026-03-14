package service

import (
	"context"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	"meshcart/services/inventory-service/biz/errno"
	"meshcart/services/inventory-service/biz/repository"
)

func (s *InventoryService) CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (bool, int64, *common.BizError) {
	if req == nil || req.GetSkuId() <= 0 || req.GetQuantity() <= 0 {
		return false, 0, common.ErrInvalidParam
	}

	stock, err := s.repo.GetBySKUID(ctx, req.GetSkuId())
	if err != nil {
		if err == repository.ErrStockNotFound {
			return false, 0, errno.ErrInsufficientStock
		}
		return false, 0, common.ErrInternalError
	}
	if stock.AvailableStock < int64(req.GetQuantity()) {
		return false, stock.AvailableStock, errno.ErrInsufficientStock
	}
	return true, stock.AvailableStock, nil
}
