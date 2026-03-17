package inventory

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
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	inventoryservice "meshcart/kitex_gen/meshcart/inventory/inventoryservice"
)

var (
	errNilReserveResponse = errors.New("inventory rpc returned nil reserve response")
	errNilReleaseResponse = errors.New("inventory rpc returned nil release response")
	errNilConfirmResponse = errors.New("inventory rpc returned nil confirm deduct response")
)

type ReserveSkuStocksResponse struct {
	Code    int32
	Message string
	Stocks  []*inventorypb.SkuStock
}

type ReleaseReservedSkuStocksResponse struct {
	Code    int32
	Message string
	Stocks  []*inventorypb.SkuStock
}

type ConfirmDeductReservedSkuStocksResponse struct {
	Code    int32
	Message string
	Stocks  []*inventorypb.SkuStock
}

type Client interface {
	ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) (*ReserveSkuStocksResponse, error)
	ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*ReleaseReservedSkuStocksResponse, error)
	ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*ConfirmDeductReservedSkuStocksResponse, error)
}

type kitexClient struct {
	cli inventoryservice.Client
}

func NewClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration) (Client, error) {
	opts := []client.Option{
		client.WithSuite(kitextrace.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "order-service"}),
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
		return nil, fmt.Errorf("unsupported inventory rpc discovery type: %s", discoveryType)
	}

	cli, err := inventoryservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return &kitexClient{cli: cli}, nil
}

func (c *kitexClient) ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) (*ReserveSkuStocksResponse, error) {
	resp, err := c.cli.ReserveSkuStocks(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilReserveResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ReserveSkuStocksResponse{Code: code, Message: message, Stocks: resp.Stocks}, nil
}

func (c *kitexClient) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*ReleaseReservedSkuStocksResponse, error) {
	resp, err := c.cli.ReleaseReservedSkuStocks(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilReleaseResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ReleaseReservedSkuStocksResponse{Code: code, Message: message, Stocks: resp.Stocks}, nil
}

func (c *kitexClient) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*ConfirmDeductReservedSkuStocksResponse, error) {
	resp, err := c.cli.ConfirmDeductReservedSkuStocks(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilConfirmResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ConfirmDeductReservedSkuStocksResponse{Code: code, Message: message, Stocks: resp.Stocks}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}
