package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"

	"go.uber.org/zap"
)

func (s *CartService) RemoveCartItem(ctx context.Context, userID, itemID int64) *common.BizError {
	if userID <= 0 || itemID <= 0 {
		logx.L(ctx).Warn("remove cart item rejected by invalid request",
			zap.Int64("user_id", userID),
			zap.Int64("item_id", itemID),
		)
		return common.ErrInvalidParam
	}
	if err := s.repo.DeleteByID(ctx, userID, itemID); err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Warn("remove cart item failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
			zap.Int64("item_id", itemID),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return mapped
	}
	logx.L(ctx).Info("remove cart item completed",
		zap.Int64("user_id", userID),
		zap.Int64("item_id", itemID),
	)
	return nil
}
