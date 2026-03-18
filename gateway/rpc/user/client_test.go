package user

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/server"

	basepb "meshcart/kitex_gen/meshcart/base"
	userpb "meshcart/kitex_gen/meshcart/user"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
)

type slowUserService struct {
	sleep   time.Duration
	started chan struct{}
}

func (s *slowUserService) Login(ctx context.Context, request *userpb.UserLoginRequest) (*userpb.UserLoginResponse, error) {
	select {
	case s.started <- struct{}{}:
	default:
	}
	time.Sleep(s.sleep)
	return &userpb.UserLoginResponse{
		UserId:   1,
		Username: request.GetUsername(),
		Role:     "user",
		Base:     &basepb.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}

func (s *slowUserService) Register(ctx context.Context, request *userpb.UserRegisterRequest) (*userpb.UserRegisterResponse, error) {
	return &userpb.UserRegisterResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowUserService) GetUser(ctx context.Context, request *userpb.UserGetRequest) (*userpb.UserGetResponse, error) {
	return &userpb.UserGetResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowUserService) UpdateUserRole(ctx context.Context, request *userpb.UserUpdateRoleRequest) (*userpb.UserUpdateRoleResponse, error) {
	return &userpb.UserUpdateRoleResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func TestClient_LoginTimeout(t *testing.T) {
	addr := acquireFreeAddr(t)
	svc := &slowUserService{
		sleep:   200 * time.Millisecond,
		started: make(chan struct{}, 1),
	}
	svr := userservice.NewServer(svc, server.WithServiceAddr(addr))

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- svr.Run()
	}()

	waitForServer(t, addr.String())
	t.Cleanup(func() {
		if err := svr.Stop(); err != nil {
			t.Fatalf("stop test server: %v", err)
		}
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Fatalf("server exited with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for test server shutdown")
		}
	})

	client, err := NewClient("UserService", addr.String(), "direct", "", 100*time.Millisecond, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	start := time.Now()
	_, err = client.Login(context.Background(), &LoginRequest{
		Username: "tester",
		Password: "123456",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected rpc timeout error")
	}
	t.Logf("rpc returned expected timeout error: %v", err)
	if elapsed >= svc.sleep {
		t.Fatalf("expected timeout before server completed, elapsed=%s sleep=%s", elapsed, svc.sleep)
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("expected timeout close to configured rpc timeout, got elapsed=%s", elapsed)
	}
	select {
	case <-svc.started:
	default:
		t.Fatal("expected server handler to start")
	}
	if !isTimeoutError(err) {
		t.Fatalf("expected timeout-like error, got %v", err)
	}
}

func TestClient_LoginConnectTimeout(t *testing.T) {
	client, err := newClientWithOptions(
		"UserService",
		"127.0.0.1:1",
		"direct",
		"",
		50*time.Millisecond,
		2*time.Second,
		client.WithDialer(remote.SynthesizedDialer{
			DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
				time.Sleep(timeout + 20*time.Millisecond)
				return nil, context.DeadlineExceeded
			},
		}),
		client.WithFailureRetry(noRetryPolicy()),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	start := time.Now()
	_, err = client.Login(context.Background(), &LoginRequest{
		Username: "tester",
		Password: "123456",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected connect timeout error")
	}
	t.Logf("rpc returned expected connect timeout error: %v", err)
	if elapsed >= 2*time.Second {
		t.Fatalf("expected connect timeout before rpc timeout budget was exhausted, got elapsed=%s", elapsed)
	}
	if !isTimeoutError(err) {
		t.Fatalf("expected timeout-like error, got %v", err)
	}
}

func acquireFreeAddr(t *testing.T) *net.TCPAddr {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free addr: %v", err)
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected addr type %T", ln.Addr())
	}
	return addr
}

func waitForServer(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s", addr)
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "exceed max duration")
}

func noRetryPolicy() *retry.FailurePolicy {
	p := retry.NewFailurePolicy()
	p.WithMaxRetryTimes(0)
	return p
}
