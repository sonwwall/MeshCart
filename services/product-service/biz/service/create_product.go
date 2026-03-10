package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.uber.org/zap"
)

func (s *ProductService) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (int64, *common.BizError) {
	productModel, skuModels, bizErr := s.buildModelsForWrite(
		0,
		req.Title,
		req.SubTitle,
		req.CategoryId,
		req.Brand,
		req.Description,
		req.Status,
		req.Skus,
		req.CreatorId,
		req.CreatorId,
	)
	if bizErr != nil {
		return 0, bizErr
	}

	if err := s.repo.Create(ctx, productModel, skuModels); err != nil {
		logx.L(ctx).Warn("create product repository failed",
			zap.Error(err),
			zap.Int64("product_id", productModel.ID),
		)
		return 0, mapRepositoryError(err)
	}
	return productModel.ID, nil
}
