package service

import (
	"context"

	logx "meshcart/app/log"
	dalmodel "meshcart/services/product-service/dal/model"

	"go.uber.org/zap"
)

func (s *ProductService) invalidateProductCache(ctx context.Context, productID int64, skuIDs []int64) {
	if s.cache == nil {
		return
	}
	if productID > 0 {
		if err := s.cache.DeleteProducts(ctx, []int64{productID}); err != nil {
			logx.L(ctx).Warn("invalidate product cache failed", zap.Error(err), zap.Int64("product_id", productID))
		}
	}
	if len(skuIDs) > 0 {
		if err := s.cache.DeleteSKUs(ctx, skuIDs); err != nil {
			logx.L(ctx).Warn("invalidate sku cache failed", zap.Error(err), zap.Int64s("sku_ids", skuIDs))
		}
	}
}

func skuIDsFromModels(skus []*dalmodel.ProductSKU) []int64 {
	result := make([]int64, 0, len(skus))
	for _, sku := range skus {
		if sku != nil && sku.ID > 0 {
			result = append(result, sku.ID)
		}
	}
	return result
}
