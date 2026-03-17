package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dalmodel "meshcart/services/inventory-service/dal/model"

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

func TestRepository_GetBySKUID_DBTimeout(t *testing.T) {
	db := newInventorySQLiteDB(t)
	registerBlockingQueryCallback(t, db)

	repo := NewMySQLInventoryRepository(db, 20*time.Millisecond)
	_, err := repo.GetBySKUID(context.Background(), 3001)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func TestRepository_CreateBatchAndAdjustTotalStock(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	created, err := repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 1, SKUID: 3001, TotalStock: 10, ReservedStock: 0, AvailableStock: 10, Version: 1},
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if len(created) != 1 || created[0].SKUID != 3001 {
		t.Fatalf("unexpected created stocks: %+v", created)
	}

	adjusted, err := repo.AdjustTotalStock(context.Background(), 3001, 8)
	if err != nil {
		t.Fatalf("adjust total stock: %v", err)
	}
	if adjusted.TotalStock != 8 || adjusted.AvailableStock != 8 {
		t.Fatalf("unexpected adjusted stock: %+v", adjusted)
	}
}

func TestRepository_GetBySKUID_NotFound(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	stock, err := repo.GetBySKUID(context.Background(), 3001)
	if stock != nil {
		t.Fatalf("expected nil stock, got %+v", stock)
	}
	if !errors.Is(err, ErrStockNotFound) {
		t.Fatalf("expected ErrStockNotFound, got %v", err)
	}
}

func TestRepository_ListBySKUIDs_Empty(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	stocks, err := repo.ListBySKUIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("list empty sku ids: %v", err)
	}
	if len(stocks) != 0 {
		t.Fatalf("expected empty result, got %+v", stocks)
	}
}

func TestRepository_CreateBatch_DuplicateStock(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	_, err := repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 1, SKUID: 3001, TotalStock: 10, ReservedStock: 0, AvailableStock: 10, Version: 1},
	})
	if err != nil {
		t.Fatalf("seed create batch: %v", err)
	}

	_, err = repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 2, SKUID: 3001, TotalStock: 10, ReservedStock: 0, AvailableStock: 10, Version: 1},
	})
	if !errors.Is(err, ErrStockExists) {
		t.Fatalf("expected ErrStockExists, got %v", err)
	}
}

func TestRepository_CreateBatch_InvalidQuantity(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	_, err := repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 1, SKUID: 3001, TotalStock: -1, ReservedStock: 0, AvailableStock: -1, Version: 1},
	})
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestRepository_AdjustTotalStock_InvalidWhenBelowReserved(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	_, err := repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 1, SKUID: 3001, TotalStock: 10, ReservedStock: 6, AvailableStock: 4, Version: 1},
	})
	if err != nil {
		t.Fatalf("seed create batch: %v", err)
	}

	stock, err := repo.AdjustTotalStock(context.Background(), 3001, 5)
	if stock != nil {
		t.Fatalf("expected nil stock, got %+v", stock)
	}
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestRepository_AdjustTotalStock_NotFound(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	stock, err := repo.AdjustTotalStock(context.Background(), 3001, 5)
	if stock != nil {
		t.Fatalf("expected nil stock, got %+v", stock)
	}
	if !errors.Is(err, ErrStockNotFound) {
		t.Fatalf("expected ErrStockNotFound, got %v", err)
	}
}

func TestRepository_ReserveReleaseConfirmDeduct(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	_, err := repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 1, SKUID: 3001, TotalStock: 10, ReservedStock: 0, AvailableStock: 10, Version: 1, Status: 1},
	})
	if err != nil {
		t.Fatalf("seed create batch: %v", err)
	}

	stocks, err := repo.Reserve(context.Background(), "order", "order-1", []ReservationItem{{SKUID: 3001, Quantity: 2}})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if len(stocks) != 1 || stocks[0].ReservedStock != 2 || stocks[0].AvailableStock != 8 {
		t.Fatalf("unexpected reserved stocks: %+v", stocks)
	}

	stocks, err = repo.ConfirmDeduct(context.Background(), "order", "order-1", []ReservationItem{{SKUID: 3001, Quantity: 2}})
	if err != nil {
		t.Fatalf("confirm deduct: %v", err)
	}
	if len(stocks) != 1 || stocks[0].TotalStock != 8 || stocks[0].ReservedStock != 0 || stocks[0].AvailableStock != 8 {
		t.Fatalf("unexpected confirmed stocks: %+v", stocks)
	}

	stocks, err = repo.ConfirmDeduct(context.Background(), "order", "order-1", []ReservationItem{{SKUID: 3001, Quantity: 2}})
	if err != nil {
		t.Fatalf("confirm deduct idempotent: %v", err)
	}
	if len(stocks) != 1 || stocks[0].TotalStock != 8 {
		t.Fatalf("unexpected idempotent confirmed stocks: %+v", stocks)
	}
}

func TestRepository_ReleaseWithoutReserveCreatesReleasedMarker(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	_, err := repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 1, SKUID: 3001, TotalStock: 10, ReservedStock: 0, AvailableStock: 10, Version: 1, Status: 1},
	})
	if err != nil {
		t.Fatalf("seed create batch: %v", err)
	}

	stocks, err := repo.Release(context.Background(), "order", "order-2", []ReservationItem{{SKUID: 3001, Quantity: 2}})
	if err != nil {
		t.Fatalf("release without reserve: %v", err)
	}
	if len(stocks) != 1 || stocks[0].TotalStock != 10 || stocks[0].ReservedStock != 0 || stocks[0].AvailableStock != 10 {
		t.Fatalf("unexpected released stocks: %+v", stocks)
	}

	_, err = repo.Reserve(context.Background(), "order", "order-2", []ReservationItem{{SKUID: 3001, Quantity: 2}})
	if !errors.Is(err, ErrReservationStateConflict) {
		t.Fatalf("expected ErrReservationStateConflict, got %v", err)
	}
}

func TestRepository_ReserveInsufficientStock(t *testing.T) {
	db := newInventorySQLiteDB(t)
	repo := NewMySQLInventoryRepository(db, time.Second)

	_, err := repo.CreateBatch(context.Background(), []*dalmodel.InventoryStock{
		{ID: 1, SKUID: 3001, TotalStock: 1, ReservedStock: 0, AvailableStock: 1, Version: 1, Status: 1},
	})
	if err != nil {
		t.Fatalf("seed create batch: %v", err)
	}

	_, err = repo.Reserve(context.Background(), "order", "order-3", []ReservationItem{{SKUID: 3001, Quantity: 2}})
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func newInventorySQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.InventoryStock{}, &dalmodel.InventoryReservation{}); err != nil {
		t.Fatalf("migrate sqlite schema: %v", err)
	}
	return db
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
