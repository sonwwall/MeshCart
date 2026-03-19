package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	dalmodel "meshcart/services/payment-service/dal/model"

	"gorm.io/gorm"
)

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrInvalidPayment       = errors.New("invalid payment")
	ErrPaymentStateConflict = errors.New("payment state conflict")
	ErrActionRecordNotFound = errors.New("payment action record not found")
	ErrActionRecordExists   = errors.New("payment action record exists")
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *dalmodel.Payment) (*dalmodel.Payment, error)
	GetByPaymentID(ctx context.Context, paymentID int64) (*dalmodel.Payment, error)
	GetByPaymentIDUser(ctx context.Context, paymentID, userID int64) (*dalmodel.Payment, error)
	ListByOrderID(ctx context.Context, orderID, userID int64) ([]*dalmodel.Payment, error)
	GetLatestActiveByOrderID(ctx context.Context, orderID, userID int64) (*dalmodel.Payment, error)
	TransitionStatus(ctx context.Context, transition PaymentTransition) (*dalmodel.Payment, error)
	GetActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.PaymentActionRecord, error)
	CreateActionRecord(ctx context.Context, record *dalmodel.PaymentActionRecord) error
	MarkActionRecordSucceeded(ctx context.Context, actionType, actionKey string, paymentID, orderID int64) error
	MarkActionRecordFailed(ctx context.Context, actionType, actionKey, errorMessage string) error
}

type PaymentTransition struct {
	PaymentID      int64
	FromStatuses   []int32
	ToStatus       int32
	PaymentMethod  string
	PaymentTradeNo string
	SucceededAt    *time.Time
	ClosedAt       *time.Time
	FailReason     string
	ActionType     string
	Reason         string
	ExternalRef    string
}

type MySQLPaymentRepository struct {
	db           *gorm.DB
	queryTimeout time.Duration
}

func NewMySQLPaymentRepository(db *gorm.DB, queryTimeout time.Duration) *MySQLPaymentRepository {
	return &MySQLPaymentRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLPaymentRepository) Create(ctx context.Context, payment *dalmodel.Payment) (*dalmodel.Payment, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if payment == nil || payment.PaymentID <= 0 || payment.OrderID <= 0 || payment.UserID <= 0 || payment.Amount < 0 || strings.TrimSpace(payment.PaymentMethod) == "" {
		return nil, ErrInvalidPayment
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(payment).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrActionRecordExists
			}
			return err
		}
		return tx.Create(&dalmodel.PaymentStatusLog{
			ID:         payment.PaymentID,
			PaymentID:  payment.PaymentID,
			FromStatus: 0,
			ToStatus:   payment.Status,
			ActionType: "create",
			Reason:     "payment_created",
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return r.GetByPaymentID(ctx, payment.PaymentID)
}

func (r *MySQLPaymentRepository) GetByPaymentID(ctx context.Context, paymentID int64) (*dalmodel.Payment, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var payment dalmodel.Payment
	if err := r.db.WithContext(ctx).Where("payment_id = ?", paymentID).Take(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &payment, nil
}

func (r *MySQLPaymentRepository) GetByPaymentIDUser(ctx context.Context, paymentID, userID int64) (*dalmodel.Payment, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var payment dalmodel.Payment
	if err := r.db.WithContext(ctx).Where("payment_id = ? AND user_id = ?", paymentID, userID).Take(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &payment, nil
}

func (r *MySQLPaymentRepository) ListByOrderID(ctx context.Context, orderID, userID int64) ([]*dalmodel.Payment, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if orderID <= 0 || userID <= 0 {
		return nil, ErrInvalidPayment
	}
	var payments []*dalmodel.Payment
	if err := r.db.WithContext(ctx).
		Where("order_id = ? AND user_id = ?", orderID, userID).
		Order("created_at DESC, payment_id DESC").
		Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}

func (r *MySQLPaymentRepository) GetLatestActiveByOrderID(ctx context.Context, orderID, userID int64) (*dalmodel.Payment, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var payment dalmodel.Payment
	if err := r.db.WithContext(ctx).
		Where("order_id = ? AND user_id = ? AND status IN ?", orderID, userID, []int32{1, 2}).
		Order("created_at DESC, payment_id DESC").
		Take(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &payment, nil
}

func (r *MySQLPaymentRepository) TransitionStatus(ctx context.Context, transition PaymentTransition) (*dalmodel.Payment, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if transition.PaymentID <= 0 || len(transition.FromStatuses) == 0 {
		return nil, ErrInvalidPayment
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var payment dalmodel.Payment
		if err := tx.Where("payment_id = ? AND status IN ?", transition.PaymentID, transition.FromStatuses).Take(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPaymentStateConflict
			}
			return err
		}

		updates := map[string]any{
			"status":      transition.ToStatus,
			"fail_reason": transition.FailReason,
		}
		if transition.PaymentMethod != "" {
			updates["payment_method"] = transition.PaymentMethod
		}
		if transition.PaymentTradeNo != "" {
			updates["payment_trade_no"] = transition.PaymentTradeNo
		}
		if transition.SucceededAt != nil {
			updates["succeeded_at"] = transition.SucceededAt
		}
		if transition.ClosedAt != nil {
			updates["closed_at"] = transition.ClosedAt
		}
		if err := tx.Model(&dalmodel.Payment{}).Where("payment_id = ?", transition.PaymentID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&dalmodel.PaymentStatusLog{
			ID:          time.Now().UnixNano(),
			PaymentID:   transition.PaymentID,
			FromStatus:  payment.Status,
			ToStatus:    transition.ToStatus,
			ActionType:  transition.ActionType,
			Reason:      transition.Reason,
			ExternalRef: transition.ExternalRef,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return r.GetByPaymentID(ctx, transition.PaymentID)
}

func (r *MySQLPaymentRepository) GetActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.PaymentActionRecord, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var record dalmodel.PaymentActionRecord
	if err := r.db.WithContext(ctx).Where("action_type = ? AND action_key = ?", actionType, actionKey).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrActionRecordNotFound
		}
		return nil, err
	}
	return &record, nil
}

func (r *MySQLPaymentRepository) CreateActionRecord(ctx context.Context, record *dalmodel.PaymentActionRecord) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if record == nil || record.ID <= 0 || strings.TrimSpace(record.ActionType) == "" || strings.TrimSpace(record.ActionKey) == "" {
		return ErrInvalidPayment
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ErrActionRecordExists
		}
		return err
	}
	return nil
}

func (r *MySQLPaymentRepository) MarkActionRecordSucceeded(ctx context.Context, actionType, actionKey string, paymentID, orderID int64) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	result := r.db.WithContext(ctx).Model(&dalmodel.PaymentActionRecord{}).
		Where("action_type = ? AND action_key = ?", actionType, actionKey).
		Updates(map[string]any{
			"status":        "succeeded",
			"payment_id":    paymentID,
			"order_id":      orderID,
			"error_message": "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrActionRecordNotFound
	}
	return nil
}

func (r *MySQLPaymentRepository) MarkActionRecordFailed(ctx context.Context, actionType, actionKey, errorMessage string) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	result := r.db.WithContext(ctx).Model(&dalmodel.PaymentActionRecord{}).
		Where("action_type = ? AND action_key = ?", actionType, actionKey).
		Updates(map[string]any{
			"status":        "failed",
			"error_message": truncateString(errorMessage, 255),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
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
