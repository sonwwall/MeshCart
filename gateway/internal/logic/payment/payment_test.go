package payment

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	paymentrpc "meshcart/gateway/rpc/payment"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
)

type stubPaymentClient struct {
	createPaymentFn         func(context.Context, *paymentpb.CreatePaymentRequest) (*paymentrpc.CreatePaymentResponse, error)
	getPaymentFn            func(context.Context, *paymentpb.GetPaymentRequest) (*paymentrpc.GetPaymentResponse, error)
	listPaymentsByOrderFn   func(context.Context, *paymentpb.ListPaymentsByOrderRequest) (*paymentrpc.ListPaymentsByOrderResponse, error)
	confirmPaymentSuccessFn func(context.Context, *paymentpb.ConfirmPaymentSuccessRequest) (*paymentrpc.ConfirmPaymentSuccessResponse, error)
}

func (s *stubPaymentClient) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*paymentrpc.CreatePaymentResponse, error) {
	if s.createPaymentFn != nil {
		return s.createPaymentFn(ctx, req)
	}
	return &paymentrpc.CreatePaymentResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubPaymentClient) GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*paymentrpc.GetPaymentResponse, error) {
	if s.getPaymentFn != nil {
		return s.getPaymentFn(ctx, req)
	}
	return &paymentrpc.GetPaymentResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubPaymentClient) ListPaymentsByOrder(ctx context.Context, req *paymentpb.ListPaymentsByOrderRequest) (*paymentrpc.ListPaymentsByOrderResponse, error) {
	if s.listPaymentsByOrderFn != nil {
		return s.listPaymentsByOrderFn(ctx, req)
	}
	return &paymentrpc.ListPaymentsByOrderResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubPaymentClient) ConfirmPaymentSuccess(ctx context.Context, req *paymentpb.ConfirmPaymentSuccessRequest) (*paymentrpc.ConfirmPaymentSuccessResponse, error) {
	if s.confirmPaymentSuccessFn != nil {
		return s.confirmPaymentSuccessFn(ctx, req)
	}
	return &paymentrpc.ConfirmPaymentSuccessResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func newPaymentTestServiceContext(t *testing.T, paymentClient paymentrpc.Client) *svc.ServiceContext {
	t.Helper()

	jwtMiddleware, err := middleware.NewJWT(config.JWTConfig{
		Secret:            "test-secret",
		Issuer:            "meshcart.gateway",
		TimeoutMinutes:    120,
		MaxRefreshMinutes: 720,
	})
	if err != nil {
		t.Fatalf("create jwt middleware: %v", err)
	}
	accessController, err := authz.NewAccessController()
	if err != nil {
		t.Fatalf("create access controller: %v", err)
	}

	return &svc.ServiceContext{
		PaymentClient: paymentClient,
		JWT:           jwtMiddleware,
		AccessControl: accessController,
	}
}

func TestCreateLogic_Success(t *testing.T) {
	logic := NewCreateLogic(context.Background(), newPaymentTestServiceContext(t, &stubPaymentClient{
		createPaymentFn: func(_ context.Context, req *paymentpb.CreatePaymentRequest) (*paymentrpc.CreatePaymentResponse, error) {
			if req.GetUserId() != 101 || req.GetOrderId() != 10 || req.GetPaymentMethod() != "mock" {
				t.Fatalf("unexpected create payment req: %+v", req)
			}
			return &paymentrpc.CreatePaymentResponse{Code: common.CodeOK, Payment: &paymentpb.Payment{PaymentId: 1, OrderId: 10, UserId: 101, Status: 1, PaymentMethod: "mock", Amount: 1999, Currency: "CNY"}}, nil
		},
	}))

	data, bizErr := logic.Create(101, &types.CreatePaymentRequest{OrderID: 10, PaymentMethod: "mock", RequestID: "pay-req-1"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.PaymentID != 1 || data.PaymentMethod != "mock" {
		t.Fatalf("unexpected payment data: %+v", data)
	}
}

func TestGetLogic_Success(t *testing.T) {
	logic := NewGetLogic(context.Background(), newPaymentTestServiceContext(t, &stubPaymentClient{
		getPaymentFn: func(_ context.Context, req *paymentpb.GetPaymentRequest) (*paymentrpc.GetPaymentResponse, error) {
			if req.GetUserId() != 101 || req.GetPaymentId() != 1 {
				t.Fatalf("unexpected get payment req: %+v", req)
			}
			return &paymentrpc.GetPaymentResponse{Code: common.CodeOK, Payment: &paymentpb.Payment{PaymentId: 1, OrderId: 10, UserId: 101, Status: 1}}, nil
		},
	}))

	data, bizErr := logic.Get(101, 1)
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.PaymentID != 1 {
		t.Fatalf("unexpected payment data: %+v", data)
	}
}

func TestListByOrderLogic_Success(t *testing.T) {
	logic := NewListByOrderLogic(context.Background(), newPaymentTestServiceContext(t, &stubPaymentClient{
		listPaymentsByOrderFn: func(_ context.Context, req *paymentpb.ListPaymentsByOrderRequest) (*paymentrpc.ListPaymentsByOrderResponse, error) {
			if req.GetUserId() != 101 || req.GetOrderId() != 10 {
				t.Fatalf("unexpected list payments req: %+v", req)
			}
			return &paymentrpc.ListPaymentsByOrderResponse{Code: common.CodeOK, Payments: []*paymentpb.Payment{{PaymentId: 1, OrderId: 10, UserId: 101, Status: 1, PaymentMethod: "mock"}}}, nil
		},
	}))

	data, bizErr := logic.ListByOrder(101, 10)
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || len(data.Payments) != 1 || data.Payments[0].PaymentID != 1 {
		t.Fatalf("unexpected list data: %+v", data)
	}
}

func TestMockSuccessLogic_Success(t *testing.T) {
	logic := NewMockSuccessLogic(context.Background(), newPaymentTestServiceContext(t, &stubPaymentClient{
		confirmPaymentSuccessFn: func(_ context.Context, req *paymentpb.ConfirmPaymentSuccessRequest) (*paymentrpc.ConfirmPaymentSuccessResponse, error) {
			if req.GetPaymentId() != 1 || req.GetPaymentMethod() != "mock" || req.GetPaymentTradeNo() != "trade-1" {
				t.Fatalf("unexpected confirm payment req: %+v", req)
			}
			return &paymentrpc.ConfirmPaymentSuccessResponse{Code: common.CodeOK, Payment: &paymentpb.Payment{PaymentId: 1, Status: 2, PaymentMethod: "mock", PaymentTradeNo: "trade-1"}}, nil
		},
	}))

	data, bizErr := logic.Confirm(101, 1, &types.ConfirmMockPaymentRequest{PaymentTradeNo: "trade-1"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.Status != 2 || data.PaymentTradeNo != "trade-1" {
		t.Fatalf("unexpected payment data: %+v", data)
	}
}
