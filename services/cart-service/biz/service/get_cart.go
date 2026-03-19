package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	cartpb "meshcart/kitex_gen/meshcart/cart"

	"go.uber.org/zap"
)

func (s *CartService) GetCart(ctx context.Context, userID int64) ([]*cartpb.CartItem, *common.BizError) {
	if userID <= 0 {
		logx.L(ctx).Warn("get cart rejected by invalid request", zap.Int64("user_id", userID))
		return nil, common.ErrInvalidParam
	}
	items, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		logx.L(ctx).Error("get cart repository failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
		)
		return nil, common.ErrInternalError
	}
	logx.L(ctx).Info("get cart completed",
		zap.Int64("user_id", userID),
		zap.Int("item_count", len(items)),
	)
	return toRPCCartItems(items), nil
}
