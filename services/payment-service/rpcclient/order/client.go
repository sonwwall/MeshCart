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
	errNilGetOrderResponse         = errors.New("order rpc returned nil get order response")
	errNilConfirmOrderPaidResponse = errors.New("order rpc returned nil confirm order paid response")
)

type GetOrderResponse struct {
	Code    int32
	Message string
	Order   *orderpb.Order
}

type ConfirmOrderPaidResponse struct {
	Code    int32
	Message string
	Order   *orderpb.Order
}

type Client interface {
	GetOrder(ctx context.Context, userID, orderID int64) (*GetOrderResponse, error)
	ConfirmOrderPaid(ctx context.Context, req *orderpb.ConfirmOrderPaidRequest) (*ConfirmOrderPaidResponse, error)
}

type kitexClient struct {
	readCli          orderservice.Client
	writeCli         orderservice.Client
	fallbackReadCli  orderservice.Client
	fallbackWriteCli orderservice.Client
}

func NewClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration) (Client, error) {
	readCli, err := newRawClient(serviceName, hostPort, discoveryType, consulAddress, connectTimeout, rpcTimeout, client.WithFailureRetry(commonx.NewReadFailureRetryPolicy(rpcTimeout)))
	if err != nil {
		return nil, err
	}
	writeCli, err := newRawClient(serviceName, hostPort, discoveryType, consulAddress, connectTimeout, rpcTimeout)
	if err != nil {
		return nil, err
	}
	result := &kitexClient{readCli: readCli, writeCli: writeCli}
	if shouldBuildDirectFallback(discoveryType, hostPort) {
		fallbackReadCli, err := newRawClient(serviceName, hostPort, "direct", consulAddress, connectTimeout, rpcTimeout, client.WithFailureRetry(commonx.NewReadFailureRetryPolicy(rpcTimeout)))
		if err != nil {
			return nil, err
		}
		fallbackWriteCli, err := newRawClient(serviceName, hostPort, "direct", consulAddress, connectTimeout, rpcTimeout)
		if err != nil {
			return nil, err
		}
		result.fallbackReadCli = fallbackReadCli
		result.fallbackWriteCli = fallbackWriteCli
	}
	return result, nil
}

func newRawClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration, extraOpts ...client.Option) (orderservice.Client, error) {
	opts := []client.Option{
		client.WithSuite(kitextrace.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "payment-service"}),
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

	return orderservice.NewClient(serviceName, opts...)
}

func (c *kitexClient) GetOrder(ctx context.Context, userID, orderID int64) (*GetOrderResponse, error) {
	resp, err := c.readCli.GetOrder(ctx, &orderpb.GetOrderRequest{UserId: userID, OrderId: orderID})
	if err != nil && shouldFallbackToDirect(err) && c.fallbackReadCli != nil {
		resp, err = c.fallbackReadCli.GetOrder(ctx, &orderpb.GetOrderRequest{UserId: userID, OrderId: orderID})
	}
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilGetOrderResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &GetOrderResponse{Code: code, Message: message, Order: resp.Order}, nil
}

func (c *kitexClient) ConfirmOrderPaid(ctx context.Context, req *orderpb.ConfirmOrderPaidRequest) (*ConfirmOrderPaidResponse, error) {
	resp, err := c.writeCli.ConfirmOrderPaid(ctx, req)
	if err != nil && shouldFallbackToDirect(err) && c.fallbackWriteCli != nil {
		resp, err = c.fallbackWriteCli.ConfirmOrderPaid(ctx, req)
	}
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilConfirmOrderPaidResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ConfirmOrderPaidResponse{Code: code, Message: message, Order: resp.Order}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}

func shouldBuildDirectFallback(discoveryType, hostPort string) bool {
	return strings.EqualFold(discoveryType, "consul") && strings.TrimSpace(hostPort) != ""
}

func shouldFallbackToDirect(err error) bool {
	if err == nil {
		return false
	}
	lowerErr := strings.ToLower(err.Error())
	return strings.Contains(lowerErr, "no service found") || strings.Contains(lowerErr, "service discovery error")
}
