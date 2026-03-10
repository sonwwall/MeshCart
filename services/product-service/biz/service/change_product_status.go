package service

import (
	"context"

	"meshcart/app/common"
)

func (s *ProductService) ChangeProductStatus(ctx context.Context, productID int64, status int32, operatorID int64) *common.BizError {
	if productID <= 0 || !isValidProductStatus(status) {
		return common.ErrInvalidParam
	}
	if err := s.repo.ChangeStatus(ctx, productID, status, operatorID); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}
