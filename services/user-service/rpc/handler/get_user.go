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
