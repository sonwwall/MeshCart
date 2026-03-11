package user

import (
	"context"
	"errors"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"
)

func TestRegister_InvalidParam(t *testing.T) {
	logic := NewRegisterLogic(context.Background(), newTestServiceContext(t, &stubUserClient{}))

	bizErr := logic.Register(&types.UserRegisterRequest{Username: " ", Password: ""})
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestRegister_RPCError(t *testing.T) {
	logic := NewRegisterLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		registerFn: func(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error) {
			return nil, errors.New("rpc failed")
		},
	}))

	bizErr := logic.Register(&types.UserRegisterRequest{Username: "tester", Password: "123456"})
	if bizErr != common.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %+v", bizErr)
	}
}

func TestRegister_RPCTimeoutError(t *testing.T) {
	logic := NewRegisterLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		registerFn: func(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error) {
			return nil, context.DeadlineExceeded
		},
	}))

	bizErr := logic.Register(&types.UserRegisterRequest{Username: "tester", Password: "123456"})
	if bizErr != common.ErrServiceBusy {
		t.Fatalf("expected ErrServiceBusy, got %+v", bizErr)
	}
}

func TestRegister_RPCServiceUnavailable(t *testing.T) {
	logic := NewRegisterLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		registerFn: func(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error) {
			return nil, errors.New("dial tcp 127.0.0.1:8888: connect: connection refused")
		},
	}))

	bizErr := logic.Register(&types.UserRegisterRequest{Username: "tester", Password: "123456"})
	if bizErr != common.ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable, got %+v", bizErr)
	}
}

func TestRegister_BusinessError(t *testing.T) {
	logic := NewRegisterLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		registerFn: func(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error) {
			return &userrpc.RegisterResponse{Code: 2010004, Message: "用户名已存在"}, nil
		},
	}))

	bizErr := logic.Register(&types.UserRegisterRequest{Username: "tester", Password: "123456"})
	if bizErr == nil || bizErr.Code != 2010004 {
		t.Fatalf("expected business error 2010004, got %+v", bizErr)
	}
}

func TestRegister_Success(t *testing.T) {
	logic := NewRegisterLogic(context.Background(), newTestServiceContext(t, &stubUserClient{
		registerFn: func(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error) {
			return &userrpc.RegisterResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	bizErr := logic.Register(&types.UserRegisterRequest{Username: "tester", Password: "123456"})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
}
