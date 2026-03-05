package service

import (
	"context"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
)

type UserService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Login(ctx context.Context, username, password string) *common.BizError {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return errno.ErrUserNotFound
		}
		return common.ErrInternalError
	}

	if user.IsLocked {
		return errno.ErrUserLocked
	}
	if user.Password != password {
		return errno.ErrPasswordInvalid
	}

	return nil
}
