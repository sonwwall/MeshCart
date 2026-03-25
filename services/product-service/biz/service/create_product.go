package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.uber.org/zap"
)

func (s *ProductService) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (int64, []*productpb.ProductSku, *common.BizError) {
	logx.L(ctx).Info("create product start",
		zap.String("title", req.GetTitle()),
		zap.Int("sku_count", len(req.GetSkus())),
		zap.Int64("creator_id", req.GetCreatorId()),
		zap.Int32("status", req.GetStatus()),
	)
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
		logx.L(ctx).Warn("create product rejected by invalid request",
			zap.String("title", req.GetTitle()),
			zap.Int("sku_count", len(req.GetSkus())),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return 0, nil, bizErr
	}

	if err := s.repo.Create(ctx, productModel, skuModels); err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Warn("create product repository failed",
			zap.Error(err),
			zap.Int64("product_id", productModel.ID),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return 0, nil, mapped
	}
	s.invalidateProductCache(ctx, productModel.ID, skuIDsFromModels(skuModels))
	logx.L(ctx).Info("create product completed",
		zap.Int64("product_id", productModel.ID),
		zap.Int("sku_count", len(skuModels)),
	)
	return productModel.ID, toProductSkusForCreate(skuModels), nil
}
