package cart

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	kitextrace "github.com/kitex-contrib/obs-opentelemetry/tracing"
	consul "github.com/kitex-contrib/registry-consul"

	basepb "meshcart/kitex_gen/meshcart/base"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	cartservice "meshcart/kitex_gen/meshcart/cart/cartservice"
)

var (
	errNilGetCartResponse        = errors.New("cart rpc returned nil get cart response")
	errNilAddCartItemResponse    = errors.New("cart rpc returned nil add cart item response")
	errNilUpdateCartItemResponse = errors.New("cart rpc returned nil update cart item response")
	errNilRemoveCartItemResponse = errors.New("cart rpc returned nil remove cart item response")
	errNilClearCartResponse      = errors.New("cart rpc returned nil clear cart response")
)

type GetCartResponse struct {
	Code    int32
	Message string
	Items   []*cartpb.CartItem
}

type AddCartItemResponse struct {
	Code    int32
	Message string
	Item    *cartpb.CartItem
}

type UpdateCartItemResponse struct {
	Code    int32
	Message string
	Item    *cartpb.CartItem
}

type RemoveCartItemResponse struct {
	Code    int32
	Message string
}

type ClearCartResponse struct {
	Code    int32
	Message string
}

type Client interface {
	GetCart(ctx context.Context, req *cartpb.GetCartRequest) (*GetCartResponse, error)
	AddCartItem(ctx context.Context, req *cartpb.AddCartItemRequest) (*AddCartItemResponse, error)
	UpdateCartItem(ctx context.Context, req *cartpb.UpdateCartItemRequest) (*UpdateCartItemResponse, error)
	RemoveCartItem(ctx context.Context, req *cartpb.RemoveCartItemRequest) (*RemoveCartItemResponse, error)
	ClearCart(ctx context.Context, req *cartpb.ClearCartRequest) (*ClearCartResponse, error)
}

type kitexClient struct {
	cli cartservice.Client
}

func NewClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration) (Client, error) {
	return newClientWithOptions(serviceName, hostPort, discoveryType, consulAddress, connectTimeout, rpcTimeout)
}

func newClientWithOptions(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration, extraOpts ...client.Option) (Client, error) {
	opts := []client.Option{
		client.WithSuite(kitextrace.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "gateway"}),
	}
	if connectTimeout > 0 {
		opts = append(opts, client.WithConnectTimeout(connectTimeout))
	}
	if rpcTimeout > 0 {
		opts = append(opts, client.WithRPCTimeout(rpcTimeout))
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
		return nil, fmt.Errorf("unsupported cart rpc discovery type: %s", discoveryType)
	}
	opts = append(opts, extraOpts...)

	cli, err := cartservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return &kitexClient{cli: cli}, nil
}

func (c *kitexClient) GetCart(ctx context.Context, req *cartpb.GetCartRequest) (*GetCartResponse, error) {
	resp, err := c.cli.GetCart(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilGetCartResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &GetCartResponse{Code: code, Message: message, Items: resp.Items}, nil
}

func (c *kitexClient) AddCartItem(ctx context.Context, req *cartpb.AddCartItemRequest) (*AddCartItemResponse, error) {
	resp, err := c.cli.AddCartItem(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilAddCartItemResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &AddCartItemResponse{Code: code, Message: message, Item: resp.Item}, nil
}

func (c *kitexClient) UpdateCartItem(ctx context.Context, req *cartpb.UpdateCartItemRequest) (*UpdateCartItemResponse, error) {
	resp, err := c.cli.UpdateCartItem(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilUpdateCartItemResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &UpdateCartItemResponse{Code: code, Message: message, Item: resp.Item}, nil
}

func (c *kitexClient) RemoveCartItem(ctx context.Context, req *cartpb.RemoveCartItemRequest) (*RemoveCartItemResponse, error) {
	resp, err := c.cli.RemoveCartItem(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilRemoveCartItemResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &RemoveCartItemResponse{Code: code, Message: message}, nil
}

func (c *kitexClient) ClearCart(ctx context.Context, req *cartpb.ClearCartRequest) (*ClearCartResponse, error) {
	resp, err := c.cli.ClearCart(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilClearCartResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ClearCartResponse{Code: code, Message: message}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}
