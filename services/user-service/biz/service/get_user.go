package service

import (
	"context"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"
)

func (s *UserService) GetUser(ctx context.Context, userID int64) (*dalmodel.User, *common.BizError) {
	if userID <= 0 {
		return nil, common.ErrInvalidParam
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, errno.ErrUserNotFound
		}
		return nil, common.ErrInternalError
	}
	return user, nil
}
