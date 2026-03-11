package user

import (
	"context"
	"errors"
	"strings"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"
)

type stubUserClient struct {
	loginFn      func(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error)
	registerFn   func(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error)
	getUserFn    func(ctx context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error)
	updateRoleFn func(ctx context.Context, req *userrpc.UpdateUserRoleRequest) (*userrpc.UpdateUserRoleResponse, error)
}

func (s *stubUserClient) Login(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
	return s.loginFn(ctx, req)
}

func (s *stubUserClient) Register(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error) {
	return s.registerFn(ctx, req)
}

func (s *stubUserClient) GetUser(ctx context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error) {
	return s.getUserFn(ctx, req)
}

func (s *stubUserClient) UpdateUserRole(ctx context.Context, req *userrpc.UpdateUserRoleRequest) (*userrpc.UpdateUserRoleResponse, error) {
	return s.updateRoleFn(ctx, req)
}

func newTestServiceContext(t *testing.T, client userrpc.Client) *svc.ServiceContext {
	t.Helper()
	jwtMiddleware, err := middleware.NewJWT(config.JWTConfig{
		Secret:            "test-secret",
		Issuer:            "test-issuer",
		TimeoutMinutes:    30,
		MaxRefreshMinutes: 60,
	})
	if err != nil {
		t.Fatalf("init jwt middleware: %v", err)
	}
	return &svc.ServiceContext{
		UserClient: client,
		JWT:        jwtMiddleware,
	}
}

func TestLogin_InvalidParam(t *testing.T) {
	logic := NewLoginLogic(context.Background(), newTestServiceContext(t, &stubUserClient{}))

	data, bizErr := logic.Login(&types.UserLoginRequest{Username: " ", Password: ""})
	if data != nil {
		t.Fatalf("expected nil data, got %+v", data)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestLogin_RPCError(t *testing.T) {
	logic := NewLoginLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		loginFn: func(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return nil, errors.New("rpc failed")
		},
	}))

	data, bizErr := logic.Login(&types.UserLoginRequest{Username: "tester", Password: "123456"})
	if data != nil {
		t.Fatalf("expected nil data, got %+v", data)
	}
	if bizErr != common.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %+v", bizErr)
	}
}

func TestLogin_RPCTimeoutError(t *testing.T) {
	logic := NewLoginLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		loginFn: func(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return nil, context.DeadlineExceeded
		},
	}))

	data, bizErr := logic.Login(&types.UserLoginRequest{Username: "tester", Password: "123456"})
	if data != nil {
		t.Fatalf("expected nil data, got %+v", data)
	}
	if bizErr != common.ErrServiceBusy {
		t.Fatalf("expected ErrServiceBusy, got %+v", bizErr)
	}
}

func TestLogin_RPCServiceUnavailable(t *testing.T) {
	logic := NewLoginLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		loginFn: func(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return nil, errors.New("dial tcp 127.0.0.1:8888: connect: connection refused")
		},
	}))

	data, bizErr := logic.Login(&types.UserLoginRequest{Username: "tester", Password: "123456"})
	if data != nil {
		t.Fatalf("expected nil data, got %+v", data)
	}
	if bizErr != common.ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable, got %+v", bizErr)
	}
}

func TestLogin_BusinessError(t *testing.T) {
	logic := NewLoginLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		loginFn: func(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return &userrpc.LoginResponse{Code: 2010002, Message: "用户名或密码错误"}, nil
		},
	}))

	data, bizErr := logic.Login(&types.UserLoginRequest{Username: "tester", Password: "badpass"})
	if data != nil {
		t.Fatalf("expected nil data, got %+v", data)
	}
	if bizErr == nil || bizErr.Code != 2010002 {
		t.Fatalf("expected business error 2010002, got %+v", bizErr)
	}
}

func TestLogin_Success(t *testing.T) {
	logic := NewLoginLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		loginFn: func(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return &userrpc.LoginResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   12345,
				Username: "tester",
				Role:     "superadmin",
			}, nil
		},
	}))

	data, bizErr := logic.Login(&types.UserLoginRequest{Username: "tester", Password: "123456"})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if data == nil {
		t.Fatal("expected login data")
	}
	if data.UserID != 12345 {
		t.Fatalf("expected user id 12345, got %d", data.UserID)
	}
	if data.Username != "tester" {
		t.Fatalf("expected username tester, got %s", data.Username)
	}
	if data.Role != "superadmin" {
		t.Fatalf("expected role superadmin, got %s", data.Role)
	}
	if !strings.HasPrefix(data.Token, "Bearer ") {
		t.Fatalf("expected bearer token, got %s", data.Token)
	}
}
