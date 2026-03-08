package user

import (
	"context"
	"errors"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	kitextrace "github.com/kitex-contrib/obs-opentelemetry/tracing"

	user "meshcart/kitex_gen/meshcart/user"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
)

var errNilLoginResponse = errors.New("user rpc returned nil login response")
var errNilRegisterResponse = errors.New("user rpc returned nil register response")

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

type RegisterRequest struct {
	Username string
	Password string
}

type RegisterResponse struct {
	Code    int32
	Message string
}

type Client interface {
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error)
}

type kitexClient struct {
	cli userservice.Client
}

func NewClient(serviceName, hostPort string) (Client, error) {
	cli, err := userservice.NewClient(
		serviceName,
		client.WithHostPorts(hostPort),
		client.WithSuite(kitextrace.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "gateway"}),
	)
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
		Code:    code,
		Message: message,
		// Current user.thrift only returns BaseResponse.
		// Keep data fields for forward compatibility after IDL extension.
		UserID:   0,
		Token:    "",
		Username: req.Username,
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
