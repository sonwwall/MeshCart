package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"

	"go.uber.org/zap"
)

func (s *UserService) GetUser(ctx context.Context, userID int64) (*dalmodel.User, *common.BizError) {
	if userID <= 0 {
		logx.L(ctx).Warn("get user rejected by invalid request", zap.Int64("user_id", userID))
		return nil, common.ErrInvalidParam
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if err == repository.ErrUserNotFound {
			logx.L(ctx).Warn("get user rejected by missing user", zap.Int64("user_id", userID))
			return nil, errno.ErrUserNotFound
		}
		logx.L(ctx).Error("get user by id failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, common.ErrInternalError
	}
	logx.L(ctx).Info("get user completed",
		zap.Int64("user_id", user.ID),
		zap.String("role", user.Role),
	)
	return user, nil
}
