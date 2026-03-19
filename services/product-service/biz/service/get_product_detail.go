package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.uber.org/zap"
)

func (s *ProductService) GetProductDetail(ctx context.Context, productID int64) (*productpb.Product, *common.BizError) {
	if productID <= 0 {
		logx.L(ctx).Warn("get product detail rejected by invalid request", zap.Int64("product_id", productID))
		return nil, common.ErrInvalidParam
	}

	productModel, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Warn("get product detail failed",
			zap.Error(err),
			zap.Int64("product_id", productID),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return nil, mapped
	}
	return toRPCProduct(productModel), nil
}
