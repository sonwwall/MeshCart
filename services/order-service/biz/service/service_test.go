package service

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"

	"meshcart/app/common"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/errno"
	"meshcart/services/order-service/biz/repository"
	dalmodel "meshcart/services/order-service/dal/model"
)

type stubOrderRepository struct {
	createWithItemsFn func(context.Context, *dalmodel.Order, []*dalmodel.OrderItem) (*dalmodel.Order, error)
	getByOrderIDFn    func(context.Context, int64, int64) (*dalmodel.Order, error)
	listByUserIDFn    func(context.Context, int64, int, int) ([]*dalmodel.Order, int64, error)
}

func (s *stubOrderRepository) CreateWithItems(ctx context.Context, order *dalmodel.Order, items []*dalmodel.OrderItem) (*dalmodel.Order, error) {
	if s.createWithItemsFn != nil {
		return s.createWithItemsFn(ctx, order, items)
	}
	return order, nil
}

func (s *stubOrderRepository) GetByOrderID(ctx context.Context, userID, orderID int64) (*dalmodel.Order, error) {
	if s.getByOrderIDFn != nil {
		return s.getByOrderIDFn(ctx, userID, orderID)
	}
	return nil, repository.ErrOrderNotFound
}

func (s *stubOrderRepository) ListByUserID(ctx context.Context, userID int64, offset, limit int) ([]*dalmodel.Order, int64, error) {
	if s.listByUserIDFn != nil {
		return s.listByUserIDFn(ctx, userID, offset, limit)
	}
	return []*dalmodel.Order{}, 0, nil
}

func newOrderService(t *testing.T, repo repository.OrderRepository) *OrderService {
	t.Helper()

	node, err := snowflake.NewNode(10)
	if err != nil {
		t.Fatalf("new snowflake node: %v", err)
	}
	return NewOrderService(repo, node)
}

func TestOrderService_CreateOrder_Success(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		createWithItemsFn: func(_ context.Context, order *dalmodel.Order, items []*dalmodel.OrderItem) (*dalmodel.Order, error) {
			if order.UserID != 101 || order.Status != OrderStatusPending || len(items) != 1 {
				t.Fatalf("unexpected create args order=%+v items=%+v", order, items)
			}
			return &dalmodel.Order{
				OrderID:     order.OrderID,
				UserID:      order.UserID,
				Status:      order.Status,
				TotalAmount: order.TotalAmount,
				PayAmount:   order.PayAmount,
				ExpireAt:    time.Now().Add(30 * time.Minute),
				Items:       []dalmodel.OrderItem{*items[0]},
			}, nil
		},
	})

	order, bizErr := svc.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: 101,
		Items: []*orderpb.OrderItemInput{{
			ProductId:            2001,
			SkuId:                3001,
			ProductTitleSnapshot: "MeshCart Tee",
			SkuTitleSnapshot:     "Blue XL",
			SalePriceSnapshot:    1999,
			Quantity:             2,
		}},
	})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if order == nil || order.GetUserId() != 101 || order.GetTotalAmount() != 3998 || len(order.GetItems()) != 1 {
		t.Fatalf("unexpected order: %+v", order)
	}
}

func TestOrderService_GetOrder_NotFound(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{})

	order, bizErr := svc.GetOrder(context.Background(), &orderpb.GetOrderRequest{UserId: 101, OrderId: 1})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr != errno.ErrOrderNotFound {
		t.Fatalf("expected ErrOrderNotFound, got %+v", bizErr)
	}
}

func TestOrderService_ListOrders_InvalidParam(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{})

	orders, total, bizErr := svc.ListOrders(context.Background(), &orderpb.ListOrdersRequest{})
	if orders != nil || total != 0 {
		t.Fatalf("unexpected result orders=%+v total=%d", orders, total)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}
