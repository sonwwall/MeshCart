package repository

import (
	"context"
	"errors"
	"time"

	dalmodel "meshcart/services/order-service/dal/model"

	"gorm.io/gorm"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidOrder       = errors.New("invalid order")
	ErrOrderStateConflict = errors.New("order state conflict")
)

type OrderRepository interface {
	CreateWithItems(ctx context.Context, order *dalmodel.Order, items []*dalmodel.OrderItem) (*dalmodel.Order, error)
	GetByOrderID(ctx context.Context, userID, orderID int64) (*dalmodel.Order, error)
	GetByID(ctx context.Context, orderID int64) (*dalmodel.Order, error)
	ListByUserID(ctx context.Context, userID int64, offset, limit int) ([]*dalmodel.Order, int64, error)
	UpdateStatus(ctx context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error)
	ListExpiredOrders(ctx context.Context, now time.Time, limit int) ([]*dalmodel.Order, error)
}

type MySQLOrderRepository struct {
	db           *gorm.DB
	queryTimeout time.Duration
}

func NewMySQLOrderRepository(db *gorm.DB, queryTimeout time.Duration) *MySQLOrderRepository {
	return &MySQLOrderRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLOrderRepository) CreateWithItems(ctx context.Context, order *dalmodel.Order, items []*dalmodel.OrderItem) (*dalmodel.Order, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if order == nil || order.OrderID <= 0 || order.UserID <= 0 || len(items) == 0 {
		return nil, ErrInvalidOrder
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for _, item := range items {
			if item == nil || item.ID <= 0 || item.OrderID != order.OrderID || item.SKUID <= 0 || item.Quantity <= 0 {
				return ErrInvalidOrder
			}
			if err := tx.Create(item).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetByOrderID(ctx, order.UserID, order.OrderID)
}

func (r *MySQLOrderRepository) GetByOrderID(ctx context.Context, userID, orderID int64) (*dalmodel.Order, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var order dalmodel.Order
	if err := r.db.WithContext(ctx).
		Preload("Items").
		Where("order_id = ? AND user_id = ?", orderID, userID).
		Take(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *MySQLOrderRepository) GetByID(ctx context.Context, orderID int64) (*dalmodel.Order, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var order dalmodel.Order
	if err := r.db.WithContext(ctx).
		Preload("Items").
		Where("order_id = ?", orderID).
		Take(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *MySQLOrderRepository) ListByUserID(ctx context.Context, userID int64, offset, limit int) ([]*dalmodel.Order, int64, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if userID <= 0 || offset < 0 || limit <= 0 {
		return nil, 0, ErrInvalidOrder
	}

	var total int64
	if err := r.db.WithContext(ctx).Model(&dalmodel.Order{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []*dalmodel.Order
	if err := r.db.WithContext(ctx).
		Preload("Items").
		Where("user_id = ?", userID).
		Order("created_at DESC, order_id DESC").
		Offset(offset).
		Limit(limit).
		Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *MySQLOrderRepository) UpdateStatus(ctx context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if orderID <= 0 || len(fromStatuses) == 0 {
		return nil, ErrInvalidOrder
	}

	result := r.db.WithContext(ctx).
		Model(&dalmodel.Order{}).
		Where("order_id = ? AND status IN ?", orderID, fromStatuses).
		Updates(map[string]any{
			"status":        toStatus,
			"cancel_reason": cancelReason,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrOrderStateConflict
	}
	return r.GetByID(ctx, orderID)
}

func (r *MySQLOrderRepository) ListExpiredOrders(ctx context.Context, now time.Time, limit int) ([]*dalmodel.Order, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if limit <= 0 {
		return nil, ErrInvalidOrder
	}

	var orders []*dalmodel.Order
	if err := r.db.WithContext(ctx).
		Preload("Items").
		Where("status IN ? AND expire_at <= ?", []int32{1, 2}, now).
		Order("expire_at ASC, order_id ASC").
		Limit(limit).
		Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func withQueryTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}
