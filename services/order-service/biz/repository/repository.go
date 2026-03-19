package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	logx "meshcart/app/log"
	dalmodel "meshcart/services/order-service/dal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrOrderNotFound        = errors.New("order not found")
	ErrInvalidOrder         = errors.New("invalid order")
	ErrOrderStateConflict   = errors.New("order state conflict")
	ErrActionRecordNotFound = errors.New("order action record not found")
	ErrActionRecordExists   = errors.New("order action record exists")
)

type OrderRepository interface {
	CreateWithItems(ctx context.Context, order *dalmodel.Order, items []*dalmodel.OrderItem) (*dalmodel.Order, error)
	GetByOrderID(ctx context.Context, userID, orderID int64) (*dalmodel.Order, error)
	GetByID(ctx context.Context, orderID int64) (*dalmodel.Order, error)
	ListByUserID(ctx context.Context, userID int64, offset, limit int) ([]*dalmodel.Order, int64, error)
	UpdateStatus(ctx context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error)
	ListExpiredOrders(ctx context.Context, now time.Time, limit int) ([]*dalmodel.Order, error)
	TransitionStatus(ctx context.Context, transition OrderTransition) (*dalmodel.Order, error)
	GetActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.OrderActionRecord, error)
	CreateActionRecord(ctx context.Context, record *dalmodel.OrderActionRecord) error
	MarkActionRecordSucceeded(ctx context.Context, actionType, actionKey string, orderID int64) error
	MarkActionRecordFailed(ctx context.Context, actionType, actionKey, errorMessage string) error
}

type OrderTransition struct {
	OrderID        int64
	FromStatuses   []int32
	ToStatus       int32
	CancelReason   string
	PaymentID      string
	PaymentMethod  string
	PaymentTradeNo string
	PaidAt         *time.Time
	ActionType     string
	Reason         string
	ExternalRef    string
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
			logx.L(ctx).Error("create order insert failed",
				zap.Error(err),
				zap.Int64("order_id", order.OrderID),
				zap.Int64("user_id", order.UserID),
				zap.Int32("status", order.Status),
			)
			return err
		}
		for _, item := range items {
			if item == nil || item.ID <= 0 || item.OrderID != order.OrderID || item.SKUID <= 0 || item.Quantity <= 0 {
				return ErrInvalidOrder
			}
			if err := tx.Create(item).Error; err != nil {
				logx.L(ctx).Error("create order item failed",
					zap.Error(err),
					zap.Int64("order_id", item.OrderID),
					zap.Int64("item_id", item.ID),
					zap.Int64("sku_id", item.SKUID),
					zap.Int32("quantity", item.Quantity),
				)
				return err
			}
		}
		if err := tx.Create(&dalmodel.OrderStatusLog{
			ID:         order.OrderID,
			OrderID:    order.OrderID,
			FromStatus: 0,
			ToStatus:   order.Status,
			ActionType: "create",
			Reason:     "order_created",
		}).Error; err != nil {
			logx.L(ctx).Error("create order status log failed",
				zap.Error(err),
				zap.Int64("order_id", order.OrderID),
				zap.Int32("to_status", order.Status),
			)
			return err
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
		logx.L(ctx).Error("get order by order_id and user_id failed",
			zap.Error(err),
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", userID),
		)
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
		logx.L(ctx).Error("get order by order_id failed",
			zap.Error(err),
			zap.Int64("order_id", orderID),
		)
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
		logx.L(ctx).Error("count orders by user_id failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
		)
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
		logx.L(ctx).Error("list orders by user_id failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
			zap.Int("offset", offset),
			zap.Int("limit", limit),
		)
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *MySQLOrderRepository) UpdateStatus(ctx context.Context, orderID int64, fromStatuses []int32, toStatus int32, cancelReason string) (*dalmodel.Order, error) {
	return r.TransitionStatus(ctx, OrderTransition{
		OrderID:      orderID,
		FromStatuses: fromStatuses,
		ToStatus:     toStatus,
		CancelReason: cancelReason,
	})
}

func (r *MySQLOrderRepository) TransitionStatus(ctx context.Context, transition OrderTransition) (*dalmodel.Order, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if transition.OrderID <= 0 || len(transition.FromStatuses) == 0 {
		return nil, ErrInvalidOrder
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order dalmodel.Order
		if err := tx.Where("order_id = ? AND status IN ?", transition.OrderID, transition.FromStatuses).Take(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logx.L(ctx).Warn("order transition rejected by current status",
					zap.Int64("order_id", transition.OrderID),
					zap.Any("from_statuses", transition.FromStatuses),
					zap.Int32("to_status", transition.ToStatus),
					zap.String("action_type", transition.ActionType),
				)
				return ErrOrderStateConflict
			}
			logx.L(ctx).Error("load order for transition failed",
				zap.Error(err),
				zap.Int64("order_id", transition.OrderID),
				zap.Any("from_statuses", transition.FromStatuses),
				zap.Int32("to_status", transition.ToStatus),
				zap.String("action_type", transition.ActionType),
			)
			return err
		}

		updates := map[string]any{
			"status":        transition.ToStatus,
			"cancel_reason": transition.CancelReason,
		}
		if transition.PaymentID != "" {
			updates["payment_id"] = transition.PaymentID
		}
		if transition.PaymentMethod != "" {
			updates["payment_method"] = transition.PaymentMethod
		}
		if transition.PaymentTradeNo != "" {
			updates["payment_trade_no"] = transition.PaymentTradeNo
		}
		if transition.PaidAt != nil {
			updates["paid_at"] = transition.PaidAt
		}
		if err := tx.Model(&dalmodel.Order{}).Where("order_id = ?", transition.OrderID).Updates(updates).Error; err != nil {
			logx.L(ctx).Error("update order status failed",
				zap.Error(err),
				zap.Int64("order_id", transition.OrderID),
				zap.Int32("from_status", order.Status),
				zap.Int32("to_status", transition.ToStatus),
				zap.String("action_type", transition.ActionType),
			)
			return err
		}
		if err := tx.Create(&dalmodel.OrderStatusLog{
			ID:          time.Now().UnixNano(),
			OrderID:     transition.OrderID,
			FromStatus:  order.Status,
			ToStatus:    transition.ToStatus,
			ActionType:  transition.ActionType,
			Reason:      transition.Reason,
			ExternalRef: transition.ExternalRef,
		}).Error; err != nil {
			logx.L(ctx).Error("create order status log failed",
				zap.Error(err),
				zap.Int64("order_id", transition.OrderID),
				zap.Int32("from_status", order.Status),
				zap.Int32("to_status", transition.ToStatus),
				zap.String("action_type", transition.ActionType),
			)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, transition.OrderID)
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
		logx.L(ctx).Error("list expired orders failed",
			zap.Error(err),
			zap.Time("now", now),
			zap.Int("limit", limit),
		)
		return nil, err
	}
	return orders, nil
}

func (r *MySQLOrderRepository) GetActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.OrderActionRecord, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var record dalmodel.OrderActionRecord
	if err := r.db.WithContext(ctx).Where("action_type = ? AND action_key = ?", actionType, actionKey).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrActionRecordNotFound
		}
		logx.L(ctx).Error("get order action record failed",
			zap.Error(err),
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
		)
		return nil, err
	}
	return &record, nil
}

func (r *MySQLOrderRepository) CreateActionRecord(ctx context.Context, record *dalmodel.OrderActionRecord) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if record == nil || record.ID <= 0 || strings.TrimSpace(record.ActionType) == "" || strings.TrimSpace(record.ActionKey) == "" {
		return ErrInvalidOrder
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		if isUniqueConstraintError(err) {
			logx.L(ctx).Warn("create order action record duplicate key",
				zap.Error(err),
				zap.String("action_type", record.ActionType),
				zap.String("action_key", record.ActionKey),
				zap.Int64("order_id", record.OrderID),
				zap.Int64("user_id", record.UserID),
			)
			return ErrActionRecordExists
		}
		logx.L(ctx).Error("create order action record failed",
			zap.Error(err),
			zap.String("action_type", record.ActionType),
			zap.String("action_key", record.ActionKey),
			zap.Int64("order_id", record.OrderID),
			zap.Int64("user_id", record.UserID),
		)
		return err
	}
	return nil
}

func (r *MySQLOrderRepository) MarkActionRecordSucceeded(ctx context.Context, actionType, actionKey string, orderID int64) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	result := r.db.WithContext(ctx).Model(&dalmodel.OrderActionRecord{}).
		Where("action_type = ? AND action_key = ?", actionType, actionKey).
		Updates(map[string]any{
			"status":        "succeeded",
			"order_id":      orderID,
			"error_message": "",
		})
	if result.Error != nil {
		logx.L(ctx).Error("mark order action record succeeded failed",
			zap.Error(result.Error),
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
			zap.Int64("order_id", orderID),
		)
		return result.Error
	}
	if result.RowsAffected == 0 {
		logx.L(ctx).Warn("mark order action record succeeded missed record",
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
			zap.Int64("order_id", orderID),
		)
		return ErrActionRecordNotFound
	}
	return nil
}

func (r *MySQLOrderRepository) MarkActionRecordFailed(ctx context.Context, actionType, actionKey, errorMessage string) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	result := r.db.WithContext(ctx).Model(&dalmodel.OrderActionRecord{}).
		Where("action_type = ? AND action_key = ?", actionType, actionKey).
		Updates(map[string]any{
			"status":        "failed",
			"error_message": truncateString(errorMessage, 255),
		})
	if result.Error != nil {
		logx.L(ctx).Error("mark order action record failed state failed",
			zap.Error(result.Error),
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
			zap.String("error_message", truncateString(errorMessage, 255)),
		)
		return result.Error
	}
	if result.RowsAffected == 0 {
		logx.L(ctx).Warn("mark order action record failed missed record",
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
		)
		return ErrActionRecordNotFound
	}
	return nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "UNIQUE constraint failed")
}

func truncateString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func withQueryTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}
