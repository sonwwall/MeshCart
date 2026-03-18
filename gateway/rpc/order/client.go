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

	commonx "meshcart/app/common"
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
	readCli  orderservice.Client
	writeCli orderservice.Client
}

func NewClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration) (Client, error) {
	readCli, err := newRawClientWithOptions(serviceName, hostPort, discoveryType, consulAddress, connectTimeout, rpcTimeout, client.WithFailureRetry(commonx.NewReadFailureRetryPolicy(rpcTimeout)))
	if err != nil {
		return nil, err
	}
	writeCli, err := newRawClientWithOptions(serviceName, hostPort, discoveryType, consulAddress, connectTimeout, rpcTimeout)
	if err != nil {
		return nil, err
	}
	return &kitexClient{readCli: readCli, writeCli: writeCli}, nil
}

func newClientWithOptions(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration, extraOpts ...client.Option) (Client, error) {
	cli, err := newRawClientWithOptions(serviceName, hostPort, discoveryType, consulAddress, connectTimeout, rpcTimeout, extraOpts...)
	if err != nil {
		return nil, err
	}
	return &kitexClient{readCli: cli, writeCli: cli}, nil
}

func newRawClientWithOptions(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration, extraOpts ...client.Option) (orderservice.Client, error) {
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
	opts = append(opts, extraOpts...)

	cli, err := orderservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (c *kitexClient) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*CreateOrderResponse, error) {
	resp, err := c.writeCli.CreateOrder(ctx, req)
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
	resp, err := c.readCli.GetOrder(ctx, req)
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
	resp, err := c.readCli.ListOrders(ctx, req)
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
	resp, err := c.writeCli.CancelOrder(ctx, req)
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
