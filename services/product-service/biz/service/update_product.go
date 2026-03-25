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
		logx.L(ctx).Warn("update product rejected by invalid request", zap.Int64("product_id", req.GetProductId()))
		return nil, common.ErrInvalidParam
	}
	logx.L(ctx).Info("update product start",
		zap.Int64("product_id", req.GetProductId()),
		zap.Int("sku_count", len(req.GetSkus())),
		zap.Int64("operator_id", req.GetOperatorId()),
		zap.Int32("status", req.GetStatus()),
	)

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
		logx.L(ctx).Warn("update product rejected by invalid payload",
			zap.Int64("product_id", req.GetProductId()),
			zap.Int("sku_count", len(req.GetSkus())),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return nil, bizErr
	}

	if err := s.repo.Update(ctx, productModel, skuModels); err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Warn("update product repository failed",
			zap.Error(err),
			zap.Int64("product_id", productModel.ID),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return nil, mapped
	}
	s.invalidateProductCache(ctx, productModel.ID, skuIDsFromModels(skuModels))
	logx.L(ctx).Info("update product completed",
		zap.Int64("product_id", productModel.ID),
		zap.Int("sku_count", len(skuModels)),
	)
	return toProductSkusForCreate(skuModels), nil
}
