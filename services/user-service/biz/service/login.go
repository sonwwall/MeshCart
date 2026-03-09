package service

import (
	"context"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/dto"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"

	"golang.org/x/crypto/bcrypt"
)

func (s *UserService) Login(ctx context.Context, username, password string) (*dto.LoginResult, *common.BizError) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, errno.ErrUserNotFound
		}
		return nil, common.ErrInternalError
	}

	if user.IsLocked {
		return nil, errno.ErrUserLocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errno.ErrPasswordInvalid
	}

	return &dto.LoginResult{
		UserID:   user.ID,
		Username: user.Username,
	}, nil
}
