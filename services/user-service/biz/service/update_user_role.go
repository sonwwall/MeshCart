package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/errno"
	bizmodel "meshcart/services/user-service/biz/model"
	"meshcart/services/user-service/biz/repository"
)

func (s *UserService) UpdateUserRole(ctx context.Context, userID int64, role string) *common.BizError {
	if userID <= 0 {
		return common.ErrInvalidParam
	}

	role = strings.TrimSpace(role)
	if !bizmodel.IsValidRole(role) {
		return errno.ErrRoleInvalid
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return errno.ErrUserNotFound
		}
		return common.ErrInternalError
	}

	if user.Role == bizmodel.RoleSuperAdmin && role != bizmodel.RoleSuperAdmin {
		total, err := s.repo.CountByRole(ctx, bizmodel.RoleSuperAdmin)
		if err != nil {
			return common.ErrInternalError
		}
		if total <= 1 {
			return errno.ErrLastSuperAdmin
		}
	}

	if err := s.repo.UpdateRole(ctx, userID, role); err != nil {
		if err == repository.ErrUserNotFound {
			return errno.ErrUserNotFound
		}
		return common.ErrInternalError
	}
	return nil
}
