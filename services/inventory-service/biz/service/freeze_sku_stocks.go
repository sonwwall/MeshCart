package service

import (
	"context"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
)

func (s *InventoryService) FreezeSkuStocks(ctx context.Context, req *inventorypb.FreezeSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	if req == nil || len(req.GetSkuIds()) == 0 {
		return nil, common.ErrInvalidParam
	}
	for _, skuID := range req.GetSkuIds() {
		if skuID <= 0 {
			return nil, common.ErrInvalidParam
		}
	}

	stocks, err := s.repo.FreezeBySKUIDs(ctx, req.GetSkuIds())
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCSkuStocks(stocks), nil
}
