package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dalmodel "meshcart/services/cart-service/dal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWithQueryTimeout_AddsDeadline(t *testing.T) {
	ctx, cancel := withQueryTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}

	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 100*time.Millisecond {
		t.Fatalf("expected deadline close to 50ms, got remaining=%s", remaining)
	}
}

func TestWithQueryTimeout_DisabledWhenTimeoutNonPositive(t *testing.T) {
	parent := context.Background()
	ctx, cancel := withQueryTimeout(parent, 0)
	defer cancel()

	if ctx != parent {
		t.Fatal("expected original context when timeout is disabled")
	}
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("expected no deadline on original context")
	}
}

func TestWithQueryTimeout_Expires(t *testing.T) {
	ctx, cancel := withQueryTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected context deadline to fire")
	}

	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", ctx.Err())
	}
}

func TestRepository_AddOrAccumulate_CreatesNewItem(t *testing.T) {
	db := newCartSQLiteDB(t)
	repo := NewMySQLCartRepository(db, time.Second)

	item, err := repo.AddOrAccumulate(context.Background(), &dalmodel.CartItem{
		ID:                1,
		UserID:            101,
		ProductID:         2001,
		SKUID:             3001,
		Quantity:          2,
		Checked:           true,
		TitleSnapshot:     "MeshCart Tee",
		SKUTitleSnapshot:  "Blue XL",
		SalePriceSnapshot: 1999,
		CoverURLSnapshot:  "https://example.test/cover.png",
	})
	if err != nil {
		t.Fatalf("add or accumulate: %v", err)
	}
	if item == nil || item.ID != 1 || item.Quantity != 2 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestRepository_AddOrAccumulate_AccumulatesExistingItem(t *testing.T) {
	db := newCartSQLiteDB(t)
	seedCartItem(t, db, &dalmodel.CartItem{
		ID:                1,
		UserID:            101,
		ProductID:         2001,
		SKUID:             3001,
		Quantity:          2,
		Checked:           true,
		TitleSnapshot:     "Old Title",
		SKUTitleSnapshot:  "Old SKU",
		SalePriceSnapshot: 1000,
		CoverURLSnapshot:  "old.png",
	})

	repo := NewMySQLCartRepository(db, time.Second)
	item, err := repo.AddOrAccumulate(context.Background(), &dalmodel.CartItem{
		ID:                2,
		UserID:            101,
		ProductID:         2001,
		SKUID:             3001,
		Quantity:          3,
		Checked:           false,
		TitleSnapshot:     "MeshCart Tee",
		SKUTitleSnapshot:  "Blue XL",
		SalePriceSnapshot: 1999,
		CoverURLSnapshot:  "cover.png",
	})
	if err != nil {
		t.Fatalf("add or accumulate: %v", err)
	}
	if item == nil {
		t.Fatal("expected item")
	}
	if item.ID != 1 {
		t.Fatalf("expected existing item id 1, got %d", item.ID)
	}
	if item.Quantity != 5 {
		t.Fatalf("expected accumulated quantity 5, got %d", item.Quantity)
	}
	if item.Checked {
		t.Fatalf("expected checked to be updated to false")
	}
	if item.TitleSnapshot != "MeshCart Tee" || item.SKUTitleSnapshot != "Blue XL" {
		t.Fatalf("expected snapshots to be refreshed, got %+v", item)
	}
}

func TestRepository_UpdateByID_UpdatesQuantityAndChecked(t *testing.T) {
	db := newCartSQLiteDB(t)
	seedCartItem(t, db, &dalmodel.CartItem{
		ID:                1,
		UserID:            101,
		ProductID:         2001,
		SKUID:             3001,
		Quantity:          2,
		Checked:           true,
		TitleSnapshot:     "MeshCart Tee",
		SKUTitleSnapshot:  "Blue XL",
		SalePriceSnapshot: 1999,
		CoverURLSnapshot:  "cover.png",
	})

	repo := NewMySQLCartRepository(db, time.Second)
	checked := false
	item, err := repo.UpdateByID(context.Background(), 101, 1, 7, &checked)
	if err != nil {
		t.Fatalf("update by id: %v", err)
	}
	if item.Quantity != 7 {
		t.Fatalf("expected quantity 7, got %d", item.Quantity)
	}
	if item.Checked {
		t.Fatalf("expected checked false after update")
	}
}

func TestRepository_DeleteAndClear(t *testing.T) {
	db := newCartSQLiteDB(t)
	seedCartItem(t, db, &dalmodel.CartItem{ID: 1, UserID: 101, ProductID: 2001, SKUID: 3001, Quantity: 2})
	seedCartItem(t, db, &dalmodel.CartItem{ID: 2, UserID: 101, ProductID: 2002, SKUID: 3002, Quantity: 1})
	seedCartItem(t, db, &dalmodel.CartItem{ID: 3, UserID: 102, ProductID: 2003, SKUID: 3003, Quantity: 1})

	repo := NewMySQLCartRepository(db, time.Second)
	if err := repo.DeleteByID(context.Background(), 101, 1); err != nil {
		t.Fatalf("delete by id: %v", err)
	}
	items, err := repo.ListByUserID(context.Background(), 101)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 1 || items[0].ID != 2 {
		t.Fatalf("unexpected items after delete: %+v", items)
	}

	if err := repo.ClearByUserID(context.Background(), 101); err != nil {
		t.Fatalf("clear by user id: %v", err)
	}
	items, err = repo.ListByUserID(context.Background(), 101)
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items after clear, got %+v", items)
	}
	otherItems, err := repo.ListByUserID(context.Background(), 102)
	if err != nil {
		t.Fatalf("list other user: %v", err)
	}
	if len(otherItems) != 1 || otherItems[0].ID != 3 {
		t.Fatalf("expected other user item untouched, got %+v", otherItems)
	}
}

func TestRepository_ListByUserID_DBTimeout(t *testing.T) {
	db := newCartSQLiteDB(t)
	registerBlockingQueryCallback(t, db)

	repo := NewMySQLCartRepository(db, 20*time.Millisecond)
	_, err := repo.ListByUserID(context.Background(), 101)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func newCartSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.CartItem{}); err != nil {
		t.Fatalf("migrate sqlite schema: %v", err)
	}
	return db
}

func seedCartItem(t *testing.T, db *gorm.DB, item *dalmodel.CartItem) {
	t.Helper()
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("seed cart item: %v", err)
	}
}

func registerBlockingQueryCallback(t *testing.T, db *gorm.DB) {
	t.Helper()

	const callbackName = "test:block_until_ctx_done"
	if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}
		<-ctx.Done()
		_ = tx.AddError(ctx.Err())
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
}
