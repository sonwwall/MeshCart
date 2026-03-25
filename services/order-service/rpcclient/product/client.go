package product

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
	productpb "meshcart/kitex_gen/meshcart/product"
	productservice "meshcart/kitex_gen/meshcart/product/productservice"
)

var (
	errNilDetailResponse      = errors.New("product rpc returned nil detail response")
	errNilBatchProductsResp   = errors.New("product rpc returned nil batch get products response")
	errNilBatchGetSKUResponse = errors.New("product rpc returned nil batch get sku response")
)

type GetProductDetailResponse struct {
	Code    int32
	Message string
	Product *productpb.Product
}

type BatchGetSKUResponse struct {
	Code    int32
	Message string
	Skus    []*productpb.ProductSku
}

type BatchGetProductsResponse struct {
	Code     int32
	Message  string
	Products []*productpb.Product
}

type Client interface {
	GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*GetProductDetailResponse, error)
	BatchGetProducts(ctx context.Context, req *productpb.BatchGetProductsRequest) (*BatchGetProductsResponse, error)
	BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*BatchGetSKUResponse, error)
}

type kitexClient struct {
	readCli productservice.Client
}

func NewClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration) (Client, error) {
	cli, err := newRawClient(serviceName, hostPort, discoveryType, consulAddress, connectTimeout, rpcTimeout, client.WithFailureRetry(commonx.NewReadFailureRetryPolicy(rpcTimeout)))
	if err != nil {
		return nil, err
	}
	return &kitexClient{readCli: cli}, nil
}

func newRawClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration, extraOpts ...client.Option) (productservice.Client, error) {
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
		return nil, fmt.Errorf("unsupported product rpc discovery type: %s", discoveryType)
	}
	opts = append(opts, extraOpts...)

	cli, err := productservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (c *kitexClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*GetProductDetailResponse, error) {
	resp, err := c.readCli.GetProductDetail(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilDetailResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &GetProductDetailResponse{Code: code, Message: message, Product: resp.Product}, nil
}

func (c *kitexClient) BatchGetProducts(ctx context.Context, req *productpb.BatchGetProductsRequest) (*BatchGetProductsResponse, error) {
	resp, err := c.readCli.BatchGetProducts(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilBatchProductsResp
	}
	code, message := baseCodeMessage(resp.Base)
	return &BatchGetProductsResponse{Code: code, Message: message, Products: resp.Products}, nil
}

func (c *kitexClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*BatchGetSKUResponse, error) {
	resp, err := c.readCli.BatchGetSku(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilBatchGetSKUResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &BatchGetSKUResponse{Code: code, Message: message, Skus: resp.Skus}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}
