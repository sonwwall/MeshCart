package user

import (
	"context"
	"strings"
)

const (
	codeOK              int32 = 0
	codeUserNotFound    int32 = 2010001
	codePasswordInvalid int32 = 2010002
	codeUserLocked      int32 = 2010003
)

type LoginRequest struct {
	Username string
	Password string
}

type LoginResponse struct {
	Code     int32
	Message  string
	UserID   int64
	Token    string
	Username string
}

type Client interface {
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
}

type mockClient struct{}

func NewMockClient() Client {
	return &mockClient{}
}

func (c *mockClient) Login(_ context.Context, req *LoginRequest) (*LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)

	switch {
	case username == "":
		return &LoginResponse{Code: codeUserNotFound, Message: "用户不存在"}, nil
	case username == "locked":
		return &LoginResponse{Code: codeUserLocked, Message: "用户已被锁定"}, nil
	case password != "123456":
		return &LoginResponse{Code: codePasswordInvalid, Message: "用户名或密码错误"}, nil
	default:
		return &LoginResponse{
			Code:     codeOK,
			Message:  "成功",
			UserID:   10001,
			Token:    "mock-token",
			Username: username,
		}, nil
	}
}
