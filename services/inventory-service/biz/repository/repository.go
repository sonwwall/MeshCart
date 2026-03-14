package repository

import (
	"context"
	"errors"
	"time"

	dalmodel "meshcart/services/inventory-service/dal/model"

	"gorm.io/gorm"
)

var (
	ErrStockNotFound = errors.New("inventory stock not found")
)

type InventoryRepository interface {
	GetBySKUID(ctx context.Context, skuID int64) (*dalmodel.InventoryStock, error)
	ListBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error)
}

type MySQLInventoryRepository struct {
	db           *gorm.DB
	queryTimeout time.Duration
}

func NewMySQLInventoryRepository(db *gorm.DB, queryTimeout time.Duration) *MySQLInventoryRepository {
	return &MySQLInventoryRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLInventoryRepository) GetBySKUID(ctx context.Context, skuID int64) (*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var stock dalmodel.InventoryStock
	if err := r.db.WithContext(ctx).Where("sku_id = ?", skuID).Take(&stock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStockNotFound
		}
		return nil, err
	}
	return &stock, nil
}

func (r *MySQLInventoryRepository) ListBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var stocks []*dalmodel.InventoryStock
	if len(skuIDs) == 0 {
		return stocks, nil
	}
	if err := r.db.WithContext(ctx).
		Where("sku_id IN ?", skuIDs).
		Order("sku_id ASC").
		Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func withQueryTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}
