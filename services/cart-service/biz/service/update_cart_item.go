package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	cartpb "meshcart/kitex_gen/meshcart/cart"

	"go.uber.org/zap"
)

func (s *CartService) UpdateCartItem(ctx context.Context, req *cartpb.UpdateCartItemRequest) (*cartpb.CartItem, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || req.GetItemId() <= 0 || req.GetQuantity() <= 0 {
		logx.L(ctx).Warn("update cart item rejected by invalid request",
			zap.Int64("user_id", req.GetUserId()),
			zap.Int64("item_id", req.GetItemId()),
			zap.Int32("quantity", req.GetQuantity()),
		)
		return nil, common.ErrInvalidParam
	}

	item, err := s.repo.UpdateByID(ctx, req.GetUserId(), req.GetItemId(), req.GetQuantity(), req.Checked)
	if err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Warn("update cart item failed",
			zap.Error(err),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int64("item_id", req.GetItemId()),
			zap.Int32("quantity", req.GetQuantity()),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return nil, mapped
	}
	logx.L(ctx).Info("update cart item completed",
		zap.Int64("user_id", item.UserID),
		zap.Int64("item_id", item.ID),
		zap.Int32("quantity", item.Quantity),
	)
	return toRPCCartItem(item), nil
}
