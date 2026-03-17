package order

import (
	"context"
	"errors"
	"testing"

	callopt "github.com/cloudwego/kitex/client/callopt"
	basepb "meshcart/kitex_gen/meshcart/base"
	orderpb "meshcart/kitex_gen/meshcart/order"
	orderservice "meshcart/kitex_gen/meshcart/order/orderservice"
)

type stubKitexOrderClient struct {
	createOrderFn func(context.Context, *orderpb.CreateOrderRequest) (*orderpb.CreateOrderResponse, error)
	getOrderFn    func(context.Context, *orderpb.GetOrderRequest) (*orderpb.GetOrderResponse, error)
	listOrdersFn  func(context.Context, *orderpb.ListOrdersRequest) (*orderpb.ListOrdersResponse, error)
	cancelOrderFn func(context.Context, *orderpb.CancelOrderRequest) (*orderpb.CancelOrderResponse, error)
}

var _ orderservice.Client = (*stubKitexOrderClient)(nil)

func (s *stubKitexOrderClient) CreateOrder(ctx context.Context, request *orderpb.CreateOrderRequest, _ ...callopt.Option) (*orderpb.CreateOrderResponse, error) {
	return s.createOrderFn(ctx, request)
}

func (s *stubKitexOrderClient) GetOrder(ctx context.Context, request *orderpb.GetOrderRequest, _ ...callopt.Option) (*orderpb.GetOrderResponse, error) {
	return s.getOrderFn(ctx, request)
}

func (s *stubKitexOrderClient) ListOrders(ctx context.Context, request *orderpb.ListOrdersRequest, _ ...callopt.Option) (*orderpb.ListOrdersResponse, error) {
	return s.listOrdersFn(ctx, request)
}

func (s *stubKitexOrderClient) CancelOrder(ctx context.Context, request *orderpb.CancelOrderRequest, _ ...callopt.Option) (*orderpb.CancelOrderResponse, error) {
	return s.cancelOrderFn(ctx, request)
}

func (s *stubKitexOrderClient) ConfirmOrderPaid(context.Context, *orderpb.ConfirmOrderPaidRequest, ...callopt.Option) (*orderpb.ConfirmOrderPaidResponse, error) {
	return &orderpb.ConfirmOrderPaidResponse{}, nil
}

func (s *stubKitexOrderClient) CloseExpiredOrders(context.Context, *orderpb.CloseExpiredOrdersRequest, ...callopt.Option) (*orderpb.CloseExpiredOrdersResponse, error) {
	return &orderpb.CloseExpiredOrdersResponse{}, nil
}

func TestClient_CreateOrder_NilResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexOrderClient{
		createOrderFn: func(context.Context, *orderpb.CreateOrderRequest) (*orderpb.CreateOrderResponse, error) {
			return nil, nil
		},
	}}

	resp, err := c.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, errNilCreateOrderResponse) {
		t.Fatalf("expected errNilCreateOrderResponse, got %v", err)
	}
}

func TestClient_GetOrder_MapsBaseResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexOrderClient{
		getOrderFn: func(context.Context, *orderpb.GetOrderRequest) (*orderpb.GetOrderResponse, error) {
			return &orderpb.GetOrderResponse{
				Base:  &basepb.BaseResponse{Code: 0, Message: "成功"},
				Order: &orderpb.Order{OrderId: 1, UserId: 101, Status: 2},
			}, nil
		},
	}}

	resp, err := c.GetOrder(context.Background(), &orderpb.GetOrderRequest{UserId: 101, OrderId: 1})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Code != 0 || resp.Message != "成功" || resp.Order == nil || resp.Order.GetOrderId() != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestClient_ListOrders_NilResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexOrderClient{
		listOrdersFn: func(context.Context, *orderpb.ListOrdersRequest) (*orderpb.ListOrdersResponse, error) {
			return nil, nil
		},
	}}

	resp, err := c.ListOrders(context.Background(), &orderpb.ListOrdersRequest{})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, errNilListOrdersResponse) {
		t.Fatalf("expected errNilListOrdersResponse, got %v", err)
	}
}

func TestClient_CancelOrder_MapsBaseResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexOrderClient{
		cancelOrderFn: func(context.Context, *orderpb.CancelOrderRequest) (*orderpb.CancelOrderResponse, error) {
			return &orderpb.CancelOrderResponse{
				Base:  &basepb.BaseResponse{Code: 0, Message: "成功"},
				Order: &orderpb.Order{OrderId: 1, UserId: 101, Status: 4, CancelReason: "changed_mind"},
			}, nil
		},
	}}

	resp, err := c.CancelOrder(context.Background(), &orderpb.CancelOrderRequest{UserId: 101, OrderId: 1})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Code != 0 || resp.Order == nil || resp.Order.GetStatus() != 4 || resp.Order.GetCancelReason() != "changed_mind" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
