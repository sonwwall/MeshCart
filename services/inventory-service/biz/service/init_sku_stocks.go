package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	dalmodel "meshcart/services/inventory-service/dal/model"

	"go.uber.org/zap"
)

func (s *InventoryService) InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	if req == nil || len(req.GetStocks()) == 0 {
		count := 0
		if req != nil {
			count = len(req.GetStocks())
		}
		logx.L(ctx).Warn("init sku stocks rejected by invalid request", zap.Int("stock_count", count))
		return nil, common.ErrInvalidParam
	}
	logx.L(ctx).Info("init sku stocks start", zap.Int("stock_count", len(req.GetStocks())))

	models := make([]*dalmodel.InventoryStock, 0, len(req.GetStocks()))
	for _, item := range req.GetStocks() {
		if item == nil || item.GetSkuId() <= 0 || item.GetTotalStock() < 0 {
			logx.L(ctx).Warn("init sku stocks rejected by invalid stock item",
				zap.Int64("sku_id", item.GetSkuId()),
				zap.Int64("total_stock", item.GetTotalStock()),
			)
			return nil, common.ErrInvalidParam
		}
		models = append(models, &dalmodel.InventoryStock{
			ID:             item.GetSkuId(),
			SKUID:          item.GetSkuId(),
			TotalStock:     item.GetTotalStock(),
			ReservedStock:  0,
			AvailableStock: item.GetTotalStock(),
			Status:         StockStatusActive,
			Version:        1,
		})
	}

	stocks, err := s.repo.CreateBatch(ctx, models)
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Warn("init sku stocks failed",
			zap.Error(err),
			zap.Int("stock_count", len(models)),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
	}
	skuIDs := make([]int64, 0, len(stocks))
	for _, stock := range stocks {
		if stock != nil {
			skuIDs = append(skuIDs, stock.SKUID)
		}
	}
	logx.L(ctx).Info("init sku stocks completed",
		zap.Int("stock_count", len(stocks)),
		zap.Int64s("sku_ids", skuIDs),
	)
	return toRPCSkuStocks(stocks), nil
}
