package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"

	"meshcart/app/common"
	orderpb "meshcart/kitex_gen/meshcart/order"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	"meshcart/services/payment-service/biz/errno"
	"meshcart/services/payment-service/biz/repository"
	dalmodel "meshcart/services/payment-service/dal/model"
	orderrpc "meshcart/services/payment-service/rpcclient/order"
)

type stubPaymentRepository struct {
	createFn                   func(context.Context, *dalmodel.Payment) (*dalmodel.Payment, error)
	getByPaymentIDFn           func(context.Context, int64) (*dalmodel.Payment, error)
	getByPaymentIDUserFn       func(context.Context, int64, int64) (*dalmodel.Payment, error)
	listByOrderIDFn            func(context.Context, int64, int64) ([]*dalmodel.Payment, error)
	getLatestActiveByOrderIDFn func(context.Context, int64, int64) (*dalmodel.Payment, error)
	transitionStatusFn         func(context.Context, repository.PaymentTransition) (*dalmodel.Payment, error)
	getActionRecordFn          func(context.Context, string, string) (*dalmodel.PaymentActionRecord, error)
	createActionRecordFn       func(context.Context, *dalmodel.PaymentActionRecord) error
	markActionOKFn             func(context.Context, string, string, int64, int64) error
	markActionFailFn           func(context.Context, string, string, string) error
}

func (s *stubPaymentRepository) Create(ctx context.Context, payment *dalmodel.Payment) (*dalmodel.Payment, error) {
	if s.createFn != nil {
		return s.createFn(ctx, payment)
	}
	return payment, nil
}
func (s *stubPaymentRepository) GetByPaymentID(ctx context.Context, paymentID int64) (*dalmodel.Payment, error) {
	if s.getByPaymentIDFn != nil {
		return s.getByPaymentIDFn(ctx, paymentID)
	}
	return nil, repository.ErrPaymentNotFound
}
func (s *stubPaymentRepository) GetByPaymentIDUser(ctx context.Context, paymentID, userID int64) (*dalmodel.Payment, error) {
	if s.getByPaymentIDUserFn != nil {
		return s.getByPaymentIDUserFn(ctx, paymentID, userID)
	}
	return nil, repository.ErrPaymentNotFound
}
func (s *stubPaymentRepository) ListByOrderID(ctx context.Context, orderID, userID int64) ([]*dalmodel.Payment, error) {
	if s.listByOrderIDFn != nil {
		return s.listByOrderIDFn(ctx, orderID, userID)
	}
	return []*dalmodel.Payment{}, nil
}
func (s *stubPaymentRepository) GetLatestActiveByOrderID(ctx context.Context, orderID, userID int64) (*dalmodel.Payment, error) {
	if s.getLatestActiveByOrderIDFn != nil {
		return s.getLatestActiveByOrderIDFn(ctx, orderID, userID)
	}
	return nil, repository.ErrPaymentNotFound
}
func (s *stubPaymentRepository) TransitionStatus(ctx context.Context, transition repository.PaymentTransition) (*dalmodel.Payment, error) {
	if s.transitionStatusFn != nil {
		return s.transitionStatusFn(ctx, transition)
	}
	return nil, repository.ErrPaymentStateConflict
}
func (s *stubPaymentRepository) GetActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.PaymentActionRecord, error) {
	if s.getActionRecordFn != nil {
		return s.getActionRecordFn(ctx, actionType, actionKey)
	}
	return nil, repository.ErrActionRecordNotFound
}
func (s *stubPaymentRepository) CreateActionRecord(ctx context.Context, record *dalmodel.PaymentActionRecord) error {
	if s.createActionRecordFn != nil {
		return s.createActionRecordFn(ctx, record)
	}
	return nil
}
func (s *stubPaymentRepository) MarkActionRecordSucceeded(ctx context.Context, actionType, actionKey string, paymentID, orderID int64) error {
	if s.markActionOKFn != nil {
		return s.markActionOKFn(ctx, actionType, actionKey, paymentID, orderID)
	}
	return nil
}
func (s *stubPaymentRepository) MarkActionRecordFailed(ctx context.Context, actionType, actionKey, errorMessage string) error {
	if s.markActionFailFn != nil {
		return s.markActionFailFn(ctx, actionType, actionKey, errorMessage)
	}
	return nil
}

type stubOrderClient struct {
	getOrderFn         func(context.Context, int64, int64) (*orderrpc.GetOrderResponse, error)
	confirmOrderPaidFn func(context.Context, *orderpb.ConfirmOrderPaidRequest) (*orderrpc.ConfirmOrderPaidResponse, error)
}

func (s *stubOrderClient) GetOrder(ctx context.Context, userID, orderID int64) (*orderrpc.GetOrderResponse, error) {
	if s.getOrderFn != nil {
		return s.getOrderFn(ctx, userID, orderID)
	}
	return &orderrpc.GetOrderResponse{}, nil
}
func (s *stubOrderClient) ConfirmOrderPaid(ctx context.Context, req *orderpb.ConfirmOrderPaidRequest) (*orderrpc.ConfirmOrderPaidResponse, error) {
	if s.confirmOrderPaidFn != nil {
		return s.confirmOrderPaidFn(ctx, req)
	}
	return &orderrpc.ConfirmOrderPaidResponse{}, nil
}

func newPaymentService(t *testing.T, repo repository.PaymentRepository, orderClient orderrpc.Client) *PaymentService {
	t.Helper()
	node, err := snowflake.NewNode(30)
	if err != nil {
		t.Fatalf("new snowflake node: %v", err)
	}
	svc := NewPaymentService(repo, node, orderClient)
	svc.nowFunc = func() time.Time { return time.Unix(1710000000, 0) }
	return svc
}

func TestPaymentService_CreatePayment_Success(t *testing.T) {
	svc := newPaymentService(t, &stubPaymentRepository{
		createFn: func(_ context.Context, payment *dalmodel.Payment) (*dalmodel.Payment, error) {
			if payment.OrderID != 10 || payment.UserID != 101 || payment.Amount != 1999 || payment.PaymentMethod != "mock" {
				t.Fatalf("unexpected payment create args: %+v", payment)
			}
			return payment, nil
		},
	}, &stubOrderClient{
		getOrderFn: func(_ context.Context, userID, orderID int64) (*orderrpc.GetOrderResponse, error) {
			return &orderrpc.GetOrderResponse{Code: 0, Order: &orderpb.Order{OrderId: orderID, UserId: userID, Status: 2, PayAmount: 1999}}, nil
		},
	})

	payment, bizErr := svc.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{OrderId: 10, UserId: 101, PaymentMethod: "mock"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if payment == nil || payment.GetOrderId() != 10 || payment.GetStatus() != PaymentStatusPending {
		t.Fatalf("unexpected payment: %+v", payment)
	}
}

func TestPaymentService_CreatePayment_ReturnActivePayment(t *testing.T) {
	svc := newPaymentService(t, &stubPaymentRepository{
		getLatestActiveByOrderIDFn: func(_ context.Context, orderID, userID int64) (*dalmodel.Payment, error) {
			return &dalmodel.Payment{PaymentID: 1, OrderID: orderID, UserID: userID, Status: PaymentStatusPending, PaymentMethod: "mock", Amount: 1999, Currency: "CNY"}, nil
		},
	}, &stubOrderClient{})

	payment, bizErr := svc.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{OrderId: 10, UserId: 101, PaymentMethod: "mock"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if payment.GetPaymentId() != 1 {
		t.Fatalf("unexpected active payment: %+v", payment)
	}
}

func TestPaymentService_ConfirmPaymentSuccess_Success(t *testing.T) {
	svc := newPaymentService(t, &stubPaymentRepository{
		getByPaymentIDFn: func(_ context.Context, paymentID int64) (*dalmodel.Payment, error) {
			return &dalmodel.Payment{PaymentID: paymentID, OrderID: 10, UserID: 101, Status: PaymentStatusPending, PaymentMethod: "mock", Amount: 1999, Currency: "CNY"}, nil
		},
		transitionStatusFn: func(_ context.Context, transition repository.PaymentTransition) (*dalmodel.Payment, error) {
			if transition.ToStatus != PaymentStatusSucceeded || transition.PaymentMethod != "mock" || transition.PaymentTradeNo != "mock-100" {
				t.Fatalf("unexpected transition: %+v", transition)
			}
			return &dalmodel.Payment{PaymentID: transition.PaymentID, OrderID: 10, UserID: 101, Status: PaymentStatusSucceeded, PaymentMethod: transition.PaymentMethod, PaymentTradeNo: transition.PaymentTradeNo, SucceededAt: transition.SucceededAt}, nil
		},
	}, &stubOrderClient{
		confirmOrderPaidFn: func(_ context.Context, req *orderpb.ConfirmOrderPaidRequest) (*orderrpc.ConfirmOrderPaidResponse, error) {
			if req.GetOrderId() != 10 || req.GetPaymentId() != "100" || req.GetPaymentMethod() != "mock" || req.GetPaymentTradeNo() != "mock-100" {
				t.Fatalf("unexpected confirm order paid req: %+v", req)
			}
			return &orderrpc.ConfirmOrderPaidResponse{Code: 0, Order: &orderpb.Order{OrderId: 10, Status: 3}}, nil
		},
	})

	payment, bizErr := svc.ConfirmPaymentSuccess(context.Background(), &paymentpb.ConfirmPaymentSuccessRequest{PaymentId: 100, PaymentMethod: "mock"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if payment == nil || payment.GetStatus() != PaymentStatusSucceeded || payment.GetPaymentTradeNo() != "mock-100" {
		t.Fatalf("unexpected payment: %+v", payment)
	}
}

func TestPaymentService_ConfirmPaymentSuccess_OrderRPCError(t *testing.T) {
	svc := newPaymentService(t, &stubPaymentRepository{
		getByPaymentIDFn: func(_ context.Context, paymentID int64) (*dalmodel.Payment, error) {
			return &dalmodel.Payment{PaymentID: paymentID, OrderID: 10, UserID: 101, Status: PaymentStatusPending, PaymentMethod: "mock", Amount: 1999, Currency: "CNY"}, nil
		},
	}, &stubOrderClient{
		confirmOrderPaidFn: func(context.Context, *orderpb.ConfirmOrderPaidRequest) (*orderrpc.ConfirmOrderPaidResponse, error) {
			return nil, errors.New("rpc down")
		},
	})

	payment, bizErr := svc.ConfirmPaymentSuccess(context.Background(), &paymentpb.ConfirmPaymentSuccessRequest{PaymentId: 100, PaymentMethod: "mock"})
	if payment != nil {
		t.Fatalf("expected nil payment, got %+v", payment)
	}
	if bizErr != common.ErrServiceUnavailable {
		t.Fatalf("expected service unavailable, got %+v", bizErr)
	}
}

func TestPaymentService_ConfirmPaymentSuccess_RetryAfterFailedActionRecord(t *testing.T) {
	svc := newPaymentService(t, &stubPaymentRepository{
		getActionRecordFn: func(_ context.Context, actionType, actionKey string) (*dalmodel.PaymentActionRecord, error) {
			return &dalmodel.PaymentActionRecord{ActionType: actionType, ActionKey: actionKey, PaymentID: 100, OrderID: 10, Status: "failed"}, nil
		},
		getByPaymentIDFn: func(_ context.Context, paymentID int64) (*dalmodel.Payment, error) {
			return &dalmodel.Payment{PaymentID: paymentID, OrderID: 10, UserID: 101, Status: PaymentStatusPending, PaymentMethod: "mock", Amount: 1999, Currency: "CNY"}, nil
		},
		transitionStatusFn: func(_ context.Context, transition repository.PaymentTransition) (*dalmodel.Payment, error) {
			return &dalmodel.Payment{PaymentID: transition.PaymentID, OrderID: 10, UserID: 101, Status: PaymentStatusSucceeded, PaymentMethod: transition.PaymentMethod, PaymentTradeNo: transition.PaymentTradeNo, SucceededAt: transition.SucceededAt}, nil
		},
	}, &stubOrderClient{
		confirmOrderPaidFn: func(_ context.Context, req *orderpb.ConfirmOrderPaidRequest) (*orderrpc.ConfirmOrderPaidResponse, error) {
			return &orderrpc.ConfirmOrderPaidResponse{Code: 0, Order: &orderpb.Order{OrderId: 10, Status: 3}}, nil
		},
	})

	payment, bizErr := svc.ConfirmPaymentSuccess(context.Background(), &paymentpb.ConfirmPaymentSuccessRequest{PaymentId: 100, PaymentMethod: "mock"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if payment == nil || payment.GetStatus() != PaymentStatusSucceeded {
		t.Fatalf("unexpected payment: %+v", payment)
	}
}

func TestPaymentService_GetPayment_NotFound(t *testing.T) {
	svc := newPaymentService(t, &stubPaymentRepository{}, &stubOrderClient{})
	payment, bizErr := svc.GetPayment(context.Background(), &paymentpb.GetPaymentRequest{PaymentId: 1, UserId: 101})
	if payment != nil {
		t.Fatalf("expected nil payment, got %+v", payment)
	}
	if bizErr != errno.ErrPaymentNotFound {
		t.Fatalf("expected ErrPaymentNotFound, got %+v", bizErr)
	}
}
