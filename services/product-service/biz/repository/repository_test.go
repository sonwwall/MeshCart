package repository

import (
	"context"
	"testing"
	"time"

	dalmodel "meshcart/services/product-service/dal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMySQLProductRepository_UpdateMarksMissingSKUsInactive(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.Product{}, &dalmodel.ProductSKU{}, &dalmodel.ProductSKUAttr{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMySQLProductRepository(db, time.Second)
	ctx := context.Background()

	product := &dalmodel.Product{ID: 2001, Title: "Old", Status: 1}
	skuKeep := &dalmodel.ProductSKU{ID: 3001, SPUID: 2001, Title: "Keep", SalePrice: 100, Status: 1}
	skuDelete := &dalmodel.ProductSKU{ID: 3002, SPUID: 2001, Title: "Delete", SalePrice: 120, Status: 1}
	attr := &dalmodel.ProductSKUAttr{ID: 4001, SKUID: 3002, AttrName: "颜色", AttrValue: "黑色", Sort: 1}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := db.Create(skuKeep).Error; err != nil {
		t.Fatalf("create keep sku: %v", err)
	}
	if err := db.Create(skuDelete).Error; err != nil {
		t.Fatalf("create delete sku: %v", err)
	}
	if err := db.Create(attr).Error; err != nil {
		t.Fatalf("create attr: %v", err)
	}

	err = repo.Update(ctx, &dalmodel.Product{
		ID:          2001,
		Title:       "New",
		SubTitle:    "",
		CategoryID:  1,
		Brand:       "",
		Description: "",
		Status:      1,
		UpdatedBy:   99,
	}, []*dalmodel.ProductSKU{
		{ID: 3001, SPUID: 2001, Title: "Keep Updated", SalePrice: 150, Status: 1},
	})
	if err != nil {
		t.Fatalf("update product: %v", err)
	}

	var stale dalmodel.ProductSKU
	if err := db.Where("id = ?", 3002).Take(&stale).Error; err != nil {
		t.Fatalf("load stale sku: %v", err)
	}
	if stale.Status != 0 {
		t.Fatalf("expected stale sku status 0, got %d", stale.Status)
	}

	var attrs []dalmodel.ProductSKUAttr
	if err := db.Where("sku_id = ?", 3002).Find(&attrs).Error; err != nil {
		t.Fatalf("load stale attrs: %v", err)
	}
	if len(attrs) != 1 {
		t.Fatalf("expected stale attrs retained, got %+v", attrs)
	}
}
