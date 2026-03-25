package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"

	"go.uber.org/zap"
)

func (s *ProductService) BatchGetProducts(ctx context.Context, productIDs []int64) ([]*productpb.Product, *common.BizError) {
	if len(productIDs) == 0 {
		return nil, common.ErrInvalidParam
	}

	seen := make(map[int64]struct{}, len(productIDs))
	uniqueIDs := make([]int64, 0, len(productIDs))
	for _, productID := range productIDs {
		if productID <= 0 {
			logx.L(ctx).Warn("batch get products rejected by invalid product_id", zap.Int64("product_id", productID))
			return nil, common.ErrInvalidParam
		}
		if _, ok := seen[productID]; ok {
			continue
		}
		seen[productID] = struct{}{}
		uniqueIDs = append(uniqueIDs, productID)
	}

	cachedProducts := make(map[int64]*productpb.Product, len(uniqueIDs))
	missingIDs := uniqueIDs
	if s.cache != nil {
		cached, cacheErr := s.cache.GetProducts(ctx, uniqueIDs)
		if cacheErr != nil {
			logx.L(ctx).Warn("batch get products cache read failed", zap.Error(cacheErr), zap.Int64s("product_ids", uniqueIDs))
		} else {
			cachedProducts = cached
			if len(cached) > 0 {
				missingIDs = make([]int64, 0, len(uniqueIDs)-len(cached))
				for _, productID := range uniqueIDs {
					if _, ok := cached[productID]; !ok {
						missingIDs = append(missingIDs, productID)
					}
				}
			}
		}
	}

	resultMap := make(map[int64]*productpb.Product, len(uniqueIDs))
	for productID, product := range cachedProducts {
		resultMap[productID] = product
	}
	if len(missingIDs) == 0 {
		return orderedProducts(uniqueIDs, resultMap)
	}

	products, err := s.repo.GetByIDs(ctx, missingIDs)
	if err != nil {
		logx.L(ctx).Error("batch get products repository failed", zap.Error(err), zap.Int64s("product_ids", missingIDs))
		return nil, common.ErrInternalError
	}
	if len(products) != len(missingIDs) {
		return nil, mapRepositoryError(repository.ErrProductNotFound)
	}

	fetchedProducts := make([]*productpb.Product, 0, len(products))
	for _, productModel := range products {
		product := toRPCProduct(productModel)
		resultMap[productModel.ID] = product
		fetchedProducts = append(fetchedProducts, product)
	}
	if s.cache != nil && len(fetchedProducts) > 0 {
		if cacheErr := s.cache.SetProducts(ctx, fetchedProducts); cacheErr != nil {
			logx.L(ctx).Warn("batch get products cache write failed", zap.Error(cacheErr), zap.Int64s("product_ids", missingIDs))
		}
	}

	return orderedProducts(uniqueIDs, resultMap)
}

func orderedProducts(productIDs []int64, productMap map[int64]*productpb.Product) ([]*productpb.Product, *common.BizError) {
	result := make([]*productpb.Product, 0, len(productIDs))
	for _, productID := range productIDs {
		product, ok := productMap[productID]
		if !ok {
			return nil, mapRepositoryError(repository.ErrProductNotFound)
		}
		result = append(result, product)
	}
	return result, nil
}
