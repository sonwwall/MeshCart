package user

import (
	"context"
	"errors"

	tracex "meshcart/app/trace"

	"github.com/cloudwego/kitex/client"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	user "meshcart/kitex_gen/meshcart/user"
	userservice "meshcart/kitex_gen/meshcart/user/userservice"
)

var errNilLoginResponse = errors.New("user rpc returned nil login response")

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

type kitexClient struct {
	cli userservice.Client
}

func NewClient(serviceName, hostPort string) (Client, error) {
	cli, err := userservice.NewClient(
		serviceName,
		client.WithHostPorts(hostPort),
	)
	if err != nil {
		return nil, err
	}
	return &kitexClient{cli: cli}, nil
}

func (c *kitexClient) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	ctx, span := tracex.StartSpan(ctx, "meshcart.gateway", "gateway.rpc.user.login", oteltrace.WithSpanKind(oteltrace.SpanKindClient))
	defer span.End()

	resp, err := c.cli.Login(ctx, &user.UserLoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "kitex login call failed")
		return nil, err
	}
	if resp == nil {
		span.SetStatus(codes.Error, errNilLoginResponse.Error())
		return nil, errNilLoginResponse
	}

	var code int32
	var message string
	if resp.Base != nil {
		code = resp.Base.Code
		message = resp.Base.Message
	}
	if code == 0 {
		span.SetStatus(codes.Ok, "ok")
	} else {
		span.SetStatus(codes.Error, message)
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
