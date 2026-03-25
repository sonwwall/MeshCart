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

	products, err := s.repo.GetByIDs(ctx, uniqueIDs)
	if err != nil {
		logx.L(ctx).Error("batch get products repository failed", zap.Error(err), zap.Int64s("product_ids", uniqueIDs))
		return nil, common.ErrInternalError
	}
	if len(products) != len(uniqueIDs) {
		return nil, mapRepositoryError(repository.ErrProductNotFound)
	}

	productMap := make(map[int64]*productpb.Product, len(products))
	for _, productModel := range products {
		productMap[productModel.ID] = toRPCProduct(productModel)
	}

	result := make([]*productpb.Product, 0, len(uniqueIDs))
	for _, productID := range uniqueIDs {
		product, ok := productMap[productID]
		if !ok {
			return nil, mapRepositoryError(repository.ErrProductNotFound)
		}
		result = append(result, product)
	}
	return result, nil
}
