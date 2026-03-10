package service

import (
	"context"

	"meshcart/app/common"
	productpb "meshcart/kitex_gen/meshcart/product"
)

func (s *ProductService) GetProductDetail(ctx context.Context, productID int64) (*productpb.Product, *common.BizError) {
	if productID <= 0 {
		return nil, common.ErrInvalidParam
	}

	productModel, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCProduct(productModel), nil
}
