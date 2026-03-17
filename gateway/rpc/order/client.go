package order

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
	orderpb "meshcart/kitex_gen/meshcart/order"
	orderservice "meshcart/kitex_gen/meshcart/order/orderservice"
)

var (
	errNilCreateOrderResponse = errors.New("order rpc returned nil create order response")
	errNilGetOrderResponse    = errors.New("order rpc returned nil get order response")
	errNilListOrdersResponse  = errors.New("order rpc returned nil list orders response")
	errNilCancelOrderResponse = errors.New("order rpc returned nil cancel order response")
)

type CreateOrderResponse struct {
	Code    int32
	Message string
	Order   *orderpb.Order
}

type GetOrderResponse struct {
	Code    int32
	Message string
	Order   *orderpb.Order
}

type ListOrdersResponse struct {
	Code    int32
	Message string
	Orders  []*orderpb.Order
	Total   int64
}

type CancelOrderResponse struct {
	Code    int32
	Message string
	Order   *orderpb.Order
}

type Client interface {
	CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*CreateOrderResponse, error)
	GetOrder(ctx context.Context, req *orderpb.GetOrderRequest) (*GetOrderResponse, error)
	ListOrders(ctx context.Context, req *orderpb.ListOrdersRequest) (*ListOrdersResponse, error)
	CancelOrder(ctx context.Context, req *orderpb.CancelOrderRequest) (*CancelOrderResponse, error)
}

type kitexClient struct {
	cli orderservice.Client
}

func NewClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration) (Client, error) {
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
		return nil, fmt.Errorf("unsupported order rpc discovery type: %s", discoveryType)
	}

	cli, err := orderservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return &kitexClient{cli: cli}, nil
}

func (c *kitexClient) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*CreateOrderResponse, error) {
	resp, err := c.cli.CreateOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilCreateOrderResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &CreateOrderResponse{Code: code, Message: message, Order: resp.Order}, nil
}

func (c *kitexClient) GetOrder(ctx context.Context, req *orderpb.GetOrderRequest) (*GetOrderResponse, error) {
	resp, err := c.cli.GetOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilGetOrderResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &GetOrderResponse{Code: code, Message: message, Order: resp.Order}, nil
}

func (c *kitexClient) ListOrders(ctx context.Context, req *orderpb.ListOrdersRequest) (*ListOrdersResponse, error) {
	resp, err := c.cli.ListOrders(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilListOrdersResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ListOrdersResponse{Code: code, Message: message, Orders: resp.Orders, Total: resp.Total}, nil
}

func (c *kitexClient) CancelOrder(ctx context.Context, req *orderpb.CancelOrderRequest) (*CancelOrderResponse, error) {
	resp, err := c.cli.CancelOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilCancelOrderResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &CancelOrderResponse{Code: code, Message: message, Order: resp.Order}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}
