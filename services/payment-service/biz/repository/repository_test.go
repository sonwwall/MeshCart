package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	dalmodel "meshcart/services/payment-service/dal/model"
)

func TestMySQLPaymentRepository_ActiveOrderUniqueGuard(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:payment-repository-active-order?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.Payment{}, &dalmodel.PaymentStatusLog{}); err != nil {
		t.Fatalf("migrate payment schema: %v", err)
	}

	repo := NewMySQLPaymentRepository(db, time.Second)
	ctx := context.Background()
	activeOrderID := int64(10)

	first, err := repo.Create(ctx, &dalmodel.Payment{
		PaymentID:     1001,
		OrderID:       10,
		ActiveOrderID: &activeOrderID,
		UserID:        101,
		Status:        1,
		PaymentMethod: "mock",
		Amount:        1999,
		Currency:      "CNY",
		ExpireAt:      time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create first payment: %v", err)
	}
	if first.ActiveOrderID == nil || *first.ActiveOrderID != 10 {
		t.Fatalf("unexpected first payment: %+v", first)
	}

	_, err = repo.Create(ctx, &dalmodel.Payment{
		PaymentID:     1002,
		OrderID:       10,
		ActiveOrderID: &activeOrderID,
		UserID:        101,
		Status:        1,
		PaymentMethod: "mock",
		Amount:        1999,
		Currency:      "CNY",
		ExpireAt:      time.Now().Add(15 * time.Minute),
	})
	if err != ErrActivePaymentExists {
		t.Fatalf("expected ErrActivePaymentExists, got %v", err)
	}

	closedAt := time.Now()
	closed, err := repo.TransitionStatus(ctx, PaymentTransition{
		PaymentID:    1001,
		FromStatuses: []int32{1},
		ToStatus:     4,
		ClosedAt:     &closedAt,
		ActionType:   "close",
		Reason:       "test_close",
	})
	if err != nil {
		t.Fatalf("close first payment: %v", err)
	}
	if closed.ActiveOrderID != nil {
		t.Fatalf("expected active slot released, got %+v", closed)
	}

	second, err := repo.Create(ctx, &dalmodel.Payment{
		PaymentID:     1003,
		OrderID:       10,
		ActiveOrderID: &activeOrderID,
		UserID:        101,
		Status:        1,
		PaymentMethod: "mock",
		Amount:        1999,
		Currency:      "CNY",
		ExpireAt:      time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create second active payment after close: %v", err)
	}
	if second.ActiveOrderID == nil || *second.ActiveOrderID != 10 {
		t.Fatalf("unexpected second payment: %+v", second)
	}
}
