package order

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	orderrpc "meshcart/gateway/rpc/order"
	orderpb "meshcart/kitex_gen/meshcart/order"
)

type stubOrderClient struct {
	createOrderFn func(context.Context, *orderpb.CreateOrderRequest) (*orderrpc.CreateOrderResponse, error)
	getOrderFn    func(context.Context, *orderpb.GetOrderRequest) (*orderrpc.GetOrderResponse, error)
	listOrdersFn  func(context.Context, *orderpb.ListOrdersRequest) (*orderrpc.ListOrdersResponse, error)
	cancelOrderFn func(context.Context, *orderpb.CancelOrderRequest) (*orderrpc.CancelOrderResponse, error)
}

func (s *stubOrderClient) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderrpc.CreateOrderResponse, error) {
	if s.createOrderFn != nil {
		return s.createOrderFn(ctx, req)
	}
	return &orderrpc.CreateOrderResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubOrderClient) GetOrder(ctx context.Context, req *orderpb.GetOrderRequest) (*orderrpc.GetOrderResponse, error) {
	if s.getOrderFn != nil {
		return s.getOrderFn(ctx, req)
	}
	return &orderrpc.GetOrderResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubOrderClient) ListOrders(ctx context.Context, req *orderpb.ListOrdersRequest) (*orderrpc.ListOrdersResponse, error) {
	if s.listOrdersFn != nil {
		return s.listOrdersFn(ctx, req)
	}
	return &orderrpc.ListOrdersResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubOrderClient) CancelOrder(ctx context.Context, req *orderpb.CancelOrderRequest) (*orderrpc.CancelOrderResponse, error) {
	if s.cancelOrderFn != nil {
		return s.cancelOrderFn(ctx, req)
	}
	return &orderrpc.CancelOrderResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func newOrderTestServiceContext(t *testing.T, orderClient orderrpc.Client) *svc.ServiceContext {
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
		OrderClient:   orderClient,
		JWT:           jwtMiddleware,
		AccessControl: accessController,
	}
}

func TestCreateLogic_Success(t *testing.T) {
	logic := NewCreateLogic(context.Background(), newOrderTestServiceContext(t, &stubOrderClient{
		createOrderFn: func(_ context.Context, req *orderpb.CreateOrderRequest) (*orderrpc.CreateOrderResponse, error) {
			if req.GetUserId() != 101 || len(req.GetItems()) != 1 || req.GetItems()[0].GetSkuId() != 3001 {
				t.Fatalf("unexpected create order req: %+v", req)
			}
			return &orderrpc.CreateOrderResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Order: &orderpb.Order{
					OrderId:     1,
					UserId:      101,
					Status:      2,
					TotalAmount: 3998,
					Items: []*orderpb.OrderItem{
						{ItemId: 11, OrderId: 1, ProductId: 2001, SkuId: 3001, Quantity: 2},
					},
				},
			}, nil
		},
	}))

	data, bizErr := logic.Create(101, &types.CreateOrderRequest{
		RequestID: "req-1",
		Items: []types.CreateOrderItemInput{
			{ProductID: 2001, SKUID: 3001, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.OrderID != 1 || len(data.Items) != 1 {
		t.Fatalf("unexpected order data: %+v", data)
	}
}

func TestGetLogic_Success(t *testing.T) {
	logic := NewGetLogic(context.Background(), newOrderTestServiceContext(t, &stubOrderClient{
		getOrderFn: func(_ context.Context, req *orderpb.GetOrderRequest) (*orderrpc.GetOrderResponse, error) {
			if req.GetUserId() != 101 || req.GetOrderId() != 1 {
				t.Fatalf("unexpected get order req: %+v", req)
			}
			return &orderrpc.GetOrderResponse{
				Code: common.CodeOK,
				Order: &orderpb.Order{
					OrderId: 1,
					UserId:  101,
					Status:  2,
				},
			}, nil
		},
	}))

	data, bizErr := logic.Get(101, 1)
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.OrderID != 1 {
		t.Fatalf("unexpected order data: %+v", data)
	}
}

func TestListLogic_DefaultPagination(t *testing.T) {
	logic := NewListLogic(context.Background(), newOrderTestServiceContext(t, &stubOrderClient{
		listOrdersFn: func(_ context.Context, req *orderpb.ListOrdersRequest) (*orderrpc.ListOrdersResponse, error) {
			if req.GetUserId() != 101 || req.GetPage() != 1 || req.GetPageSize() != 20 {
				t.Fatalf("unexpected list orders req: %+v", req)
			}
			return &orderrpc.ListOrdersResponse{
				Code: common.CodeOK,
				Orders: []*orderpb.Order{{
					OrderId:     1,
					UserId:      101,
					Status:      2,
					TotalAmount: 3998,
					Items: []*orderpb.OrderItem{
						{ItemId: 11, OrderId: 1, ProductId: 2001, SkuId: 3001, Quantity: 2},
					},
				}},
				Total: 1,
			}, nil
		},
	}))

	data, bizErr := logic.List(101, &types.ListOrdersRequest{})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.Total != 1 || len(data.Orders) != 1 {
		t.Fatalf("unexpected list data: %+v", data)
	}
	if data.Orders[0].OrderID != 1 || data.Orders[0].ItemCount != 1 {
		t.Fatalf("unexpected order summary: %+v", data.Orders[0])
	}
}

func TestCancelLogic_Success(t *testing.T) {
	logic := NewCancelLogic(context.Background(), newOrderTestServiceContext(t, &stubOrderClient{
		cancelOrderFn: func(_ context.Context, req *orderpb.CancelOrderRequest) (*orderrpc.CancelOrderResponse, error) {
			if req.GetUserId() != 101 || req.GetOrderId() != 1 || req.GetCancelReason() != "changed_mind" {
				t.Fatalf("unexpected cancel order req: %+v", req)
			}
			return &orderrpc.CancelOrderResponse{
				Code:  common.CodeOK,
				Order: &orderpb.Order{OrderId: 1, UserId: 101, Status: 4, CancelReason: "changed_mind"},
			}, nil
		},
	}))

	data, bizErr := logic.Cancel(101, 1, &types.CancelOrderRequest{CancelReason: "changed_mind"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.Status != 4 || data.CancelReason != "changed_mind" {
		t.Fatalf("unexpected cancel data: %+v", data)
	}
}
