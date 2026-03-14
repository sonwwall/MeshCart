package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.uber.org/zap"
)

func (s *ProductService) UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) ([]*productpb.ProductSku, *common.BizError) {
	if req.ProductId <= 0 {
		return nil, common.ErrInvalidParam
	}

	productModel, skuModels, bizErr := s.buildModelsForWrite(
		req.ProductId,
		req.Title,
		req.SubTitle,
		req.CategoryId,
		req.Brand,
		req.Description,
		req.Status,
		req.Skus,
		0,
		req.OperatorId,
	)
	if bizErr != nil {
		return nil, bizErr
	}

	if err := s.repo.Update(ctx, productModel, skuModels); err != nil {
		logx.L(ctx).Warn("update product repository failed",
			zap.Error(err),
			zap.Int64("product_id", productModel.ID),
		)
		return nil, mapRepositoryError(err)
	}
	return toProductSkusForCreate(skuModels), nil
}
