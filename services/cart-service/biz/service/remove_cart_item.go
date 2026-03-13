package service

import (
	"context"

	"meshcart/app/common"
)

func (s *CartService) RemoveCartItem(ctx context.Context, userID, itemID int64) *common.BizError {
	if userID <= 0 || itemID <= 0 {
		return common.ErrInvalidParam
	}
	if err := s.repo.DeleteByID(ctx, userID, itemID); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}
