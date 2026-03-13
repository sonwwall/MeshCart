package repository

import (
	"context"
	"errors"
	"time"

	dalmodel "meshcart/services/cart-service/dal/model"

	"gorm.io/gorm"
)

var (
	ErrCartItemNotFound = errors.New("cart item not found")
)

type CartRepository interface {
	ListByUserID(ctx context.Context, userID int64) ([]*dalmodel.CartItem, error)
	AddOrAccumulate(ctx context.Context, item *dalmodel.CartItem) (*dalmodel.CartItem, error)
	UpdateByID(ctx context.Context, userID, itemID int64, quantity int32, checked *bool) (*dalmodel.CartItem, error)
	DeleteByID(ctx context.Context, userID, itemID int64) error
	ClearByUserID(ctx context.Context, userID int64) error
}

type MySQLCartRepository struct {
	db           *gorm.DB
	queryTimeout time.Duration
}

func NewMySQLCartRepository(db *gorm.DB, queryTimeout time.Duration) *MySQLCartRepository {
	return &MySQLCartRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLCartRepository) ListByUserID(ctx context.Context, userID int64) ([]*dalmodel.CartItem, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var items []*dalmodel.CartItem
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC, id DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *MySQLCartRepository) AddOrAccumulate(ctx context.Context, item *dalmodel.CartItem) (*dalmodel.CartItem, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if item == nil {
		return nil, errors.New("nil cart item")
	}

	var result dalmodel.CartItem
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing dalmodel.CartItem
		err := tx.
			Where("user_id = ? AND sku_id = ?", item.UserID, item.SKUID).
			Take(&existing).Error
		switch {
		case err == nil:
			updates := map[string]any{
				"product_id":          item.ProductID,
				"quantity":            existing.Quantity + item.Quantity,
				"checked":             item.Checked,
				"title_snapshot":      item.TitleSnapshot,
				"sku_title_snapshot":  item.SKUTitleSnapshot,
				"sale_price_snapshot": item.SalePriceSnapshot,
				"cover_url_snapshot":  item.CoverURLSnapshot,
			}
			if err := tx.Model(&dalmodel.CartItem{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
				return err
			}
			return tx.Where("id = ?", existing.ID).Take(&result).Error
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(item).Error; err != nil {
				return err
			}
			result = *item
			return nil
		default:
			return err
		}
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *MySQLCartRepository) UpdateByID(ctx context.Context, userID, itemID int64, quantity int32, checked *bool) (*dalmodel.CartItem, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var item dalmodel.CartItem
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", itemID, userID).Take(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCartItemNotFound
			}
			return err
		}

		updates := map[string]any{
			"quantity": quantity,
		}
		if checked != nil {
			updates["checked"] = *checked
		}
		if err := tx.Model(&dalmodel.CartItem{}).Where("id = ?", itemID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", itemID).Take(&item).Error
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *MySQLCartRepository) DeleteByID(ctx context.Context, userID, itemID int64) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", itemID, userID).Delete(&dalmodel.CartItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCartItemNotFound
	}
	return nil
}

func (r *MySQLCartRepository) ClearByUserID(ctx context.Context, userID int64) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&dalmodel.CartItem{}).Error
}

func withQueryTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}
