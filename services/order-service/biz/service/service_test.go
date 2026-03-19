package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/order-service/biz/errno"
	"meshcart/services/order-service/biz/repository"
	dalmodel "meshcart/services/order-service/dal/model"
	inventoryrpc "meshcart/services/order-service/rpcclient/inventory"
	productrpc "meshcart/services/order-service/rpcclient/product"
)

type stubOrderRepository struct {
	createWithItemsFn     func(context.Context, *dalmodel.Order, []*dalmodel.OrderItem) (*dalmodel.Order, error)
	getByOrderIDFn        func(context.Context, int64, int64) (*dalmodel.Order, error)
	getByIDFn             func(context.Context, int64) (*dalmodel.Order, error)
	listByUserIDFn        func(context.Context, int64, int, int) ([]*dalmodel.Order, int64, error)
	updateStatusFn        func(context.Context, int64, []int32, int32, string) (*dalmodel.Order, error)
	listExpiredFn         func(context.Context, time.Time, int) ([]*dalmodel.Order, error)
	transitionStatusFn    func(context.Context, repository.OrderTransition) (*dalmodel.Order, error)
	getActionRecordFn     func(context.Context, string, string) (*dalmodel.OrderActionRecord, error)
	createActionRecordFn  func(context.Context, *dalmodel.OrderActionRecord) error
	markActionSucceededFn func(context.Context, string, string, int64) error
	markActionFailedFn    func(context.Context, string, string, string) error
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

func (s *stubOrderRepository) GetByID(ctx context.Context, orderID int64) (*dalmodel.Order, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, orderID)
	}
	return nil, repository.ErrOrderNotFound
}

func (s *stubOrderRepository) ListByUserID(ctx context.Context, userID int64, offset, limit int) ([]*dalmodel.Order, int64, error) {
	if s.listByUserIDFn != nil {
		return s.listByUserIDFn(ctx, userID, offset, limit)
	}
	return []*dalmodel.Order{}, 0, nil
}

func (s *stubOrderRepository) UpdateStatus(ctx context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error) {
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, orderID, fromStatuses, toStatus, cancelReason)
	}
	return nil, repository.ErrOrderStateConflict
}

func (s *stubOrderRepository) ListExpiredOrders(ctx context.Context, now time.Time, limit int) ([]*dalmodel.Order, error) {
	if s.listExpiredFn != nil {
		return s.listExpiredFn(ctx, now, limit)
	}
	return []*dalmodel.Order{}, nil
}

func (s *stubOrderRepository) TransitionStatus(ctx context.Context, transition repository.OrderTransition) (*dalmodel.Order, error) {
	if s.transitionStatusFn != nil {
		return s.transitionStatusFn(ctx, transition)
	}
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, transition.OrderID, transition.FromStatuses, transition.ToStatus, transition.CancelReason)
	}
	return nil, repository.ErrOrderStateConflict
}

func (s *stubOrderRepository) GetActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.OrderActionRecord, error) {
	if s.getActionRecordFn != nil {
		return s.getActionRecordFn(ctx, actionType, actionKey)
	}
	return nil, repository.ErrActionRecordNotFound
}

func (s *stubOrderRepository) CreateActionRecord(ctx context.Context, record *dalmodel.OrderActionRecord) error {
	if s.createActionRecordFn != nil {
		return s.createActionRecordFn(ctx, record)
	}
	return nil
}

func (s *stubOrderRepository) MarkActionRecordSucceeded(ctx context.Context, actionType, actionKey string, orderID int64) error {
	if s.markActionSucceededFn != nil {
		return s.markActionSucceededFn(ctx, actionType, actionKey, orderID)
	}
	return nil
}

func (s *stubOrderRepository) MarkActionRecordFailed(ctx context.Context, actionType, actionKey, errorMessage string) error {
	if s.markActionFailedFn != nil {
		return s.markActionFailedFn(ctx, actionType, actionKey, errorMessage)
	}
	return nil
}

type stubProductClient struct {
	batchGetSKUFn      func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error)
	getProductDetailFn func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error)
}

func (s *stubProductClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
	if s.batchGetSKUFn != nil {
		return s.batchGetSKUFn(ctx, req)
	}
	return &productrpc.BatchGetSKUResponse{}, nil
}

func (s *stubProductClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
	if s.getProductDetailFn != nil {
		return s.getProductDetailFn(ctx, req)
	}
	return &productrpc.GetProductDetailResponse{}, nil
}

type stubInventoryClient struct {
	reserveFn func(context.Context, *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error)
	releaseFn func(context.Context, *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error)
	confirmFn func(context.Context, *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error)
}

func (s *stubInventoryClient) ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
	if s.reserveFn != nil {
		return s.reserveFn(ctx, req)
	}
	return &inventoryrpc.ReserveSkuStocksResponse{}, nil
}

func (s *stubInventoryClient) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
	if s.releaseFn != nil {
		return s.releaseFn(ctx, req)
	}
	return &inventoryrpc.ReleaseReservedSkuStocksResponse{}, nil
}

func (s *stubInventoryClient) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error) {
	if s.confirmFn != nil {
		return s.confirmFn(ctx, req)
	}
	return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{}, nil
}

func newOrderService(t *testing.T, repo repository.OrderRepository, productClient productrpc.Client, inventoryClient inventoryrpc.Client) *OrderService {
	t.Helper()

	node, err := snowflake.NewNode(10)
	if err != nil {
		t.Fatalf("new snowflake node: %v", err)
	}
	svc := NewOrderService(repo, node, productClient, inventoryClient)
	svc.nowFunc = func() time.Time { return time.Unix(1710000000, 0) }
	return svc
}

func TestOrderService_CreateOrder_Success(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		createWithItemsFn: func(_ context.Context, order *dalmodel.Order, items []*dalmodel.OrderItem) (*dalmodel.Order, error) {
			if order.UserID != 101 || order.Status != OrderStatusReserved || len(items) != 1 {
				t.Fatalf("unexpected create args order=%+v items=%+v", order, items)
			}
			return &dalmodel.Order{
				OrderID:      order.OrderID,
				UserID:       order.UserID,
				Status:       order.Status,
				TotalAmount:  order.TotalAmount,
				PayAmount:    order.PayAmount,
				ExpireAt:     order.ExpireAt,
				CancelReason: order.CancelReason,
				Items:        []dalmodel.OrderItem{*items[0]},
			}, nil
		},
	}, &stubProductClient{
		batchGetSKUFn: func(_ context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			if len(req.GetSkuIds()) != 1 || req.GetSkuIds()[0] != 3001 {
				t.Fatalf("unexpected batch get sku req: %+v", req)
			}
			return &productrpc.BatchGetSKUResponse{Code: 0, Skus: []*productpb.ProductSku{{Id: 3001, SpuId: 2001, Title: "Blue XL", SalePrice: 1999, Status: 1}}}, nil
		},
		getProductDetailFn: func(_ context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			if req.GetProductId() != 2001 {
				t.Fatalf("unexpected product detail req: %+v", req)
			}
			return &productrpc.GetProductDetailResponse{Code: 0, Product: &productpb.Product{Id: 2001, Title: "MeshCart Tee", Status: 2}}, nil
		},
	}, &stubInventoryClient{
		reserveFn: func(_ context.Context, req *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
			if req.GetBizType() != "order" || len(req.GetItems()) != 1 || req.GetItems()[0].GetSkuId() != 3001 || req.GetItems()[0].GetQuantity() != 2 {
				t.Fatalf("unexpected reserve req: %+v", req)
			}
			return &inventoryrpc.ReserveSkuStocksResponse{Code: 0}, nil
		},
	})

	order, bizErr := svc.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: 101,
		Items: []*orderpb.OrderItemInput{{
			ProductId: 2001,
			SkuId:     3001,
			Quantity:  2,
		}},
	})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if order == nil || order.GetUserId() != 101 || order.GetTotalAmount() != 3998 || len(order.GetItems()) != 1 {
		t.Fatalf("unexpected order: %+v", order)
	}
	if order.GetStatus() != OrderStatusReserved || order.GetItems()[0].GetProductTitleSnapshot() != "MeshCart Tee" {
		t.Fatalf("unexpected reserved order: %+v", order)
	}
}

func TestOrderService_GetOrder_NotFound(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{}, &stubProductClient{}, &stubInventoryClient{})

	order, bizErr := svc.GetOrder(context.Background(), &orderpb.GetOrderRequest{UserId: 101, OrderId: 1})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr != errno.ErrOrderNotFound {
		t.Fatalf("expected ErrOrderNotFound, got %+v", bizErr)
	}
}

func TestOrderService_ListOrders_InvalidParam(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{}, &stubProductClient{}, &stubInventoryClient{})

	orders, total, bizErr := svc.ListOrders(context.Background(), &orderpb.ListOrdersRequest{})
	if orders != nil || total != 0 {
		t.Fatalf("unexpected result orders=%+v total=%d", orders, total)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestOrderService_CreateOrder_ReserveFailed(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{}, &stubProductClient{
		batchGetSKUFn: func(_ context.Context, _ *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			return &productrpc.BatchGetSKUResponse{Code: 0, Skus: []*productpb.ProductSku{{Id: 3001, SpuId: 2001, Title: "Blue XL", SalePrice: 1999, Status: 1}}}, nil
		},
		getProductDetailFn: func(_ context.Context, _ *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{Code: 0, Product: &productpb.Product{Id: 2001, Title: "MeshCart Tee", Status: 2}}, nil
		},
	}, &stubInventoryClient{
		reserveFn: func(_ context.Context, _ *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
			return &inventoryrpc.ReserveSkuStocksResponse{Code: 2050002, Message: "库存不足"}, nil
		},
	})

	order, bizErr := svc.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: 101,
		Items:  []*orderpb.OrderItemInput{{ProductId: 2001, SkuId: 3001, Quantity: 1}},
	})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr != errno.ErrOrderInsufficientStock {
		t.Fatalf("expected ErrOrderInsufficientStock, got %+v", bizErr)
	}
}

func TestOrderService_CreateOrder_CreateFailedReleaseReserved(t *testing.T) {
	released := false
	svc := newOrderService(t, &stubOrderRepository{
		createWithItemsFn: func(_ context.Context, _ *dalmodel.Order, _ []*dalmodel.OrderItem) (*dalmodel.Order, error) {
			return nil, errors.New("db failed")
		},
	}, &stubProductClient{
		batchGetSKUFn: func(_ context.Context, _ *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			return &productrpc.BatchGetSKUResponse{Code: 0, Skus: []*productpb.ProductSku{{Id: 3001, SpuId: 2001, Title: "Blue XL", SalePrice: 1999, Status: 1}}}, nil
		},
		getProductDetailFn: func(_ context.Context, _ *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{Code: 0, Product: &productpb.Product{Id: 2001, Title: "MeshCart Tee", Status: 2}}, nil
		},
	}, &stubInventoryClient{
		reserveFn: func(_ context.Context, _ *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
			return &inventoryrpc.ReserveSkuStocksResponse{Code: 0}, nil
		},
		releaseFn: func(_ context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
			released = req.GetBizType() == "order" && len(req.GetItems()) == 1 && req.GetItems()[0].GetSkuId() == 3001
			return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: 0}, nil
		},
	})

	order, bizErr := svc.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: 101,
		Items:  []*orderpb.OrderItemInput{{ProductId: 2001, SkuId: 3001, Quantity: 1}},
	})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr != common.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %+v", bizErr)
	}
	if !released {
		t.Fatalf("expected reserved stock to be released on create failure")
	}
}

func TestOrderService_CancelOrder_Success(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		getByOrderIDFn: func(_ context.Context, userID, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{
				OrderID: orderID,
				UserID:  userID,
				Status:  OrderStatusReserved,
				Items: []dalmodel.OrderItem{
					{SKUID: 3001, Quantity: 2},
				},
			}, nil
		},
		updateStatusFn: func(_ context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error) {
			if orderID != 1 || toStatus != OrderStatusCancelled || cancelReason != "user_cancelled" {
				t.Fatalf("unexpected update status args: orderID=%d to=%d reason=%q", orderID, toStatus, cancelReason)
			}
			return &dalmodel.Order{OrderID: 1, UserID: 101, Status: OrderStatusCancelled, CancelReason: cancelReason}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{
		releaseFn: func(_ context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
			if req.GetBizType() != "order" || req.GetBizId() != "1" || len(req.GetItems()) != 1 || req.GetItems()[0].GetQuantity() != 2 {
				t.Fatalf("unexpected release req: %+v", req)
			}
			return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: 0}, nil
		},
	})

	order, bizErr := svc.CancelOrder(context.Background(), &orderpb.CancelOrderRequest{UserId: 101, OrderId: 1})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if order == nil || order.GetStatus() != OrderStatusCancelled {
		t.Fatalf("unexpected cancelled order: %+v", order)
	}
}

func TestOrderService_CancelOrder_AlreadyPaid(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		getByOrderIDFn: func(_ context.Context, userID, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{OrderID: orderID, UserID: userID, Status: OrderStatusPaid}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{})

	order, bizErr := svc.CancelOrder(context.Background(), &orderpb.CancelOrderRequest{UserId: 101, OrderId: 1})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr != errno.ErrOrderPaidImmutable {
		t.Fatalf("expected ErrOrderPaidImmutable, got %+v", bizErr)
	}
}

func TestOrderService_CloseExpiredOrders_Success(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		listExpiredFn: func(_ context.Context, now time.Time, limit int) ([]*dalmodel.Order, error) {
			if limit != 10 {
				t.Fatalf("unexpected limit: %d", limit)
			}
			return []*dalmodel.Order{
				{
					OrderID: 1,
					Status:  OrderStatusReserved,
					Items: []dalmodel.OrderItem{
						{SKUID: 3001, Quantity: 1},
					},
				},
			}, nil
		},
		updateStatusFn: func(_ context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error) {
			if orderID != 1 || toStatus != OrderStatusClosed || cancelReason != "order_expired" {
				t.Fatalf("unexpected update args: %d %d %q", orderID, toStatus, cancelReason)
			}
			return &dalmodel.Order{OrderID: orderID, Status: toStatus, CancelReason: cancelReason}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{
		releaseFn: func(_ context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
			if req.GetBizId() != "1" || len(req.GetItems()) != 1 || req.GetItems()[0].GetQuantity() != 1 {
				t.Fatalf("unexpected release req: %+v", req)
			}
			return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: 0}, nil
		},
	})

	limit := int32(10)
	orderIDs, bizErr := svc.CloseExpiredOrders(context.Background(), &orderpb.CloseExpiredOrdersRequest{Limit: &limit})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if len(orderIDs) != 1 || orderIDs[0] != 1 {
		t.Fatalf("unexpected closed orders: %+v", orderIDs)
	}
}

func TestOrderService_CreateOrder_IdempotentByRequestID(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		getActionRecordFn: func(_ context.Context, actionType, actionKey string) (*dalmodel.OrderActionRecord, error) {
			if actionType != "create" || actionKey != "req-create-1" {
				t.Fatalf("unexpected action lookup: %s %s", actionType, actionKey)
			}
			return &dalmodel.OrderActionRecord{ActionType: actionType, ActionKey: actionKey, OrderID: 10101, Status: "succeeded"}, nil
		},
		getByIDFn: func(_ context.Context, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{OrderID: orderID, UserID: 101, Status: OrderStatusReserved}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{})

	order, bizErr := svc.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: ptrString("req-create-1"),
		Items:     []*orderpb.OrderItemInput{{ProductId: 2001, SkuId: 3001, Quantity: 1}},
	})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if order.GetOrderId() != 10101 {
		t.Fatalf("unexpected idempotent order: %+v", order)
	}
}

func TestOrderService_ConfirmOrderPaid_Success(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		getByIDFn: func(_ context.Context, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{
				OrderID: orderID,
				UserID:  101,
				Status:  OrderStatusReserved,
				Items:   []dalmodel.OrderItem{{SKUID: 3001, Quantity: 2}},
			}, nil
		},
		transitionStatusFn: func(_ context.Context, transition repository.OrderTransition) (*dalmodel.Order, error) {
			if transition.OrderID != 1 || transition.ToStatus != OrderStatusPaid || transition.PaymentID != "pay-1" || transition.PaymentMethod != "mock" || transition.PaymentTradeNo != "trade-1" || transition.ActionType != "pay_confirm" {
				t.Fatalf("unexpected transition: %+v", transition)
			}
			return &dalmodel.Order{OrderID: 1, UserID: 101, Status: OrderStatusPaid, PaymentID: transition.PaymentID, PaymentMethod: transition.PaymentMethod, PaymentTradeNo: transition.PaymentTradeNo, PaidAt: transition.PaidAt}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{
		confirmFn: func(_ context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error) {
			if req.GetBizType() != "order" || req.GetBizId() != "1" || len(req.GetItems()) != 1 || req.GetItems()[0].GetQuantity() != 2 {
				t.Fatalf("unexpected confirm req: %+v", req)
			}
			return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{Code: 0}, nil
		},
	})

	order, bizErr := svc.ConfirmOrderPaid(context.Background(), &orderpb.ConfirmOrderPaidRequest{OrderId: 1, PaymentId: "pay-1", PaymentMethod: ptrString("MOCK"), PaymentTradeNo: ptrString("trade-1")})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if order == nil || order.GetStatus() != OrderStatusPaid || order.GetPaymentId() != "pay-1" || order.GetPaymentMethod() != "mock" || order.GetPaymentTradeNo() != "trade-1" {
		t.Fatalf("unexpected paid order: %+v", order)
	}
}

func TestOrderService_ConfirmOrderPaid_IdempotentByPaymentID(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		getActionRecordFn: func(_ context.Context, actionType, actionKey string) (*dalmodel.OrderActionRecord, error) {
			return &dalmodel.OrderActionRecord{ActionType: actionType, ActionKey: actionKey, OrderID: 1, Status: "succeeded"}, nil
		},
		getByIDFn: func(_ context.Context, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{OrderID: orderID, UserID: 101, Status: OrderStatusPaid, PaymentID: "pay-1"}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{})

	order, bizErr := svc.ConfirmOrderPaid(context.Background(), &orderpb.ConfirmOrderPaidRequest{OrderId: 1, PaymentId: "pay-1"})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if order.GetOrderId() != 1 || order.GetStatus() != OrderStatusPaid {
		t.Fatalf("unexpected idempotent paid order: %+v", order)
	}
}

func TestOrderService_ConfirmOrderPaid_ConflictOnDifferentPaymentID(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		getByIDFn: func(_ context.Context, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{OrderID: orderID, Status: OrderStatusPaid, PaymentID: "pay-old"}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{})

	order, bizErr := svc.ConfirmOrderPaid(context.Background(), &orderpb.ConfirmOrderPaidRequest{OrderId: 1, PaymentId: "pay-new"})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr != errno.ErrOrderPaymentConflict {
		t.Fatalf("expected ErrOrderPaymentConflict, got %+v", bizErr)
	}
}

func TestOrderService_ConfirmOrderPaid_ConflictOnDifferentTradeNo(t *testing.T) {
	svc := newOrderService(t, &stubOrderRepository{
		getByIDFn: func(_ context.Context, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{OrderID: orderID, Status: OrderStatusPaid, PaymentID: "pay-1", PaymentTradeNo: "trade-old"}, nil
		},
	}, &stubProductClient{}, &stubInventoryClient{})

	order, bizErr := svc.ConfirmOrderPaid(context.Background(), &orderpb.ConfirmOrderPaidRequest{OrderId: 1, PaymentId: "pay-1", PaymentTradeNo: ptrString("trade-new")})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr != errno.ErrOrderPaymentConflict {
		t.Fatalf("expected ErrOrderPaymentConflict, got %+v", bizErr)
	}
}

func ptrString(v string) *string {
	return &v
}
