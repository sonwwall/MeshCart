package service

import (
	"context"

	"meshcart/app/common"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"
	dalmodel "meshcart/services/product-service/dal/model"
)

func (s *ProductService) BatchGetSKU(ctx context.Context, skuIDs []int64) ([]*productpb.ProductSku, *common.BizError) {
	if len(skuIDs) == 0 {
		return nil, common.ErrInvalidParam
	}

	skus, err := s.repo.GetSKUsByIDs(ctx, skuIDs)
	if err != nil {
		return nil, common.ErrInternalError
	}
	if len(skus) != len(skuIDs) {
		return nil, mapRepositoryError(repository.ErrSKUNotFound)
	}

	skuMap := make(map[int64]*dalmodel.ProductSKU, len(skus))
	for _, sku := range skus {
		skuMap[sku.ID] = sku
	}

	result := make([]*productpb.ProductSku, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		skuModel, ok := skuMap[skuID]
		if !ok {
			return nil, mapRepositoryError(repository.ErrSKUNotFound)
		}
		result = append(result, toRPCSKU(skuModel))
	}
	return result, nil
}
