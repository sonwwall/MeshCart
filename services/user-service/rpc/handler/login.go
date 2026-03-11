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
