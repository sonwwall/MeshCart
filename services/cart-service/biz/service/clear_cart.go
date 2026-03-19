package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"

	"go.uber.org/zap"
)

func (s *CartService) ClearCart(ctx context.Context, userID int64) *common.BizError {
	if userID <= 0 {
		logx.L(ctx).Warn("clear cart rejected by invalid request", zap.Int64("user_id", userID))
		return common.ErrInvalidParam
	}
	if err := s.repo.ClearByUserID(ctx, userID); err != nil {
		logx.L(ctx).Error("clear cart repository failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
		)
		return common.ErrInternalError
	}
	logx.L(ctx).Info("clear cart completed", zap.Int64("user_id", userID))
	return nil
}
