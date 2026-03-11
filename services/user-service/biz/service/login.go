package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/services/user-service/biz/dto"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"

	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"
)

func (s *UserService) Login(ctx context.Context, username, password string) (*dto.LoginResult, *common.BizError) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, errno.ErrUserNotFound
		}
		logx.L(ctx).Error("get user by username failed", zap.String("username", username), zap.Error(err))
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
		Role:     user.Role,
	}, nil
}
