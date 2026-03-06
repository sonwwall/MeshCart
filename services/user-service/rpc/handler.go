package main

import (
	"context"
	"time"

	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	tracex "meshcart/app/trace"
	base "meshcart/kitex_gen/meshcart/base"
	user "meshcart/kitex_gen/meshcart/user"
	"meshcart/services/user-service/biz/service"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
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
	// 下游服务入站 server span：与 gateway 的 client span 共同组成跨服务链路。
	start := time.Now()
	code := int32(0)
	defer func() {
		metricsx.ObserveRPC("user-service", "login", code, time.Since(start))
	}()

	ctx, span := tracex.StartSpan(ctx, "meshcart.user-service", "user.rpc.login", oteltrace.WithSpanKind(oteltrace.SpanKindServer))
	defer span.End()
	span.SetAttributes(attribute.String("user.username", request.Username))

	bizErr := s.svc.Login(ctx, request.Username, request.Password)
	if bizErr != nil {
		code = bizErr.Code
		span.SetStatus(codes.Error, bizErr.Msg)
		logx.L(ctx).Warn("user login failed",
			zap.String("username", request.Username),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return &user.UserLoginResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	span.SetStatus(codes.Ok, "ok")
	logx.L(ctx).Info("user login success", zap.String("username", request.Username))
	return &user.UserLoginResponse{Base: &base.BaseResponse{Code: 0, Message: "成功"}}, nil
}
