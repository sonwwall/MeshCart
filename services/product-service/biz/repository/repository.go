package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dalmodel "meshcart/services/product-service/dal/model"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrProductNotFound = errors.New("product not found")
	ErrSKUNotFound     = errors.New("sku not found")
	ErrSKUCodeExists   = errors.New("sku code already exists")
)

type DuplicateKeyError struct {
	Key string
	Err error
}

func (e *DuplicateKeyError) Error() string {
	if e == nil {
		return ""
	}
	if e.Key == "" {
		return fmt.Sprintf("duplicate key: %v", e.Err)
	}
	return fmt.Sprintf("duplicate key %s: %v", e.Key, e.Err)
}

func (e *DuplicateKeyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type ListFilter struct {
	Page       int32
	PageSize   int32
	Status     *int32
	CategoryID *int64
	Keyword    string
}

type ProductRepository interface {
	Create(ctx context.Context, product *dalmodel.Product, skus []*dalmodel.ProductSKU) error
	Update(ctx context.Context, product *dalmodel.Product, skus []*dalmodel.ProductSKU) error
	ChangeStatus(ctx context.Context, productID int64, status int32) error
	GetByID(ctx context.Context, productID int64) (*dalmodel.Product, error)
	List(ctx context.Context, filter ListFilter) ([]*dalmodel.Product, int64, error)
	ListSKUsByProductIDs(ctx context.Context, productIDs []int64) ([]*dalmodel.ProductSKU, error)
	GetSKUsByIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.ProductSKU, error)
}

type MySQLProductRepository struct {
	db *gorm.DB
}

func NewMySQLProductRepository(db *gorm.DB) *MySQLProductRepository {
	return &MySQLProductRepository{db: db}
}

func (r *MySQLProductRepository) Create(ctx context.Context, product *dalmodel.Product, skus []*dalmodel.ProductSKU) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(product).Error; err != nil {
			return mapSQLError(err)
		}
		for _, sku := range skus {
			if err := tx.Omit("Attrs").Create(sku).Error; err != nil {
				return mapSQLError(err)
			}
			if len(sku.Attrs) == 0 {
				continue
			}
			if err := tx.Create(&sku.Attrs).Error; err != nil {
				return mapSQLError(err)
			}
		}
		return nil
	})
}

func (r *MySQLProductRepository) Update(ctx context.Context, product *dalmodel.Product, skus []*dalmodel.ProductSKU) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingProduct dalmodel.Product
		if err := tx.Select("id").Where("id = ?", product.ID).First(&existingProduct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductNotFound
			}
			return err
		}

		if err := tx.Model(&dalmodel.Product{}).
			Where("id = ?", product.ID).
			Updates(map[string]any{
				"title":       product.Title,
				"sub_title":   product.SubTitle,
				"category_id": product.CategoryID,
				"brand":       product.Brand,
				"description": product.Description,
				"status":      product.Status,
			}).Error; err != nil {
			return err
		}

		var existingSKUs []*dalmodel.ProductSKU
		if err := tx.Where("spu_id = ?", product.ID).Find(&existingSKUs).Error; err != nil {
			return err
		}

		existingSKUMap := make(map[int64]struct{}, len(existingSKUs))
		for _, sku := range existingSKUs {
			existingSKUMap[sku.ID] = struct{}{}
		}

		requestedSKUMap := make(map[int64]struct{}, len(skus))
		for _, sku := range skus {
			requestedSKUMap[sku.ID] = struct{}{}
			if _, ok := existingSKUMap[sku.ID]; ok {
				if err := tx.Model(&dalmodel.ProductSKU{}).
					Where("id = ? AND spu_id = ?", sku.ID, product.ID).
					Updates(map[string]any{
						"sku_code":     sku.SKUCode,
						"title":        sku.Title,
						"sale_price":   sku.SalePrice,
						"market_price": sku.MarketPrice,
						"status":       sku.Status,
						"cover_url":    sku.CoverURL,
					}).Error; err != nil {
					return mapSQLError(err)
				}
				if err := tx.Where("sku_id = ?", sku.ID).Delete(&dalmodel.ProductSKUAttr{}).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Omit("Attrs").Create(sku).Error; err != nil {
					return mapSQLError(err)
				}
			}

			if len(sku.Attrs) == 0 {
				continue
			}
			if err := tx.Create(&sku.Attrs).Error; err != nil {
				return mapSQLError(err)
			}
		}

		var staleSKUIds []int64
		for _, sku := range existingSKUs {
			if _, ok := requestedSKUMap[sku.ID]; !ok {
				staleSKUIds = append(staleSKUIds, sku.ID)
			}
		}
		if len(staleSKUIds) > 0 {
			if err := tx.Where("sku_id IN ?", staleSKUIds).Delete(&dalmodel.ProductSKUAttr{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ? AND spu_id = ?", staleSKUIds, product.ID).Delete(&dalmodel.ProductSKU{}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *MySQLProductRepository) ChangeStatus(ctx context.Context, productID int64, status int32) error {
	result := r.db.WithContext(ctx).Model(&dalmodel.Product{}).
		Where("id = ?", productID).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *MySQLProductRepository) GetByID(ctx context.Context, productID int64) (*dalmodel.Product, error) {
	var product dalmodel.Product
	err := r.db.WithContext(ctx).
		Preload("Skus", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("Skus.Attrs", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort ASC, id ASC")
		}).
		Where("id = ?", productID).
		First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return &product, nil
}

func (r *MySQLProductRepository) List(ctx context.Context, filter ListFilter) ([]*dalmodel.Product, int64, error) {
	query := r.db.WithContext(ctx).Model(&dalmodel.Product{})
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", *filter.CategoryID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR sub_title LIKE ? OR brand LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var products []*dalmodel.Product
	err := query.Order("id DESC").
		Limit(int(filter.PageSize)).
		Offset(int((filter.Page - 1) * filter.PageSize)).
		Find(&products).Error
	if err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *MySQLProductRepository) ListSKUsByProductIDs(ctx context.Context, productIDs []int64) ([]*dalmodel.ProductSKU, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}

	var skus []*dalmodel.ProductSKU
	if err := r.db.WithContext(ctx).
		Where("spu_id IN ?", productIDs).
		Order("id ASC").
		Find(&skus).Error; err != nil {
		return nil, err
	}
	return skus, nil
}

func (r *MySQLProductRepository) GetSKUsByIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.ProductSKU, error) {
	if len(skuIDs) == 0 {
		return nil, nil
	}

	var skus []*dalmodel.ProductSKU
	err := r.db.WithContext(ctx).
		Preload("Attrs", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort ASC, id ASC")
		}).
		Where("id IN ?", skuIDs).
		Order("id ASC").
		Find(&skus).Error
	if err != nil {
		return nil, err
	}
	return skus, nil
}

func mapSQLError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return &DuplicateKeyError{Err: err}
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		key := duplicateKeyName(mysqlErr.Message)
		if strings.Contains(key, "uk_sku_code") {
			return ErrSKUCodeExists
		}
		return &DuplicateKeyError{
			Key: key,
			Err: err,
		}
	}
	return err
}

func duplicateKeyName(message string) string {
	if message == "" {
		return ""
	}

	idx := strings.LastIndex(message, "for key ")
	if idx < 0 {
		return ""
	}

	keyPart := strings.TrimSpace(message[idx+len("for key "):])
	keyPart = strings.Trim(keyPart, "'` ")
	return keyPart
}
