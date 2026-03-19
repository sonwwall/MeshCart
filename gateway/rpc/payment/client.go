package payment

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
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	paymentservice "meshcart/kitex_gen/meshcart/payment/paymentservice"
)

var (
	errNilCreatePaymentResponse         = errors.New("payment rpc returned nil create payment response")
	errNilGetPaymentResponse            = errors.New("payment rpc returned nil get payment response")
	errNilListPaymentsByOrderResponse   = errors.New("payment rpc returned nil list payments by order response")
	errNilConfirmPaymentSuccessResponse = errors.New("payment rpc returned nil confirm payment success response")
)

type CreatePaymentResponse struct {
	Code    int32
	Message string
	Payment *paymentpb.Payment
}

type GetPaymentResponse struct {
	Code    int32
	Message string
	Payment *paymentpb.Payment
}

type ListPaymentsByOrderResponse struct {
	Code     int32
	Message  string
	Payments []*paymentpb.Payment
}

type ConfirmPaymentSuccessResponse struct {
	Code    int32
	Message string
	Payment *paymentpb.Payment
}

type Client interface {
	CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*CreatePaymentResponse, error)
	GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*GetPaymentResponse, error)
	ListPaymentsByOrder(ctx context.Context, req *paymentpb.ListPaymentsByOrderRequest) (*ListPaymentsByOrderResponse, error)
	ConfirmPaymentSuccess(ctx context.Context, req *paymentpb.ConfirmPaymentSuccessRequest) (*ConfirmPaymentSuccessResponse, error)
}

type kitexClient struct {
	readCli  paymentservice.Client
	writeCli paymentservice.Client
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
	return &kitexClient{readCli: readCli, writeCli: writeCli}, nil
}

func newRawClient(serviceName, hostPort, discoveryType, consulAddress string, connectTimeout, rpcTimeout time.Duration, extraOpts ...client.Option) (paymentservice.Client, error) {
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
		return nil, fmt.Errorf("unsupported payment rpc discovery type: %s", discoveryType)
	}
	opts = append(opts, extraOpts...)

	return paymentservice.NewClient(serviceName, opts...)
}

func (c *kitexClient) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*CreatePaymentResponse, error) {
	resp, err := c.writeCli.CreatePayment(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilCreatePaymentResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &CreatePaymentResponse{Code: code, Message: message, Payment: resp.Payment}, nil
}

func (c *kitexClient) GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*GetPaymentResponse, error) {
	resp, err := c.readCli.GetPayment(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilGetPaymentResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &GetPaymentResponse{Code: code, Message: message, Payment: resp.Payment}, nil
}

func (c *kitexClient) ListPaymentsByOrder(ctx context.Context, req *paymentpb.ListPaymentsByOrderRequest) (*ListPaymentsByOrderResponse, error) {
	resp, err := c.readCli.ListPaymentsByOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilListPaymentsByOrderResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ListPaymentsByOrderResponse{Code: code, Message: message, Payments: resp.Payments}, nil
}

func (c *kitexClient) ConfirmPaymentSuccess(ctx context.Context, req *paymentpb.ConfirmPaymentSuccessRequest) (*ConfirmPaymentSuccessResponse, error) {
	resp, err := c.writeCli.ConfirmPaymentSuccess(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errNilConfirmPaymentSuccessResponse
	}
	code, message := baseCodeMessage(resp.Base)
	return &ConfirmPaymentSuccessResponse{Code: code, Message: message, Payment: resp.Payment}, nil
}

func baseCodeMessage(base *basepb.BaseResponse) (int32, string) {
	if base == nil {
		return 0, ""
	}
	return base.GetCode(), base.GetMessage()
}
