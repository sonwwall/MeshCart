package payment

import (
	"context"
	"errors"
	"testing"

	callopt "github.com/cloudwego/kitex/client/callopt"
	basepb "meshcart/kitex_gen/meshcart/base"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	paymentservice "meshcart/kitex_gen/meshcart/payment/paymentservice"
)

type stubKitexPaymentClient struct {
	createPaymentFn         func(context.Context, *paymentpb.CreatePaymentRequest) (*paymentpb.CreatePaymentResponse, error)
	getPaymentFn            func(context.Context, *paymentpb.GetPaymentRequest) (*paymentpb.GetPaymentResponse, error)
	listPaymentsByOrderFn   func(context.Context, *paymentpb.ListPaymentsByOrderRequest) (*paymentpb.ListPaymentsByOrderResponse, error)
	confirmPaymentSuccessFn func(context.Context, *paymentpb.ConfirmPaymentSuccessRequest) (*paymentpb.ConfirmPaymentSuccessResponse, error)
	closePaymentFn          func(context.Context, *paymentpb.ClosePaymentRequest) (*paymentpb.ClosePaymentResponse, error)
}

var _ paymentservice.Client = (*stubKitexPaymentClient)(nil)

func (s *stubKitexPaymentClient) CreatePayment(ctx context.Context, request *paymentpb.CreatePaymentRequest, _ ...callopt.Option) (*paymentpb.CreatePaymentResponse, error) {
	return s.createPaymentFn(ctx, request)
}
func (s *stubKitexPaymentClient) GetPayment(ctx context.Context, request *paymentpb.GetPaymentRequest, _ ...callopt.Option) (*paymentpb.GetPaymentResponse, error) {
	return s.getPaymentFn(ctx, request)
}
func (s *stubKitexPaymentClient) ListPaymentsByOrder(ctx context.Context, request *paymentpb.ListPaymentsByOrderRequest, _ ...callopt.Option) (*paymentpb.ListPaymentsByOrderResponse, error) {
	return s.listPaymentsByOrderFn(ctx, request)
}
func (s *stubKitexPaymentClient) ConfirmPaymentSuccess(ctx context.Context, request *paymentpb.ConfirmPaymentSuccessRequest, _ ...callopt.Option) (*paymentpb.ConfirmPaymentSuccessResponse, error) {
	return s.confirmPaymentSuccessFn(ctx, request)
}
func (s *stubKitexPaymentClient) ClosePayment(ctx context.Context, request *paymentpb.ClosePaymentRequest, _ ...callopt.Option) (*paymentpb.ClosePaymentResponse, error) {
	return s.closePaymentFn(ctx, request)
}

func TestClient_CreatePayment_NilResponse(t *testing.T) {
	stub := &stubKitexPaymentClient{
		createPaymentFn: func(context.Context, *paymentpb.CreatePaymentRequest) (*paymentpb.CreatePaymentResponse, error) {
			return nil, nil
		},
	}
	c := &kitexClient{readCli: stub, writeCli: stub}

	resp, err := c.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, errNilCreatePaymentResponse) {
		t.Fatalf("expected errNilCreatePaymentResponse, got %v", err)
	}
}

func TestClient_GetPayment_MapsBaseResponse(t *testing.T) {
	stub := &stubKitexPaymentClient{
		getPaymentFn: func(context.Context, *paymentpb.GetPaymentRequest) (*paymentpb.GetPaymentResponse, error) {
			return &paymentpb.GetPaymentResponse{
				Base:    &basepb.BaseResponse{Code: 0, Message: "成功"},
				Payment: &paymentpb.Payment{PaymentId: 1, OrderId: 10, Status: 1},
			}, nil
		},
	}
	c := &kitexClient{readCli: stub, writeCli: stub}

	resp, err := c.GetPayment(context.Background(), &paymentpb.GetPaymentRequest{PaymentId: 1, UserId: 101})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Code != 0 || resp.Message != "成功" || resp.Payment == nil || resp.Payment.GetPaymentId() != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestClient_ListPaymentsByOrder_NilResponse(t *testing.T) {
	stub := &stubKitexPaymentClient{
		listPaymentsByOrderFn: func(context.Context, *paymentpb.ListPaymentsByOrderRequest) (*paymentpb.ListPaymentsByOrderResponse, error) {
			return nil, nil
		},
	}
	c := &kitexClient{readCli: stub, writeCli: stub}

	resp, err := c.ListPaymentsByOrder(context.Background(), &paymentpb.ListPaymentsByOrderRequest{})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, errNilListPaymentsByOrderResponse) {
		t.Fatalf("expected errNilListPaymentsByOrderResponse, got %v", err)
	}
}

func TestClient_ConfirmPaymentSuccess_MapsBaseResponse(t *testing.T) {
	stub := &stubKitexPaymentClient{
		confirmPaymentSuccessFn: func(context.Context, *paymentpb.ConfirmPaymentSuccessRequest) (*paymentpb.ConfirmPaymentSuccessResponse, error) {
			return &paymentpb.ConfirmPaymentSuccessResponse{
				Base:    &basepb.BaseResponse{Code: 0, Message: "成功"},
				Payment: &paymentpb.Payment{PaymentId: 1, Status: 2, PaymentTradeNo: "mock-1"},
			}, nil
		},
	}
	c := &kitexClient{readCli: stub, writeCli: stub}

	resp, err := c.ConfirmPaymentSuccess(context.Background(), &paymentpb.ConfirmPaymentSuccessRequest{PaymentId: 1, PaymentMethod: "mock"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Code != 0 || resp.Payment == nil || resp.Payment.GetStatus() != 2 || resp.Payment.GetPaymentTradeNo() != "mock-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestClient_ClosePayment_MapsBaseResponse(t *testing.T) {
	stub := &stubKitexPaymentClient{
		closePaymentFn: func(context.Context, *paymentpb.ClosePaymentRequest) (*paymentpb.ClosePaymentResponse, error) {
			return &paymentpb.ClosePaymentResponse{
				Base:    &basepb.BaseResponse{Code: 0, Message: "成功"},
				Payment: &paymentpb.Payment{PaymentId: 1, Status: 4, FailReason: "payment_closed"},
			}, nil
		},
	}
	c := &kitexClient{readCli: stub, writeCli: stub}

	resp, err := c.ClosePayment(context.Background(), &paymentpb.ClosePaymentRequest{PaymentId: 1, UserId: 101})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Code != 0 || resp.Payment == nil || resp.Payment.GetStatus() != 4 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
