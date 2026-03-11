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
