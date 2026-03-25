package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"

	"go.uber.org/zap"
)

func (s *ProductService) BatchGetSKU(ctx context.Context, skuIDs []int64) ([]*productpb.ProductSku, *common.BizError) {
	if len(skuIDs) == 0 {
		return nil, common.ErrInvalidParam
	}

	uniqueIDs := dedupeInt64s(skuIDs)
	cachedSKUs := make(map[int64]*productpb.ProductSku, len(skuIDs))
	missingIDs := uniqueIDs
	if s.cache != nil {
		cached, cacheErr := s.cache.GetSKUs(ctx, uniqueIDs)
		if cacheErr != nil {
			logx.L(ctx).Warn("batch get sku cache read failed", zap.Error(cacheErr), zap.Int64s("sku_ids", uniqueIDs))
		}
		cachedSKUs = cached
		if len(cached) > 0 {
			missingIDs = make([]int64, 0, len(uniqueIDs)-len(cached))
			for _, skuID := range uniqueIDs {
				if _, ok := cached[skuID]; !ok {
					missingIDs = append(missingIDs, skuID)
				}
			}
		}
	}
	if len(missingIDs) == 0 {
		return orderedSKUs(skuIDs, cachedSKUs)
	}

	skus, err := s.repo.GetSKUsByIDsLite(ctx, missingIDs)
	if err != nil {
		return nil, common.ErrInternalError
	}
	if len(skus) != len(missingIDs) {
		return nil, mapRepositoryError(repository.ErrSKUNotFound)
	}

	skuMap := make(map[int64]*productpb.ProductSku, len(skuIDs))
	for skuID, sku := range cachedSKUs {
		skuMap[skuID] = sku
	}
	fetchedSKUs := make([]*productpb.ProductSku, 0, len(skus))
	for _, sku := range skus {
		rpcSKU := toRPCSKU(sku)
		skuMap[sku.ID] = rpcSKU
		fetchedSKUs = append(fetchedSKUs, rpcSKU)
	}
	if s.cache != nil && len(fetchedSKUs) > 0 {
		if cacheErr := s.cache.SetSKUs(ctx, fetchedSKUs); cacheErr != nil {
			logx.L(ctx).Warn("batch get sku cache write failed", zap.Error(cacheErr), zap.Int64s("sku_ids", missingIDs))
		}
	}

	return orderedSKUs(skuIDs, skuMap)
}

func orderedSKUs(skuIDs []int64, skuMap map[int64]*productpb.ProductSku) ([]*productpb.ProductSku, *common.BizError) {
	result := make([]*productpb.ProductSku, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		sku, ok := skuMap[skuID]
		if !ok {
			return nil, mapRepositoryError(repository.ErrSKUNotFound)
		}
		result = append(result, sku)
	}
	return result, nil
}
