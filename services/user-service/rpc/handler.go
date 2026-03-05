package main

import (
	"context"

	base "meshcart/kitex_gen/meshcart/base"
	user "meshcart/kitex_gen/meshcart/user"
	"meshcart/services/user-service/biz/service"
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
	bizErr := s.svc.Login(ctx, request.Username, request.Password)
	if bizErr != nil {
		return &user.UserLoginResponse{Base: &base.BaseResponse{Code: bizErr.Code, Message: bizErr.Msg}}, nil
	}
	return &user.UserLoginResponse{Base: &base.BaseResponse{Code: 0, Message: "成功"}}, nil
}
