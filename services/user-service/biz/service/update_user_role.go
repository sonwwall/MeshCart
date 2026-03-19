package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/services/user-service/biz/errno"
	bizmodel "meshcart/services/user-service/biz/model"
	"meshcart/services/user-service/biz/repository"

	"go.uber.org/zap"
)

func (s *UserService) UpdateUserRole(ctx context.Context, userID int64, role string) *common.BizError {
	if userID <= 0 {
		logx.L(ctx).Warn("update user role rejected by invalid user_id", zap.Int64("user_id", userID))
		return common.ErrInvalidParam
	}

	role = strings.TrimSpace(role)
	if !bizmodel.IsValidRole(role) {
		logx.L(ctx).Warn("update user role rejected by invalid role",
			zap.Int64("user_id", userID),
			zap.String("role", role),
		)
		return errno.ErrRoleInvalid
	}
	logx.L(ctx).Info("update user role start",
		zap.Int64("user_id", userID),
		zap.String("role", role),
	)

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return errno.ErrUserNotFound
		}
		logx.L(ctx).Error("get target user failed before update role", zap.Int64("user_id", userID), zap.Error(err))
		return common.ErrInternalError
	}

	if user.Role == bizmodel.RoleSuperAdmin && role != bizmodel.RoleSuperAdmin {
		total, err := s.repo.CountByRole(ctx, bizmodel.RoleSuperAdmin)
		if err != nil {
			logx.L(ctx).Error("count superadmin users failed", zap.Error(err))
			return common.ErrInternalError
		}
		if total <= 1 {
			logx.L(ctx).Warn("update user role rejected by last superadmin protection",
				zap.Int64("user_id", userID),
				zap.String("role", role),
			)
			return errno.ErrLastSuperAdmin
		}
	}

	if err := s.repo.UpdateRole(ctx, userID, role); err != nil {
		if err == repository.ErrUserNotFound {
			return errno.ErrUserNotFound
		}
		logx.L(ctx).Error("update user role failed",
			zap.Int64("user_id", userID),
			zap.String("role", role),
			zap.Error(err),
		)
		return common.ErrInternalError
	}
	logx.L(ctx).Info("update user role completed",
		zap.Int64("user_id", userID),
		zap.String("role", role),
	)
	return nil
}
