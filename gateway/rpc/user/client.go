package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	kitextrace "github.com/kitex-contrib/obs-opentelemetry/tracing"
	consul "github.com/kitex-contrib/registry-consul"

	user "meshcart/kitex_gen/meshcart/user"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
)

var errNilLoginResponse = errors.New("user rpc returned nil login response")
var errNilRegisterResponse = errors.New("user rpc returned nil register response")
var errNilGetUserResponse = errors.New("user rpc returned nil get user response")
var errNilUpdateRoleResponse = errors.New("user rpc returned nil update role response")

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
	Role     string
}

type RegisterRequest struct {
	Username string
	Password string
}

type RegisterResponse struct {
	Code    int32
	Message string
}

type GetUserRequest struct {
	UserID int64
}

type GetUserResponse struct {
	Code     int32
	Message  string
	UserID   int64
	Username string
	Role     string
	IsLocked bool
}

type UpdateUserRoleRequest struct {
	UserID int64
	Role   string
}

type UpdateUserRoleResponse struct {
	Code    int32
	Message string
}

type Client interface {
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error)
	GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error)
	UpdateUserRole(ctx context.Context, req *UpdateUserRoleRequest) (*UpdateUserRoleResponse, error)
}

type kitexClient struct {
	cli userservice.Client
}

func NewClient(serviceName, hostPort, discoveryType, consulAddress string) (Client, error) {
	opts := []client.Option{
		client.WithSuite(kitextrace.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "gateway"}),
	}

	switch strings.ToLower(discoveryType) {
	case "", "direct":
		opts = append(opts, client.WithHostPorts(hostPort))
	case "consul":
		resolver, err := consul.NewConsulResolver(consulAddress)
		if err != nil {
			return nil, fmt.Errorf("create consul resolver: %w", err)
		}
		opts = append(opts, client.WithResolver(resolver))
	default:
		return nil, fmt.Errorf("unsupported user rpc discovery type: %s", discoveryType)
	}

	cli, err := userservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return &kitexClient{cli: cli}, nil
}

func (c *kitexClient) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	resp, err := c.cli.Login(ctx, &user.UserLoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilLoginResponse
	}

	var code int32
	var message string
	if resp.Base != nil {
		code = resp.Base.Code
		message = resp.Base.Message
	}
	return &LoginResponse{
		Code:     code,
		Message:  message,
		UserID:   resp.UserId,
		Token:    "",
		Username: resp.Username,
		Role:     resp.Role,
	}, nil
}

func (c *kitexClient) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	resp, err := c.cli.Register(ctx, &user.UserRegisterRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilRegisterResponse
	}

	var code int32
	var message string
	if resp.Base != nil {
		code = resp.Base.Code
		message = resp.Base.Message
	}
	return &RegisterResponse{
		Code:    code,
		Message: message,
	}, nil
}

func (c *kitexClient) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	resp, err := c.cli.GetUser(ctx, &user.UserGetRequest{UserId: req.UserID})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilGetUserResponse
	}

	var code int32
	var message string
	if resp.Base != nil {
		code = resp.Base.Code
		message = resp.Base.Message
	}
	return &GetUserResponse{
		Code:     code,
		Message:  message,
		UserID:   resp.UserId,
		Username: resp.Username,
		Role:     resp.Role,
		IsLocked: resp.IsLocked,
	}, nil
}

func (c *kitexClient) UpdateUserRole(ctx context.Context, req *UpdateUserRoleRequest) (*UpdateUserRoleResponse, error) {
	resp, err := c.cli.UpdateUserRole(ctx, &user.UserUpdateRoleRequest{
		UserId: req.UserID,
		Role:   req.Role,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilUpdateRoleResponse
	}

	var code int32
	var message string
	if resp.Base != nil {
		code = resp.Base.Code
		message = resp.Base.Message
	}
	return &UpdateUserRoleResponse{
		Code:    code,
		Message: message,
	}, nil
}
