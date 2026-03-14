package service

import (
	"context"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
)

func (s *InventoryService) BatchGetSkuStock(ctx context.Context, skuIDs []int64) ([]*inventorypb.SkuStock, *common.BizError) {
	if len(skuIDs) == 0 {
		return []*inventorypb.SkuStock{}, nil
	}
	for _, skuID := range skuIDs {
		if skuID <= 0 {
			return nil, common.ErrInvalidParam
		}
	}

	stocks, err := s.repo.ListBySKUIDs(ctx, skuIDs)
	if err != nil {
		return nil, common.ErrInternalError
	}
	return toRPCSkuStocks(stocks), nil
}
