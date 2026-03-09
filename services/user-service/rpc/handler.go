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
