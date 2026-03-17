package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dalmodel "meshcart/services/order-service/dal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepository_CreateGetList(t *testing.T) {
	db := newOrderSQLiteDB(t)
	repo := NewMySQLOrderRepository(db, time.Second)

	order, err := repo.CreateWithItems(context.Background(), &dalmodel.Order{
		OrderID:      1,
		UserID:       101,
		Status:       1,
		TotalAmount:  3998,
		PayAmount:    3998,
		ExpireAt:     time.Now().Add(30 * time.Minute),
		CancelReason: "",
	}, []*dalmodel.OrderItem{
		{
			ID:                   11,
			OrderID:              1,
			ProductID:            2001,
			SKUID:                3001,
			ProductTitleSnapshot: "MeshCart Tee",
			SKUTitleSnapshot:     "Blue XL",
			SalePriceSnapshot:    1999,
			Quantity:             2,
			SubtotalAmount:       3998,
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.OrderID != 1 || len(order.Items) != 1 {
		t.Fatalf("unexpected order: %+v", order)
	}

	got, err := repo.GetByOrderID(context.Background(), 101, 1)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if got.OrderID != 1 || len(got.Items) != 1 || got.Items[0].SKUID != 3001 {
		t.Fatalf("unexpected get result: %+v", got)
	}

	orders, total, err := repo.ListByUserID(context.Background(), 101, 0, 20)
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if total != 1 || len(orders) != 1 {
		t.Fatalf("unexpected list result total=%d orders=%+v", total, orders)
	}
}

func TestRepository_GetByOrderID_NotFound(t *testing.T) {
	db := newOrderSQLiteDB(t)
	repo := NewMySQLOrderRepository(db, time.Second)

	order, err := repo.GetByOrderID(context.Background(), 101, 1)
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestRepository_CreateWithItems_InvalidOrder(t *testing.T) {
	db := newOrderSQLiteDB(t)
	repo := NewMySQLOrderRepository(db, time.Second)

	order, err := repo.CreateWithItems(context.Background(), nil, nil)
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("expected ErrInvalidOrder, got %v", err)
	}
}

func TestRepository_UpdateStatus(t *testing.T) {
	db := newOrderSQLiteDB(t)
	repo := NewMySQLOrderRepository(db, time.Second)
	seedOrder(t, db, &dalmodel.Order{
		OrderID:      1,
		UserID:       101,
		Status:       2,
		TotalAmount:  100,
		PayAmount:    100,
		ExpireAt:     time.Now().Add(30 * time.Minute),
		CancelReason: "",
	})

	order, err := repo.UpdateStatus(context.Background(), 1, []int32{1, 2}, 4, "user_cancelled")
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if order.Status != 4 || order.CancelReason != "user_cancelled" {
		t.Fatalf("unexpected updated order: %+v", order)
	}
}

func TestRepository_TransitionStatus_WithPaymentAndLog(t *testing.T) {
	db := newOrderSQLiteDB(t)
	repo := NewMySQLOrderRepository(db, time.Second)
	seedOrder(t, db, &dalmodel.Order{
		OrderID:     1,
		UserID:      101,
		Status:      2,
		TotalAmount: 100,
		PayAmount:   100,
		ExpireAt:    time.Now().Add(30 * time.Minute),
	})
	paidAt := time.Now()

	order, err := repo.TransitionStatus(context.Background(), OrderTransition{
		OrderID:      1,
		FromStatuses: []int32{2},
		ToStatus:     3,
		PaymentID:    "pay-1",
		PaidAt:       &paidAt,
		ActionType:   "pay_confirm",
		Reason:       "payment_confirmed",
		ExternalRef:  "pay-1",
	})
	if err != nil {
		t.Fatalf("transition status: %v", err)
	}
	if order.Status != 3 || order.PaymentID != "pay-1" || order.PaidAt == nil {
		t.Fatalf("unexpected transitioned order: %+v", order)
	}

	var count int64
	if err := db.Model(&dalmodel.OrderStatusLog{}).Where("order_id = ? AND action_type = ?", 1, "pay_confirm").Count(&count).Error; err != nil {
		t.Fatalf("count status logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 pay_confirm log, got %d", count)
	}
}

func TestRepository_ListExpiredOrders(t *testing.T) {
	db := newOrderSQLiteDB(t)
	repo := NewMySQLOrderRepository(db, time.Second)
	now := time.Now()
	seedOrder(t, db, &dalmodel.Order{OrderID: 1, UserID: 101, Status: 2, TotalAmount: 100, PayAmount: 100, ExpireAt: now.Add(-time.Minute)})
	seedOrder(t, db, &dalmodel.Order{OrderID: 2, UserID: 101, Status: 4, TotalAmount: 100, PayAmount: 100, ExpireAt: now.Add(-time.Minute)})
	seedOrder(t, db, &dalmodel.Order{OrderID: 3, UserID: 101, Status: 2, TotalAmount: 100, PayAmount: 100, ExpireAt: now.Add(time.Minute)})

	orders, err := repo.ListExpiredOrders(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("list expired: %v", err)
	}
	if len(orders) != 1 || orders[0].OrderID != 1 {
		t.Fatalf("unexpected expired orders: %+v", orders)
	}
}

func TestRepository_ActionRecordLifecycle(t *testing.T) {
	db := newOrderSQLiteDB(t)
	repo := NewMySQLOrderRepository(db, time.Second)
	record := &dalmodel.OrderActionRecord{
		ID:         1,
		ActionType: "create",
		ActionKey:  "req-1",
		OrderID:    0,
		UserID:     101,
		Status:     "pending",
	}

	if err := repo.CreateActionRecord(context.Background(), record); err != nil {
		t.Fatalf("create action record: %v", err)
	}
	if err := repo.MarkActionRecordSucceeded(context.Background(), "create", "req-1", 1001); err != nil {
		t.Fatalf("mark action succeeded: %v", err)
	}
	got, err := repo.GetActionRecord(context.Background(), "create", "req-1")
	if err != nil {
		t.Fatalf("get action record: %v", err)
	}
	if got.OrderID != 1001 || got.Status != "succeeded" {
		t.Fatalf("unexpected action record: %+v", got)
	}
}

func newOrderSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.Order{}, &dalmodel.OrderItem{}, &dalmodel.OrderActionRecord{}, &dalmodel.OrderStatusLog{}); err != nil {
		t.Fatalf("migrate sqlite schema: %v", err)
	}
	return db
}

func seedOrder(t *testing.T, db *gorm.DB, order *dalmodel.Order) {
	t.Helper()

	if err := db.Create(order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
}
