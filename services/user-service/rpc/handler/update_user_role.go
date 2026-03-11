package handler

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	user "meshcart/kitex_gen/meshcart/user"

	"go.uber.org/zap"
)

func (s *UserServiceImpl) UpdateUserRole(ctx context.Context, request *user.UserUpdateRoleRequest) (resp *user.UserUpdateRoleResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("user-service", "update_user_role", code, time.Since(start))
	}()

	bizErr := s.svc.UpdateUserRole(ctx, request.UserId, request.Role)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("update user role failed",
			zap.Int64("user_id", request.UserId),
			zap.String("role", request.Role),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &user.UserUpdateRoleResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}

	logx.L(ctx).Info("update user role success", zap.Int64("user_id", request.UserId), zap.String("role", request.Role))
	return &user.UserUpdateRoleResponse{Base: &base.BaseResponse{Code: 0, Message: "成功"}}, nil
}
