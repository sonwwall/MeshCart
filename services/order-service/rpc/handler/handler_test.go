package handler

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/order-service/biz/repository"
	bizservice "meshcart/services/order-service/biz/service"
	dalmodel "meshcart/services/order-service/dal/model"
	inventoryrpc "meshcart/services/order-service/rpcclient/inventory"
	productrpc "meshcart/services/order-service/rpcclient/product"
)

type stubOrderRepository struct {
	createWithItemsFn    func(context.Context, *dalmodel.Order, []*dalmodel.OrderItem) (*dalmodel.Order, error)
	getByOrderIDFn       func(context.Context, int64, int64) (*dalmodel.Order, error)
	getByIDFn            func(context.Context, int64) (*dalmodel.Order, error)
	listByUserIDFn       func(context.Context, int64, int, int) ([]*dalmodel.Order, int64, error)
	listExpiredOrdersFn  func(context.Context, time.Time, int) ([]*dalmodel.Order, error)
	transitionStatusFn   func(context.Context, repository.OrderTransition) (*dalmodel.Order, error)
	getActionRecordFn    func(context.Context, string, string) (*dalmodel.OrderActionRecord, error)
	createActionRecordFn func(context.Context, *dalmodel.OrderActionRecord) error
	markActionOKFn       func(context.Context, string, string, int64) error
	markActionFailFn     func(context.Context, string, string, string) error
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
	return nil, 0, nil
}
func (s *stubOrderRepository) UpdateStatus(ctx context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error) {
	return s.TransitionStatus(ctx, repository.OrderTransition{OrderID: orderID, FromStatuses: fromStatuses, ToStatus: toStatus, CancelReason: cancelReason})
}
func (s *stubOrderRepository) ListExpiredOrders(ctx context.Context, now time.Time, limit int) ([]*dalmodel.Order, error) {
	if s.listExpiredOrdersFn != nil {
		return s.listExpiredOrdersFn(ctx, now, limit)
	}
	return nil, nil
}
func (s *stubOrderRepository) TransitionStatus(ctx context.Context, transition repository.OrderTransition) (*dalmodel.Order, error) {
	if s.transitionStatusFn != nil {
		return s.transitionStatusFn(ctx, transition)
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
	if s.markActionOKFn != nil {
		return s.markActionOKFn(ctx, actionType, actionKey, orderID)
	}
	return nil
}
func (s *stubOrderRepository) MarkActionRecordFailed(ctx context.Context, actionType, actionKey, errorMessage string) error {
	if s.markActionFailFn != nil {
		return s.markActionFailFn(ctx, actionType, actionKey, errorMessage)
	}
	return nil
}

type stubProductClient struct {
	batchGetFn func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error)
	detailFn   func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error)
}

func (s *stubProductClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
	if s.detailFn != nil {
		return s.detailFn(ctx, req)
	}
	return &productrpc.GetProductDetailResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubProductClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
	if s.batchGetFn != nil {
		return s.batchGetFn(ctx, req)
	}
	return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Message: "成功"}, nil
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
	return &inventoryrpc.ReserveSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubInventoryClient) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
	if s.releaseFn != nil {
		return s.releaseFn(ctx, req)
	}
	return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubInventoryClient) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error) {
	if s.confirmFn != nil {
		return s.confirmFn(ctx, req)
	}
	return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func newHandlerService(t *testing.T, repo repository.OrderRepository, productClient productrpc.Client, inventoryClient inventoryrpc.Client) *bizservice.OrderService {
	t.Helper()
	node, err := snowflake.NewNode(20)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}
	svc := bizservice.NewOrderService(repo, node, productClient, inventoryClient)
	return svc
}

func TestOrderHandler_CreateOrder_Success(t *testing.T) {
	repo := &stubOrderRepository{
		createWithItemsFn: func(_ context.Context, order *dalmodel.Order, items []*dalmodel.OrderItem) (*dalmodel.Order, error) {
			return &dalmodel.Order{OrderID: order.OrderID, UserID: order.UserID, Status: order.Status, Items: []dalmodel.OrderItem{*items[0]}}, nil
		},
	}
	svc := newHandlerService(t, repo, &stubProductClient{
		batchGetFn: func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Skus: []*productpb.ProductSku{{Id: 3001, SpuId: 2001, Title: "Blue XL", SalePrice: 1999, Status: 1}}}, nil
		},
		detailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{Code: common.CodeOK, Product: &productpb.Product{Id: 2001, Title: "MeshCart Tee", Status: 2}}, nil
		},
	}, &stubInventoryClient{
		reserveFn: func(context.Context, *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
			return &inventoryrpc.ReserveSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	})

	h := NewOrderServiceImpl(svc)
	resp, err := h.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: 101,
		Items:  []*orderpb.OrderItemInput{{ProductId: 2001, SkuId: 3001, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.GetBase().GetCode() != 0 || resp.GetOrder() == nil || resp.GetOrder().GetUserId() != 101 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestOrderHandler_CreateOrder_BizError(t *testing.T) {
	svc := newHandlerService(t, &stubOrderRepository{}, &stubProductClient{}, &stubInventoryClient{})
	h := NewOrderServiceImpl(svc)

	resp, err := h.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.GetBase().GetCode() != common.ErrInvalidParam.Code {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestOrderHandler_ConfirmOrderPaid_Success(t *testing.T) {
	repo := &stubOrderRepository{
		getByIDFn: func(_ context.Context, orderID int64) (*dalmodel.Order, error) {
			return &dalmodel.Order{OrderID: orderID, UserID: 101, Status: bizservice.OrderStatusReserved, Items: []dalmodel.OrderItem{{SKUID: 3001, Quantity: 1}}}, nil
		},
		transitionStatusFn: func(_ context.Context, transition repository.OrderTransition) (*dalmodel.Order, error) {
			paidAt := transition.PaidAt
			return &dalmodel.Order{OrderID: transition.OrderID, UserID: 101, Status: bizservice.OrderStatusPaid, PaymentID: transition.PaymentID, PaidAt: paidAt}, nil
		},
	}
	svc := newHandlerService(t, repo, &stubProductClient{}, &stubInventoryClient{
		confirmFn: func(context.Context, *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error) {
			return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	})

	h := NewOrderServiceImpl(svc)
	resp, err := h.ConfirmOrderPaid(context.Background(), &orderpb.ConfirmOrderPaidRequest{OrderId: 1, PaymentId: "pay-1"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.GetBase().GetCode() != 0 || resp.GetOrder() == nil || resp.GetOrder().GetPaymentId() != "pay-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestOrderHandler_ListOrders_Success(t *testing.T) {
	repo := &stubOrderRepository{
		listByUserIDFn: func(_ context.Context, userID int64, _, _ int) ([]*dalmodel.Order, int64, error) {
			return []*dalmodel.Order{{OrderID: 1, UserID: userID, Status: bizservice.OrderStatusReserved}}, 1, nil
		},
	}
	svc := newHandlerService(t, repo, &stubProductClient{}, &stubInventoryClient{})
	h := NewOrderServiceImpl(svc)

	resp, err := h.ListOrders(context.Background(), &orderpb.ListOrdersRequest{UserId: 101, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.GetBase().GetCode() != 0 || len(resp.GetOrders()) != 1 || resp.GetTotal() != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
