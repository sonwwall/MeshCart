package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/services/user-service/biz/dto"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func (s *UserService) Login(ctx context.Context, username, password string) (*dto.LoginResult, *common.BizError) {
	logx.L(ctx).Info("user login start", zap.String("username", username))
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		if err == repository.ErrUserNotFound {
			logx.L(ctx).Warn("user login rejected by missing user", zap.String("username", username))
			return nil, errno.ErrUserNotFound
		}
		logx.L(ctx).Error("get user by username failed", zap.String("username", username), zap.Error(err))
		return nil, common.ErrInternalError
	}

	if user.IsLocked {
		logx.L(ctx).Warn("user login rejected by locked user",
			zap.String("username", username),
			zap.Int64("user_id", user.ID),
		)
		return nil, errno.ErrUserLocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		logx.L(ctx).Warn("user login rejected by invalid password",
			zap.String("username", username),
			zap.Int64("user_id", user.ID),
		)
		return nil, errno.ErrPasswordInvalid
	}

	logx.L(ctx).Info("user login completed",
		zap.String("username", username),
		zap.Int64("user_id", user.ID),
		zap.String("role", user.Role),
	)
	return &dto.LoginResult{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}, nil
}
