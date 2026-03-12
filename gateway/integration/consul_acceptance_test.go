package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	consulapi "github.com/hashicorp/consul/api"
	consul "github.com/kitex-contrib/registry-consul"

	"meshcart/app/common"
	userrpc "meshcart/gateway/rpc/user"
	basepb "meshcart/kitex_gen/meshcart/base"
	userpb "meshcart/kitex_gen/meshcart/user"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
)

func TestConsulAcceptance_UserRPCDiscovery(t *testing.T) {
	consulAddr := os.Getenv("CONSUL_ADDR")
	if consulAddr == "" {
		consulAddr = "127.0.0.1:8500"
	}

	consulClient := newConsulClient(t, consulAddr)
	if _, err := consulClient.Agent().Self(); err != nil {
		t.Skipf("skip consul acceptance test: consul unavailable at %s: %v", consulAddr, err)
	}

	serviceAddr := reserveKitexAddr(t)
	serviceName := fmt.Sprintf("meshcart.user.acceptance.%d", time.Now().UnixNano())
	checkID := fmt.Sprintf("service:%s:%s", serviceName, serviceAddr.String())

	registry, err := consul.NewConsulRegister(consulAddr, consul.WithCheck(&consulapi.AgentServiceCheck{
		CheckID:                        checkID,
		TTL:                            "10s",
		DeregisterCriticalServiceAfter: "1m",
	}))
	if err != nil {
		t.Fatalf("init consul registry: %v", err)
	}

	svr := userservice.NewServer(
		&acceptanceUserService{},
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
		server.WithServiceAddr(serviceAddr),
		server.WithRegistry(registry),
	)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- svr.Run()
	}()

	waitForServer(t, serviceAddr.String())
	waitForConsulService(t, consulClient, serviceName)

	t.Cleanup(func() {
		_ = svr.Stop()
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Fatalf("consul acceptance server exited with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for consul acceptance server shutdown")
		}
	})

	client, err := userrpc.NewClient(serviceName, "", "consul", consulAddr, 100*time.Millisecond, time.Second)
	if err != nil {
		t.Fatalf("new consul discovery client: %v", err)
	}

	resp, err := client.Login(context.Background(), &userrpc.LoginRequest{
		Username: "tester",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("login via consul discovery: %v", err)
	}
	if resp.Code != common.CodeOK {
		t.Fatalf("expected code 0, got %d message=%q", resp.Code, resp.Message)
	}
	if resp.UserID != 301 {
		t.Fatalf("expected user_id 301, got %d", resp.UserID)
	}
	if resp.Username != "tester" {
		t.Fatalf("expected username tester, got %q", resp.Username)
	}
}

type acceptanceUserService struct{}

func (s *acceptanceUserService) Login(_ context.Context, request *userpb.UserLoginRequest) (*userpb.UserLoginResponse, error) {
	return &userpb.UserLoginResponse{
		UserId:   301,
		Username: request.GetUsername(),
		Role:     "user",
		Base:     &basepb.BaseResponse{Code: common.CodeOK, Message: "成功"},
	}, nil
}

func (s *acceptanceUserService) Register(_ context.Context, _ *userpb.UserRegisterRequest) (*userpb.UserRegisterResponse, error) {
	return &userpb.UserRegisterResponse{Base: &basepb.BaseResponse{Code: common.CodeOK, Message: "成功"}}, nil
}

func (s *acceptanceUserService) GetUser(_ context.Context, _ *userpb.UserGetRequest) (*userpb.UserGetResponse, error) {
	return &userpb.UserGetResponse{Base: &basepb.BaseResponse{Code: common.CodeOK, Message: "成功"}}, nil
}

func (s *acceptanceUserService) UpdateUserRole(_ context.Context, _ *userpb.UserUpdateRoleRequest) (*userpb.UserUpdateRoleResponse, error) {
	return &userpb.UserUpdateRoleResponse{Base: &basepb.BaseResponse{Code: common.CodeOK, Message: "成功"}}, nil
}

func newConsulClient(t *testing.T, addr string) *consulapi.Client {
	t.Helper()

	cfg := consulapi.DefaultConfig()
	cfg.Address = addr
	client, err := consulapi.NewClient(cfg)
	if err != nil {
		t.Fatalf("new consul client: %v", err)
	}
	return client
}

func reserveKitexAddr(t *testing.T) *net.TCPAddr {
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

func waitForConsulService(t *testing.T, client *consulapi.Client, serviceName string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		entries, _, err := client.Health().Service(serviceName, "", true, nil)
		if err == nil && len(entries) > 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("service %s did not become healthy in consul", serviceName)
}
