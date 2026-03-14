package service

import (
	"testing"

	dalmodel "meshcart/services/product-service/dal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProductTestDB(t *testing.T) *gorm.DB {
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
