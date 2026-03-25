package service

import (
	"context"
	"fmt"

	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"
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
	if err := s.cache.DeleteProductLists(ctx); err != nil {
		logx.L(ctx).Warn("invalidate product list cache failed", zap.Error(err), zap.Int64("product_id", productID))
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

func dedupeInt64s(values []int64) []int64 {
	if len(values) <= 1 {
		return values
	}
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func productListCacheKey(req *productpb.ListProductsRequest, page, pageSize int32) string {
	return fmt.Sprintf(
		"page=%d:size=%d:keyword=%s:status_set=%t:status=%d:category_set=%t:category=%d:creator_set=%t:creator=%d",
		page,
		pageSize,
		req.GetKeyword(),
		req.IsSetStatus(),
		req.GetStatus(),
		req.IsSetCategoryId(),
		req.GetCategoryId(),
		req.IsSetCreatorId(),
		req.GetCreatorId(),
	)
}
