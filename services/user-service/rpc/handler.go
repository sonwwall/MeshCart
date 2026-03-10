package main

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	base "meshcart/kitex_gen/meshcart/base"
	user "meshcart/kitex_gen/meshcart/user"
	"meshcart/services/user-service/biz/service"

	"go.uber.org/zap"
)

// UserServiceImpl implements the last service interface defined in the IDL.
type UserServiceImpl struct {
	svc *service.UserService
}

func NewUserServiceImpl(svc *service.UserService) *UserServiceImpl {
	return &UserServiceImpl{svc: svc}
}

// Login implements the UserServiceImpl interface.
func (s *UserServiceImpl) Login(ctx context.Context, request *user.UserLoginRequest) (resp *user.UserLoginResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("user-service", "login", code, time.Since(start))
	}()

	loginResult, bizErr := s.svc.Login(ctx, request.Username, request.Password)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("user login failed",
			zap.String("username", request.Username),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &user.UserLoginResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	logx.L(ctx).Info("user login success", zap.String("username", request.Username))
	return &user.UserLoginResponse{
		UserId:   loginResult.UserID,
		Username: loginResult.Username,
		Role:     loginResult.Role,
		Base:     &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}

func (s *UserServiceImpl) Register(ctx context.Context, request *user.UserRegisterRequest) (resp *user.UserRegisterResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("user-service", "register", code, time.Since(start))
	}()

	bizErr := s.svc.Register(ctx, request.Username, request.Password)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("user register failed",
			zap.String("username", request.Username),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &user.UserRegisterResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	logx.L(ctx).Info("user register success", zap.String("username", request.Username))
	return &user.UserRegisterResponse{Base: &base.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *UserServiceImpl) GetUser(ctx context.Context, request *user.UserGetRequest) (resp *user.UserGetResponse, err error) {
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("user-service", "get_user", code, time.Since(start))
	}()

	userInfo, bizErr := s.svc.GetUser(ctx, request.UserId)
	if bizErr != nil {
		code = bizErr.Code
		logx.L(ctx).Warn("get user failed",
			zap.Int64("user_id", request.UserId),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &user.UserGetResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}

	return &user.UserGetResponse{
		UserId:   userInfo.ID,
		Username: userInfo.Username,
		Role:     userInfo.Role,
		IsLocked: userInfo.IsLocked,
		Base:     &base.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}

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
