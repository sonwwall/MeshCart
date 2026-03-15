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
	errNilGetSkuStockResponse        = errors.New("inventory rpc returned nil get sku stock response")
	errNilBatchGetSkuStockResponse   = errors.New("inventory rpc returned nil batch get sku stock response")
	errNilCheckSaleableStockResponse = errors.New("inventory rpc returned nil check saleable stock response")
	errNilInitSkuStocksResponse      = errors.New("inventory rpc returned nil init sku stocks response")
	errNilFreezeSkuStocksResponse    = errors.New("inventory rpc returned nil freeze sku stocks response")
	errNilAdjustStockResponse        = errors.New("inventory rpc returned nil adjust stock response")
)

type GetSkuStockResponse struct {
	Code    int32
	Message string
	Stock   *inventorypb.SkuStock
}

type BatchGetSkuStockResponse struct {
	Code    int32
	Message string
	Stocks  []*inventorypb.SkuStock
}

type CheckSaleableStockResponse struct {
	Code           int32
	Message        string
	Saleable       bool
	AvailableStock int64
}

type InitSkuStocksResponse struct {
	Code    int32
	Message string
	Stocks  []*inventorypb.SkuStock
}

type FreezeSkuStocksResponse struct {
	Code    int32
	Message string
	Stocks  []*inventorypb.SkuStock
}

type AdjustStockResponse struct {
	Code    int32
	Message string
	Stock   *inventorypb.SkuStock
}

type Client interface {
	GetSkuStock(ctx context.Context, req *inventorypb.GetSkuStockRequest) (*GetSkuStockResponse, error)
	BatchGetSkuStock(ctx context.Context, req *inventorypb.BatchGetSkuStockRequest) (*BatchGetSkuStockResponse, error)
	CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (*CheckSaleableStockResponse, error)
	InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) (*InitSkuStocksResponse, error)
	FreezeSkuStocks(ctx context.Context, req *inventorypb.FreezeSkuStocksRequest) (*FreezeSkuStocksResponse, error)
	AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*AdjustStockResponse, error)
}

type kitexClient struct {
	cli inventoryservice.Client
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
		return nil, fmt.Errorf("unsupported inventory rpc discovery type: %s", discoveryType)
	}
	opts = append(opts, extraOpts...)

	cli, err := inventoryservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return &kitexClient{cli: cli}, nil
}

func (c *kitexClient) GetSkuStock(ctx context.Context, req *inventorypb.GetSkuStockRequest) (*GetSkuStockResponse, error) {
	resp, err := c.cli.GetSkuStock(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilGetSkuStockResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &GetSkuStockResponse{Code: code, Message: message, Stock: resp.Stock}, nil
}

func (c *kitexClient) BatchGetSkuStock(ctx context.Context, req *inventorypb.BatchGetSkuStockRequest) (*BatchGetSkuStockResponse, error) {
	resp, err := c.cli.BatchGetSkuStock(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilBatchGetSkuStockResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &BatchGetSkuStockResponse{Code: code, Message: message, Stocks: resp.Stocks}, nil
}

func (c *kitexClient) CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (*CheckSaleableStockResponse, error) {
	resp, err := c.cli.CheckSaleableStock(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilCheckSaleableStockResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &CheckSaleableStockResponse{
		Code:           code,
		Message:        message,
		Saleable:       resp.GetSaleable(),
		AvailableStock: resp.GetAvailableStock(),
	}, nil
}

func (c *kitexClient) InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) (*InitSkuStocksResponse, error) {
	resp, err := c.cli.InitSkuStocks(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilInitSkuStocksResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &InitSkuStocksResponse{Code: code, Message: message, Stocks: resp.Stocks}, nil
}

func (c *kitexClient) FreezeSkuStocks(ctx context.Context, req *inventorypb.FreezeSkuStocksRequest) (*FreezeSkuStocksResponse, error) {
	resp, err := c.cli.FreezeSkuStocks(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilFreezeSkuStocksResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &FreezeSkuStocksResponse{Code: code, Message: message, Stocks: resp.Stocks}, nil
}

func (c *kitexClient) AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*AdjustStockResponse, error) {
	resp, err := c.cli.AdjustStock(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilAdjustStockResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &AdjustStockResponse{Code: code, Message: message, Stock: resp.Stock}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}
