package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	dalmodel "meshcart/services/product-service/dal/model"

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

func TestRepository_GetByID_DBTimeout(t *testing.T) {
	db := newProductSQLiteDB(t)
	registerBlockingQueryCallback(t, db)

	repo := NewMySQLProductRepository(db, 20*time.Millisecond)
	_, err := repo.GetByID(context.Background(), 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func newProductSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.Product{}, &dalmodel.ProductSKU{}, &dalmodel.ProductSKUAttr{}); err != nil {
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
