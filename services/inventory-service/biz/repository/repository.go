package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	dalmodel "meshcart/services/inventory-service/dal/model"

	"gorm.io/gorm"
)

var (
	ErrStockNotFound   = errors.New("inventory stock not found")
	ErrStockExists     = errors.New("inventory stock already exists")
	ErrInvalidQuantity = errors.New("invalid stock quantity")
)

type InventoryRepository interface {
	GetBySKUID(ctx context.Context, skuID int64) (*dalmodel.InventoryStock, error)
	ListBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error)
	CreateBatch(ctx context.Context, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error)
	FreezeBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error)
	AdjustTotalStock(ctx context.Context, skuID int64, totalStock int64) (*dalmodel.InventoryStock, error)
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

func (r *MySQLInventoryRepository) CreateBatch(ctx context.Context, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if len(stocks) == 0 {
		return []*dalmodel.InventoryStock{}, nil
	}
	for _, stock := range stocks {
		if stock == nil || stock.SKUID <= 0 || stock.TotalStock < 0 || stock.ReservedStock < 0 || stock.AvailableStock < 0 {
			return nil, ErrInvalidQuantity
		}
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, stock := range stocks {
			if err := tx.Create(stock).Error; err != nil {
				lowerErr := strings.ToLower(err.Error())
				if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique constraint") {
					return ErrStockExists
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *MySQLInventoryRepository) AdjustTotalStock(ctx context.Context, skuID int64, totalStock int64) (*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if skuID <= 0 || totalStock < 0 {
		return nil, ErrInvalidQuantity
	}

	var stock dalmodel.InventoryStock
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sku_id = ?", skuID).Take(&stock).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStockNotFound
			}
			return err
		}
		if totalStock < stock.ReservedStock {
			return ErrInvalidQuantity
		}
		available := totalStock - stock.ReservedStock
		if err := tx.Model(&dalmodel.InventoryStock{}).
			Where("id = ?", stock.ID).
			Updates(map[string]any{
				"total_stock":     totalStock,
				"available_stock": available,
				"version":         stock.Version + 1,
			}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", stock.ID).Take(&stock).Error
	})
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *MySQLInventoryRepository) FreezeBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if len(skuIDs) == 0 {
		return []*dalmodel.InventoryStock{}, nil
	}
	for _, skuID := range skuIDs {
		if skuID <= 0 {
			return nil, ErrInvalidQuantity
		}
	}

	var stocks []*dalmodel.InventoryStock
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sku_id IN ?", skuIDs).Order("sku_id ASC").Find(&stocks).Error; err != nil {
			return err
		}
		if len(stocks) == 0 {
			return nil
		}
		if err := tx.Model(&dalmodel.InventoryStock{}).
			Where("sku_id IN ?", skuIDs).
			Updates(map[string]any{"status": 0}).Error; err != nil {
			return err
		}
		return tx.Where("sku_id IN ?", skuIDs).Order("sku_id ASC").Find(&stocks).Error
	})
	if err != nil {
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
