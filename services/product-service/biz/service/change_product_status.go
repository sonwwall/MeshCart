package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"

	"go.uber.org/zap"
)

func (s *ProductService) ChangeProductStatus(ctx context.Context, productID int64, status int32, operatorID int64) *common.BizError {
	if productID <= 0 || !isValidProductStatus(status) {
		logx.L(ctx).Warn("change product status rejected by invalid request",
			zap.Int64("product_id", productID),
			zap.Int32("status", status),
			zap.Int64("operator_id", operatorID),
		)
		return common.ErrInvalidParam
	}
	if err := s.repo.ChangeStatus(ctx, productID, status, operatorID); err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Warn("change product status failed",
			zap.Error(err),
			zap.Int64("product_id", productID),
			zap.Int32("status", status),
			zap.Int64("operator_id", operatorID),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return mapped
	}
	s.invalidateProductCache(ctx, productID, nil)
	logx.L(ctx).Info("change product status completed",
		zap.Int64("product_id", productID),
		zap.Int32("status", status),
		zap.Int64("operator_id", operatorID),
	)
	return nil
}
