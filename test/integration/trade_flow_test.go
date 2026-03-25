package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	productpb "meshcart/kitex_gen/meshcart/product"
	inventoryrepo "meshcart/services/inventory-service/biz/repository"
	inventoryservice "meshcart/services/inventory-service/biz/service"
	inventorymodel "meshcart/services/inventory-service/dal/model"
	ordererrno "meshcart/services/order-service/biz/errno"
	orderrepo "meshcart/services/order-service/biz/repository"
	orderservice "meshcart/services/order-service/biz/service"
	ordermodel "meshcart/services/order-service/dal/model"
	inventoryrpc "meshcart/services/order-service/rpcclient/inventory"
	productrpc "meshcart/services/order-service/rpcclient/product"
	paymenterrno "meshcart/services/payment-service/biz/errno"
	paymentrepo "meshcart/services/payment-service/biz/repository"
	paymentservice "meshcart/services/payment-service/biz/service"
	paymentmodel "meshcart/services/payment-service/dal/model"
	orderrpc "meshcart/services/payment-service/rpcclient/order"
	productrepo "meshcart/services/product-service/biz/repository"
	productservice "meshcart/services/product-service/biz/service"
	productmodel "meshcart/services/product-service/dal/model"
)

func TestTradeFlowIntegration_ServiceChain(t *testing.T) {
	ctx := context.Background()

	env := newTradeFlowEnv(t)
	productSvc := env.productSvc
	inventorySvc := env.inventorySvc
	inventoryRepo := env.inventoryRepo
	orderSvc := env.orderSvc
	paymentSvc := env.paymentSvc

	productID, skus, bizErr := productSvc.CreateProduct(ctx, &productpb.CreateProductRequest{
		Title:       "MeshCart Tee",
		SubTitle:    "Basic",
		CategoryId:  12,
		Brand:       "MeshCart",
		Description: "Basic tee",
		Status:      productservice.ProductStatusOffline,
		CreatorId:   9001,
		Skus: []*productpb.ProductSkuInput{
			{
				SkuCode:     "meshcart-tee-blue-m",
				Title:       "Blue M",
				SalePrice:   1999,
				MarketPrice: 2599,
				Status:      productservice.SKUStatusActive,
				CoverUrl:    "https://example.test/tee-blue-m.png",
			},
		},
	})
	if bizErr != nil {
		t.Fatalf("create product: %+v", bizErr)
	}
	if len(skus) != 1 {
		t.Fatalf("expected 1 sku, got %+v", skus)
	}
	skuID := skus[0].GetId()

	if _, bizErr = inventorySvc.InitSkuStocks(ctx, &inventorypb.InitSkuStocksRequest{
		Stocks: []*inventorypb.InitSkuStockItem{
			{SkuId: skuID, TotalStock: 10},
		},
	}); bizErr != nil {
		t.Fatalf("init sku stocks: %+v", bizErr)
	}

	if bizErr = productSvc.ChangeProductStatus(ctx, productID, productservice.ProductStatusOnline, 9001); bizErr != nil {
		t.Fatalf("change product status online: %+v", bizErr)
	}

	saleable, available, bizErr := inventorySvc.CheckSaleableStock(ctx, &inventorypb.CheckSaleableStockRequest{
		SkuId:    skuID,
		Quantity: 2,
	})
	if bizErr != nil {
		t.Fatalf("check saleable stock: %+v", bizErr)
	}
	if !saleable || available != 10 {
		t.Fatalf("unexpected stock before order saleable=%v available=%d", saleable, available)
	}

	order, bizErr := orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-req-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("create order: %+v", bizErr)
	}
	if order.GetStatus() != orderservice.OrderStatusReserved {
		t.Fatalf("expected reserved order, got %+v", order)
	}

	reservedStock, err := inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load reserved stock: %v", err)
	}
	if reservedStock.TotalStock != 10 || reservedStock.ReservedStock != 2 || reservedStock.AvailableStock != 8 {
		t.Fatalf("unexpected reserved stock snapshot: %+v", reservedStock)
	}

	payment, bizErr := paymentSvc.CreatePayment(ctx, &paymentpb.CreatePaymentRequest{
		OrderId:       order.GetOrderId(),
		UserId:        101,
		PaymentMethod: "mock",
		RequestId:     strPtr("pay-req-1"),
	})
	if bizErr != nil {
		t.Fatalf("create payment: %+v", bizErr)
	}
	if payment.GetStatus() != paymentservice.PaymentStatusPending {
		t.Fatalf("expected pending payment, got %+v", payment)
	}

	payment, bizErr = paymentSvc.ConfirmPaymentSuccess(ctx, &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:      payment.GetPaymentId(),
		PaymentMethod:  "mock",
		RequestId:      strPtr("pay-confirm-1"),
		PaymentTradeNo: strPtr("trade-integration-5001"),
	})
	if bizErr != nil {
		t.Fatalf("confirm payment success: %+v", bizErr)
	}
	if payment.GetStatus() != paymentservice.PaymentStatusSucceeded || payment.GetPaymentTradeNo() != "trade-integration-5001" {
		t.Fatalf("unexpected succeeded payment: %+v", payment)
	}

	paidOrder, bizErr := orderSvc.GetOrder(ctx, &orderpb.GetOrderRequest{
		UserId:  101,
		OrderId: order.GetOrderId(),
	})
	if bizErr != nil {
		t.Fatalf("get paid order: %+v", bizErr)
	}
	if paidOrder.GetStatus() != orderservice.OrderStatusPaid || paidOrder.GetPaymentId() == "" || paidOrder.GetPaymentTradeNo() != "trade-integration-5001" {
		t.Fatalf("unexpected paid order: %+v", paidOrder)
	}

	confirmedStock, err := inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load confirmed stock: %v", err)
	}
	if confirmedStock.TotalStock != 8 || confirmedStock.ReservedStock != 0 || confirmedStock.AvailableStock != 8 {
		t.Fatalf("unexpected confirmed stock snapshot: %+v", confirmedStock)
	}

	loadedPayment, bizErr := paymentSvc.GetPayment(ctx, &paymentpb.GetPaymentRequest{
		PaymentId: payment.GetPaymentId(),
		UserId:    101,
	})
	if bizErr != nil {
		t.Fatalf("get payment: %+v", bizErr)
	}
	if loadedPayment.GetStatus() != paymentservice.PaymentStatusSucceeded || loadedPayment.GetPaymentTradeNo() != "trade-integration-5001" {
		t.Fatalf("unexpected loaded payment: %+v", loadedPayment)
	}
}

func TestTradeFlowIntegration_CreateOrderFailsWhenInsufficientStock(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 1)

	order, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-insufficient-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr == nil || bizErr.Code != ordererrno.CodeOrderInsufficientStock {
		t.Fatalf("expected insufficient stock error, got %+v", bizErr)
	}

	stock, err := env.inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load stock after insufficient order: %v", err)
	}
	if stock.TotalStock != 1 || stock.ReservedStock != 0 || stock.AvailableStock != 1 {
		t.Fatalf("unexpected stock after insufficient order: %+v", stock)
	}
}

func TestTradeFlowIntegration_ConfirmPaymentFailsWhenOrderExpired(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 10)

	order, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-expired-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("create order: %+v", bizErr)
	}

	payment, bizErr := env.paymentSvc.CreatePayment(ctx, &paymentpb.CreatePaymentRequest{
		OrderId:       order.GetOrderId(),
		UserId:        101,
		PaymentMethod: "mock",
		RequestId:     strPtr("pay-expired-1"),
	})
	if bizErr != nil {
		t.Fatalf("create payment: %+v", bizErr)
	}

	expiredAt := time.Now().Add(-time.Minute)
	if err := env.orderDB.Model(&ordermodel.Order{}).
		Where("order_id = ?", order.GetOrderId()).
		Update("expire_at", expiredAt).Error; err != nil {
		t.Fatalf("expire order manually: %v", err)
	}

	confirmed, bizErr := env.paymentSvc.ConfirmPaymentSuccess(ctx, &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:      payment.GetPaymentId(),
		PaymentMethod:  "mock",
		RequestId:      strPtr("pay-expired-confirm-1"),
		PaymentTradeNo: strPtr("trade-expired-5001"),
	})
	if confirmed != nil {
		t.Fatalf("expected nil payment on expired order, got %+v", confirmed)
	}
	if bizErr == nil || bizErr.Code != paymenterrno.CodePaymentOrderStateConflict {
		t.Fatalf("expected payment order state conflict, got %+v", bizErr)
	}

	stillReserved, err := env.inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load stock after expired payment confirm: %v", err)
	}
	if stillReserved.TotalStock != 10 || stillReserved.ReservedStock != 2 || stillReserved.AvailableStock != 8 {
		t.Fatalf("unexpected stock after expired payment confirm: %+v", stillReserved)
	}

	loadedPayment, err := env.paymentRepo.GetByPaymentID(ctx, payment.GetPaymentId())
	if err != nil {
		t.Fatalf("load payment after expired payment confirm: %v", err)
	}
	if loadedPayment.Status != paymentservice.PaymentStatusPending {
		t.Fatalf("expected payment to remain pending, got %+v", loadedPayment)
	}
}

func TestTradeFlowIntegration_ConfirmPaymentConflictOnDifferentTradeNo(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 10)

	order, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-payment-conflict-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("create order: %+v", bizErr)
	}

	payment, bizErr := env.paymentSvc.CreatePayment(ctx, &paymentpb.CreatePaymentRequest{
		OrderId:       order.GetOrderId(),
		UserId:        101,
		PaymentMethod: "mock",
		RequestId:     strPtr("pay-payment-conflict-1"),
	})
	if bizErr != nil {
		t.Fatalf("create payment: %+v", bizErr)
	}

	confirmed, bizErr := env.paymentSvc.ConfirmPaymentSuccess(ctx, &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:      payment.GetPaymentId(),
		PaymentMethod:  "mock",
		RequestId:      strPtr("pay-payment-conflict-confirm-1"),
		PaymentTradeNo: strPtr("trade-ok-5001"),
	})
	if bizErr != nil {
		t.Fatalf("first confirm payment success: %+v", bizErr)
	}
	if confirmed.GetStatus() != paymentservice.PaymentStatusSucceeded {
		t.Fatalf("expected succeeded payment, got %+v", confirmed)
	}

	conflicted, bizErr := env.paymentSvc.ConfirmPaymentSuccess(ctx, &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:      payment.GetPaymentId(),
		PaymentMethod:  "mock",
		RequestId:      strPtr("pay-payment-conflict-confirm-2"),
		PaymentTradeNo: strPtr("trade-conflict-5002"),
	})
	if conflicted != nil {
		t.Fatalf("expected nil payment on conflict, got %+v", conflicted)
	}
	if bizErr == nil || bizErr.Code != paymenterrno.CodePaymentConflict {
		t.Fatalf("expected payment conflict, got %+v", bizErr)
	}

	paidOrder, bizErr := env.orderSvc.GetOrder(ctx, &orderpb.GetOrderRequest{
		UserId:  101,
		OrderId: order.GetOrderId(),
	})
	if bizErr != nil {
		t.Fatalf("get order after conflict: %+v", bizErr)
	}
	if paidOrder.GetStatus() != orderservice.OrderStatusPaid || paidOrder.GetPaymentTradeNo() != "trade-ok-5001" {
		t.Fatalf("unexpected paid order after conflict: %+v", paidOrder)
	}

	stock, err := env.inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load stock after conflict: %v", err)
	}
	if stock.TotalStock != 8 || stock.ReservedStock != 0 || stock.AvailableStock != 8 {
		t.Fatalf("unexpected stock after conflict: %+v", stock)
	}
}

func TestTradeFlowIntegration_ConfirmPaymentClosesExpiredPayment(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 10)

	order, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-payment-expired-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("create order: %+v", bizErr)
	}

	payment, bizErr := env.paymentSvc.CreatePayment(ctx, &paymentpb.CreatePaymentRequest{
		OrderId:       order.GetOrderId(),
		UserId:        101,
		PaymentMethod: "mock",
		RequestId:     strPtr("pay-payment-expired-1"),
	})
	if bizErr != nil {
		t.Fatalf("create payment: %+v", bizErr)
	}

	expiredAt := time.Now().Add(-time.Minute)
	if err := env.paymentDB.Model(&paymentmodel.Payment{}).
		Where("payment_id = ?", payment.GetPaymentId()).
		Update("expire_at", expiredAt).Error; err != nil {
		t.Fatalf("expire payment manually: %v", err)
	}

	confirmed, bizErr := env.paymentSvc.ConfirmPaymentSuccess(ctx, &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:      payment.GetPaymentId(),
		PaymentMethod:  "mock",
		RequestId:      strPtr("pay-payment-expired-confirm-1"),
		PaymentTradeNo: strPtr("trade-expired-payment-1"),
	})
	if confirmed != nil {
		t.Fatalf("expected nil payment on expired payment, got %+v", confirmed)
	}
	if bizErr == nil || bizErr.Code != paymenterrno.CodePaymentExpired {
		t.Fatalf("expected payment expired error, got %+v", bizErr)
	}

	closedPayment, err := env.paymentRepo.GetByPaymentID(ctx, payment.GetPaymentId())
	if err != nil {
		t.Fatalf("load payment after expiry: %v", err)
	}
	if closedPayment.Status != paymentservice.PaymentStatusClosed || closedPayment.FailReason != "payment_expired" {
		t.Fatalf("expected payment to be closed by expiry, got %+v", closedPayment)
	}
}

func TestTradeFlowIntegration_CreateOrderReleasesReservedStockOnPersistFailure(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 10)

	failingOrderSvc := orderservice.NewOrderService(
		&failingCreateOrderRepo{},
		newSnowflakeNode(t, 52),
		&productServiceAdapter{svc: env.productSvc},
		&inventoryServiceAdapter{svc: env.inventorySvc},
	)

	order, bizErr := failingOrderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-create-fail-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if bizErr == nil || bizErr.Code != common.ErrInternalError.Code {
		t.Fatalf("expected internal error, got %+v", bizErr)
	}

	stock, err := env.inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load stock after create failure: %v", err)
	}
	if stock.TotalStock != 10 || stock.ReservedStock != 0 || stock.AvailableStock != 10 {
		t.Fatalf("expected reserved stock to be released, got %+v", stock)
	}
}

func TestTradeFlowIntegration_CancelOrderReleasesReservedStock(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 10)

	order, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-cancel-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("create order: %+v", bizErr)
	}

	cancelled, bizErr := env.orderSvc.CancelOrder(ctx, &orderpb.CancelOrderRequest{
		UserId:       101,
		OrderId:      order.GetOrderId(),
		RequestId:    strPtr("cancel-order-1"),
		CancelReason: strPtr("user_cancelled"),
	})
	if bizErr != nil {
		t.Fatalf("cancel order: %+v", bizErr)
	}
	if cancelled.GetStatus() != orderservice.OrderStatusCancelled || cancelled.GetCancelReason() != "user_cancelled" {
		t.Fatalf("unexpected cancelled order: %+v", cancelled)
	}

	stock, err := env.inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load stock after cancel: %v", err)
	}
	if stock.TotalStock != 10 || stock.ReservedStock != 0 || stock.AvailableStock != 10 {
		t.Fatalf("expected reserved stock to be released on cancel, got %+v", stock)
	}
}

func TestTradeFlowIntegration_CreateOrderIdempotentByRequestID(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 10)

	first, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-idempotent-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("first create order: %+v", bizErr)
	}

	second, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-idempotent-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("second create order: %+v", bizErr)
	}
	if first.GetOrderId() != second.GetOrderId() || second.GetStatus() != orderservice.OrderStatusReserved {
		t.Fatalf("expected idempotent order reuse, first=%+v second=%+v", first, second)
	}

	stock, err := env.inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load stock after idempotent create: %v", err)
	}
	if stock.TotalStock != 10 || stock.ReservedStock != 2 || stock.AvailableStock != 8 {
		t.Fatalf("expected single reserve after idempotent create, got %+v", stock)
	}
}

func TestTradeFlowIntegration_ConfirmPaymentIdempotentByRequestID(t *testing.T) {
	ctx := context.Background()
	env := newTradeFlowEnv(t)

	productID, skuID := seedOnlineProductWithStock(t, ctx, env, 10)

	order, bizErr := env.orderSvc.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:    101,
		RequestId: strPtr("order-pay-idempotent-1"),
		Items: []*orderpb.OrderItemInput{
			{ProductId: productID, SkuId: skuID, Quantity: 2},
		},
	})
	if bizErr != nil {
		t.Fatalf("create order: %+v", bizErr)
	}
	payment, bizErr := env.paymentSvc.CreatePayment(ctx, &paymentpb.CreatePaymentRequest{
		OrderId:       order.GetOrderId(),
		UserId:        101,
		PaymentMethod: "mock",
		RequestId:     strPtr("pay-idempotent-1"),
	})
	if bizErr != nil {
		t.Fatalf("create payment: %+v", bizErr)
	}

	first, bizErr := env.paymentSvc.ConfirmPaymentSuccess(ctx, &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:      payment.GetPaymentId(),
		PaymentMethod:  "mock",
		RequestId:      strPtr("pay-confirm-idempotent-1"),
		PaymentTradeNo: strPtr("trade-idempotent-1"),
	})
	if bizErr != nil {
		t.Fatalf("first confirm payment: %+v", bizErr)
	}
	second, bizErr := env.paymentSvc.ConfirmPaymentSuccess(ctx, &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:      payment.GetPaymentId(),
		PaymentMethod:  "mock",
		RequestId:      strPtr("pay-confirm-idempotent-1"),
		PaymentTradeNo: strPtr("trade-idempotent-1"),
	})
	if bizErr != nil {
		t.Fatalf("second confirm payment: %+v", bizErr)
	}
	if first.GetPaymentId() != second.GetPaymentId() || second.GetStatus() != paymentservice.PaymentStatusSucceeded {
		t.Fatalf("expected idempotent confirm reuse, first=%+v second=%+v", first, second)
	}

	stock, err := env.inventoryRepo.GetBySKUID(ctx, skuID)
	if err != nil {
		t.Fatalf("load stock after idempotent confirm: %v", err)
	}
	if stock.TotalStock != 8 || stock.ReservedStock != 0 || stock.AvailableStock != 8 {
		t.Fatalf("expected single confirm deduct after idempotent confirm, got %+v", stock)
	}
}

type tradeFlowEnv struct {
	productSvc    *productservice.ProductService
	inventorySvc  *inventoryservice.InventoryService
	orderSvc      *orderservice.OrderService
	paymentSvc    *paymentservice.PaymentService
	inventoryRepo *inventoryrepo.MySQLInventoryRepository
	paymentRepo   *paymentrepo.MySQLPaymentRepository
	orderDB       *gorm.DB
	paymentDB     *gorm.DB
}

type failingCreateOrderRepo struct{}

func (r *failingCreateOrderRepo) CreateWithItems(context.Context, *ordermodel.Order, []*ordermodel.OrderItem) (*ordermodel.Order, error) {
	return nil, fmt.Errorf("simulated create failure")
}

func (r *failingCreateOrderRepo) GetByOrderID(context.Context, int64, int64) (*ordermodel.Order, error) {
	return nil, orderrepo.ErrOrderNotFound
}

func (r *failingCreateOrderRepo) GetByID(context.Context, int64) (*ordermodel.Order, error) {
	return nil, orderrepo.ErrOrderNotFound
}

func (r *failingCreateOrderRepo) ListByUserID(context.Context, int64, int, int) ([]*ordermodel.Order, int64, error) {
	return []*ordermodel.Order{}, 0, nil
}

func (r *failingCreateOrderRepo) UpdateStatus(context.Context, int64, []int32, int32, string) (*ordermodel.Order, error) {
	return nil, orderrepo.ErrOrderStateConflict
}

func (r *failingCreateOrderRepo) ListExpiredOrders(context.Context, time.Time, int) ([]*ordermodel.Order, error) {
	return []*ordermodel.Order{}, nil
}

func (r *failingCreateOrderRepo) TransitionStatus(context.Context, orderrepo.OrderTransition) (*ordermodel.Order, error) {
	return nil, orderrepo.ErrOrderStateConflict
}

func (r *failingCreateOrderRepo) GetActionRecord(context.Context, string, string) (*ordermodel.OrderActionRecord, error) {
	return nil, orderrepo.ErrActionRecordNotFound
}

func (r *failingCreateOrderRepo) CreateActionRecord(context.Context, *ordermodel.OrderActionRecord) error {
	return nil
}

func (r *failingCreateOrderRepo) MarkActionRecordSucceeded(context.Context, string, string, int64) error {
	return nil
}

func (r *failingCreateOrderRepo) MarkActionRecordFailed(context.Context, string, string, string) error {
	return nil
}

func (r *failingCreateOrderRepo) MarkActionRecordSucceededByID(context.Context, int64, int64) error {
	return nil
}

func (r *failingCreateOrderRepo) MarkActionRecordFailedByID(context.Context, int64, string) error {
	return nil
}

func newTradeFlowEnv(t *testing.T) *tradeFlowEnv {
	t.Helper()

	productDB := newSQLiteDB(t, "product-integration")
	if err := productDB.AutoMigrate(&productmodel.Product{}, &productmodel.ProductSKU{}, &productmodel.ProductSKUAttr{}, &productmodel.ProductTxBranch{}); err != nil {
		t.Fatalf("migrate product schema: %v", err)
	}
	inventoryDB := newSQLiteDB(t, "inventory-integration")
	if err := inventoryDB.AutoMigrate(&inventorymodel.InventoryStock{}, &inventorymodel.InventoryReservation{}, &inventorymodel.InventoryTxBranch{}); err != nil {
		t.Fatalf("migrate inventory schema: %v", err)
	}
	orderDB := newSQLiteDB(t, "order-integration")
	if err := orderDB.AutoMigrate(&ordermodel.Order{}, &ordermodel.OrderItem{}, &ordermodel.OrderActionRecord{}, &ordermodel.OrderStatusLog{}); err != nil {
		t.Fatalf("migrate order schema: %v", err)
	}
	paymentDB := newSQLiteDB(t, "payment-integration")
	if err := paymentDB.AutoMigrate(&paymentmodel.Payment{}, &paymentmodel.PaymentActionRecord{}, &paymentmodel.PaymentStatusLog{}); err != nil {
		t.Fatalf("migrate payment schema: %v", err)
	}

	productSvc := productservice.NewProductService(productrepo.NewMySQLProductRepository(productDB, time.Second), newSnowflakeNode(t, 41), nil)
	inventoryRepo := inventoryrepo.NewMySQLInventoryRepository(inventoryDB, time.Second, func() int64 {
		return time.Now().UnixNano()
	})
	inventorySvc := inventoryservice.NewInventoryService(inventoryRepo)
	orderSvc := orderservice.NewOrderService(
		orderrepo.NewMySQLOrderRepository(orderDB, time.Second),
		newSnowflakeNode(t, 42),
		&productServiceAdapter{svc: productSvc},
		&inventoryServiceAdapter{svc: inventorySvc},
	)
	paymentRepo := paymentrepo.NewMySQLPaymentRepository(paymentDB, time.Second)
	paymentSvc := paymentservice.NewPaymentService(
		paymentRepo,
		newSnowflakeNode(t, 43),
		&orderServiceAdapter{svc: orderSvc},
	)

	return &tradeFlowEnv{
		productSvc:    productSvc,
		inventorySvc:  inventorySvc,
		orderSvc:      orderSvc,
		paymentSvc:    paymentSvc,
		inventoryRepo: inventoryRepo,
		paymentRepo:   paymentRepo,
		orderDB:       orderDB,
		paymentDB:     paymentDB,
	}
}

func seedOnlineProductWithStock(t *testing.T, ctx context.Context, env *tradeFlowEnv, totalStock int64) (int64, int64) {
	t.Helper()

	productID, skus, bizErr := env.productSvc.CreateProduct(ctx, &productpb.CreateProductRequest{
		Title:       "MeshCart Tee",
		SubTitle:    "Basic",
		CategoryId:  12,
		Brand:       "MeshCart",
		Description: "Basic tee",
		Status:      productservice.ProductStatusOffline,
		CreatorId:   9001,
		Skus: []*productpb.ProductSkuInput{
			{
				SkuCode:     fmt.Sprintf("meshcart-tee-%d", time.Now().UnixNano()),
				Title:       "Blue M",
				SalePrice:   1999,
				MarketPrice: 2599,
				Status:      productservice.SKUStatusActive,
				CoverUrl:    "https://example.test/tee-blue-m.png",
			},
		},
	})
	if bizErr != nil {
		t.Fatalf("create seeded product: %+v", bizErr)
	}
	skuID := skus[0].GetId()
	if _, bizErr = env.inventorySvc.InitSkuStocks(ctx, &inventorypb.InitSkuStocksRequest{
		Stocks: []*inventorypb.InitSkuStockItem{{SkuId: skuID, TotalStock: totalStock}},
	}); bizErr != nil {
		t.Fatalf("init seeded stock: %+v", bizErr)
	}
	if bizErr = env.productSvc.ChangeProductStatus(ctx, productID, productservice.ProductStatusOnline, 9001); bizErr != nil {
		t.Fatalf("online seeded product: %+v", bizErr)
	}
	return productID, skuID
}

type productServiceAdapter struct {
	svc *productservice.ProductService
}

func (a *productServiceAdapter) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
	product, bizErr := a.svc.GetProductDetail(ctx, req.GetProductId())
	if bizErr != nil {
		return &productrpc.GetProductDetailResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &productrpc.GetProductDetailResponse{Code: 0, Message: "成功", Product: product}, nil
}

func (a *productServiceAdapter) BatchGetProducts(ctx context.Context, req *productpb.BatchGetProductsRequest) (*productrpc.BatchGetProductsResponse, error) {
	products, bizErr := a.svc.BatchGetProducts(ctx, req.GetProductIds())
	if bizErr != nil {
		return &productrpc.BatchGetProductsResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &productrpc.BatchGetProductsResponse{Code: 0, Message: "成功", Products: products}, nil
}

func (a *productServiceAdapter) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
	skus, bizErr := a.svc.BatchGetSKU(ctx, req.GetSkuIds())
	if bizErr != nil {
		return &productrpc.BatchGetSKUResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &productrpc.BatchGetSKUResponse{Code: 0, Message: "成功", Skus: skus}, nil
}

type inventoryServiceAdapter struct {
	svc *inventoryservice.InventoryService
}

func (a *inventoryServiceAdapter) ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
	stocks, bizErr := a.svc.ReserveSkuStocks(ctx, req)
	if bizErr != nil {
		return &inventoryrpc.ReserveSkuStocksResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &inventoryrpc.ReserveSkuStocksResponse{Code: 0, Message: "成功", Stocks: stocks}, nil
}

func (a *inventoryServiceAdapter) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
	stocks, bizErr := a.svc.ReleaseReservedSkuStocks(ctx, req)
	if bizErr != nil {
		return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: 0, Message: "成功", Stocks: stocks}, nil
}

func (a *inventoryServiceAdapter) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error) {
	stocks, bizErr := a.svc.ConfirmDeductReservedSkuStocks(ctx, req)
	if bizErr != nil {
		return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{Code: 0, Message: "成功", Stocks: stocks}, nil
}

type orderServiceAdapter struct {
	svc *orderservice.OrderService
}

func (a *orderServiceAdapter) GetOrder(ctx context.Context, userID, orderID int64) (*orderrpc.GetOrderResponse, error) {
	order, bizErr := a.svc.GetOrder(ctx, &orderpb.GetOrderRequest{UserId: userID, OrderId: orderID})
	if bizErr != nil {
		return &orderrpc.GetOrderResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &orderrpc.GetOrderResponse{Code: 0, Message: "成功", Order: order}, nil
}

func (a *orderServiceAdapter) ConfirmOrderPaid(ctx context.Context, req *orderpb.ConfirmOrderPaidRequest) (*orderrpc.ConfirmOrderPaidResponse, error) {
	order, bizErr := a.svc.ConfirmOrderPaid(ctx, req)
	if bizErr != nil {
		return &orderrpc.ConfirmOrderPaidResponse{Code: bizErr.Code, Message: bizErr.Msg}, nil
	}
	return &orderrpc.ConfirmOrderPaidResponse{Code: 0, Message: "成功", Order: order}, nil
}

func newSQLiteDB(t *testing.T, name string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=private", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func newSnowflakeNode(t *testing.T, nodeID int64) *snowflake.Node {
	t.Helper()

	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		t.Fatalf("new snowflake node: %v", err)
	}
	return node
}

func strPtr(v string) *string {
	return &v
}
