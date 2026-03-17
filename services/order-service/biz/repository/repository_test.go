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
		OrderID:     1,
		UserID:      101,
		Status:      1,
		TotalAmount: 3998,
		PayAmount:   3998,
		ExpireAt:    time.Now().Add(30 * time.Minute),
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

func newOrderSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.Order{}, &dalmodel.OrderItem{}); err != nil {
		t.Fatalf("migrate sqlite schema: %v", err)
	}
	return db
}
