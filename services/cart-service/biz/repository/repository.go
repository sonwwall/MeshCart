package repository

import (
	"context"
	"errors"
	"time"

	logx "meshcart/app/log"
	dalmodel "meshcart/services/cart-service/dal/model"

	"go.uber.org/zap"
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
		logx.L(ctx).Error("list cart items by user_id failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
		)
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
				logx.L(ctx).Error("accumulate cart item failed",
					zap.Error(err),
					zap.Int64("user_id", item.UserID),
					zap.Int64("item_id", existing.ID),
					zap.Int64("sku_id", item.SKUID),
				)
				return err
			}
			if err := tx.Where("id = ?", existing.ID).Take(&result).Error; err != nil {
				logx.L(ctx).Error("reload accumulated cart item failed",
					zap.Error(err),
					zap.Int64("user_id", item.UserID),
					zap.Int64("item_id", existing.ID),
				)
				return err
			}
			return nil
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(item).Error; err != nil {
				logx.L(ctx).Error("create cart item failed",
					zap.Error(err),
					zap.Int64("user_id", item.UserID),
					zap.Int64("item_id", item.ID),
					zap.Int64("sku_id", item.SKUID),
				)
				return err
			}
			result = *item
			return nil
		default:
			logx.L(ctx).Error("load existing cart item before add failed",
				zap.Error(err),
				zap.Int64("user_id", item.UserID),
				zap.Int64("sku_id", item.SKUID),
			)
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
			logx.L(ctx).Error("load cart item for update failed",
				zap.Error(err),
				zap.Int64("user_id", userID),
				zap.Int64("item_id", itemID),
			)
			return err
		}

		updates := map[string]any{
			"quantity": quantity,
		}
		if checked != nil {
			updates["checked"] = *checked
		}
		if err := tx.Model(&dalmodel.CartItem{}).Where("id = ?", itemID).Updates(updates).Error; err != nil {
			logx.L(ctx).Error("update cart item failed",
				zap.Error(err),
				zap.Int64("user_id", userID),
				zap.Int64("item_id", itemID),
				zap.Int32("quantity", quantity),
			)
			return err
		}
		if err := tx.Where("id = ?", itemID).Take(&item).Error; err != nil {
			logx.L(ctx).Error("reload cart item after update failed",
				zap.Error(err),
				zap.Int64("user_id", userID),
				zap.Int64("item_id", itemID),
			)
			return err
		}
		return nil
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
		logx.L(ctx).Error("delete cart item failed",
			zap.Error(result.Error),
			zap.Int64("user_id", userID),
			zap.Int64("item_id", itemID),
		)
		return result.Error
	}
	if result.RowsAffected == 0 {
		logx.L(ctx).Warn("delete cart item missed item",
			zap.Int64("user_id", userID),
			zap.Int64("item_id", itemID),
		)
		return ErrCartItemNotFound
	}
	return nil
}

func (r *MySQLCartRepository) ClearByUserID(ctx context.Context, userID int64) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&dalmodel.CartItem{}).Error; err != nil {
		logx.L(ctx).Error("clear cart by user_id failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
		)
		return err
	}
	return nil
}

func withQueryTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}
