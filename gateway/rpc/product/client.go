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

	basepb "meshcart/kitex_gen/meshcart/base"
	productpb "meshcart/kitex_gen/meshcart/product"
	productservice "meshcart/kitex_gen/meshcart/product/productservice"
)

var (
	errNilCreateResponse       = errors.New("product rpc returned nil create response")
	errNilUpdateResponse       = errors.New("product rpc returned nil update response")
	errNilChangeStatusResponse = errors.New("product rpc returned nil change status response")
	errNilDetailResponse       = errors.New("product rpc returned nil detail response")
	errNilListResponse         = errors.New("product rpc returned nil list response")
	errNilBatchGetSKUResponse  = errors.New("product rpc returned nil batch get sku response")
)

type CreateProductResponse struct {
	Code      int32
	Message   string
	ProductID int64
	Skus      []*productpb.ProductSku
}

type UpdateProductResponse struct {
	Code    int32
	Message string
	Skus    []*productpb.ProductSku
}

type ChangeProductStatusResponse struct {
	Code    int32
	Message string
}

type GetProductDetailResponse struct {
	Code    int32
	Message string
	Product *productpb.Product
}

type ListProductsResponse struct {
	Code     int32
	Message  string
	Products []*productpb.ProductListItem
	Total    int64
}

type BatchGetSKUResponse struct {
	Code    int32
	Message string
	Skus    []*productpb.ProductSku
}

type Client interface {
	CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*CreateProductResponse, error)
	UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) (*UpdateProductResponse, error)
	ChangeProductStatus(ctx context.Context, req *productpb.ChangeProductStatusRequest) (*ChangeProductStatusResponse, error)
	GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*GetProductDetailResponse, error)
	ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*ListProductsResponse, error)
	BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*BatchGetSKUResponse, error)
}

type kitexClient struct {
	cli productservice.Client
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
		return nil, fmt.Errorf("unsupported product rpc discovery type: %s", discoveryType)
	}
	opts = append(opts, extraOpts...)

	cli, err := productservice.NewClient(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	return &kitexClient{cli: cli}, nil
}

func (c *kitexClient) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*CreateProductResponse, error) {
	resp, err := c.cli.CreateProduct(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilCreateResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &CreateProductResponse{Code: code, Message: message, ProductID: resp.ProductId, Skus: resp.Skus}, nil
}

func (c *kitexClient) UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) (*UpdateProductResponse, error) {
	resp, err := c.cli.UpdateProduct(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilUpdateResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &UpdateProductResponse{Code: code, Message: message, Skus: resp.Skus}, nil
}

func (c *kitexClient) ChangeProductStatus(ctx context.Context, req *productpb.ChangeProductStatusRequest) (*ChangeProductStatusResponse, error) {
	resp, err := c.cli.ChangeProductStatus(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilChangeStatusResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ChangeProductStatusResponse{Code: code, Message: message}, nil
}

func (c *kitexClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*GetProductDetailResponse, error) {
	resp, err := c.cli.GetProductDetail(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilDetailResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &GetProductDetailResponse{Code: code, Message: message, Product: resp.Product}, nil
}

func (c *kitexClient) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*ListProductsResponse, error) {
	resp, err := c.cli.ListProducts(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilListResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ListProductsResponse{
		Code:     code,
		Message:  message,
		Products: resp.Products,
		Total:    resp.Total,
	}, nil
}

func (c *kitexClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*BatchGetSKUResponse, error) {
	resp, err := c.cli.BatchGetSku(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilBatchGetSKUResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &BatchGetSKUResponse{
		Code:    code,
		Message: message,
		Skus:    resp.Skus,
	}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}
