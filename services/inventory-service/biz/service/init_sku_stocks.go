package service

import (
	"context"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	dalmodel "meshcart/services/inventory-service/dal/model"
)

func (s *InventoryService) InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	if req == nil || len(req.GetStocks()) == 0 {
		return nil, common.ErrInvalidParam
	}

	models := make([]*dalmodel.InventoryStock, 0, len(req.GetStocks()))
	for _, item := range req.GetStocks() {
		if item == nil || item.GetSkuId() <= 0 || item.GetTotalStock() < 0 {
			return nil, common.ErrInvalidParam
		}
		models = append(models, &dalmodel.InventoryStock{
			ID:             item.GetSkuId(),
			SKUID:          item.GetSkuId(),
			TotalStock:     item.GetTotalStock(),
			ReservedStock:  0,
			AvailableStock: item.GetTotalStock(),
			Version:        1,
		})
	}

	stocks, err := s.repo.CreateBatch(ctx, models)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCSkuStocks(stocks), nil
}
