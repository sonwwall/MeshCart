package service

import (
	"context"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
)

func (s *InventoryService) GetSkuStock(ctx context.Context, skuID int64) (*inventorypb.SkuStock, *common.BizError) {
	if skuID <= 0 {
		return nil, common.ErrInvalidParam
	}

	stock, err := s.repo.GetBySKUID(ctx, skuID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCSkuStock(stock), nil
}
