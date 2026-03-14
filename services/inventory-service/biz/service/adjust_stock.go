package service

import (
	"context"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
)

func (s *InventoryService) AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*inventorypb.SkuStock, *common.BizError) {
	if req == nil || req.GetSkuId() <= 0 || req.GetTotalStock() < 0 {
		return nil, common.ErrInvalidParam
	}

	stock, err := s.repo.AdjustTotalStock(ctx, req.GetSkuId(), req.GetTotalStock())
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCSkuStock(stock), nil
}
