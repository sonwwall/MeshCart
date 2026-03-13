package service

import (
	"context"

	"meshcart/app/common"
	cartpb "meshcart/kitex_gen/meshcart/cart"
)

func (s *CartService) UpdateCartItem(ctx context.Context, req *cartpb.UpdateCartItemRequest) (*cartpb.CartItem, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || req.GetItemId() <= 0 || req.GetQuantity() <= 0 {
		return nil, common.ErrInvalidParam
	}

	item, err := s.repo.UpdateByID(ctx, req.GetUserId(), req.GetItemId(), req.GetQuantity(), req.Checked)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCCartItem(item), nil
}
