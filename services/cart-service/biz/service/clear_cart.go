package service

import (
	"context"

	"meshcart/app/common"
)

func (s *CartService) ClearCart(ctx context.Context, userID int64) *common.BizError {
	if userID <= 0 {
		return common.ErrInvalidParam
	}
	if err := s.repo.ClearByUserID(ctx, userID); err != nil {
		return common.ErrInternalError
	}
	return nil
}
