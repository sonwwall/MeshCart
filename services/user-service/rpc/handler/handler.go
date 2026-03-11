package handler

import "meshcart/services/user-service/biz/service"

type UserServiceImpl struct {
	svc *service.UserService
}

func NewUserServiceImpl(svc *service.UserService) *UserServiceImpl {
	return &UserServiceImpl{svc: svc}
}
