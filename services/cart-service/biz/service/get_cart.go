package service

import (
	"context"

	"meshcart/app/common"
	cartpb "meshcart/kitex_gen/meshcart/cart"
)

func (s *CartService) GetCart(ctx context.Context, userID int64) ([]*cartpb.CartItem, *common.BizError) {
	if userID <= 0 {
		return nil, common.ErrInvalidParam
	}
	items, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, common.ErrInternalError
	}
	return toRPCCartItems(items), nil
}
