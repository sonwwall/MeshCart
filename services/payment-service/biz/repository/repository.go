package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	logx "meshcart/app/log"
	dalmodel "meshcart/services/payment-service/dal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrInvalidPayment       = errors.New("invalid payment")
	ErrPaymentStateConflict = errors.New("payment state conflict")
	ErrActivePaymentExists  = errors.New("active payment already exists")
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
	CreateConsumeRecord(ctx context.Context, record *dalmodel.PaymentConsumeRecord) error
	GetConsumeRecord(ctx context.Context, consumerGroup, eventID string) (*dalmodel.PaymentConsumeRecord, error)
	MarkConsumeRecordSucceeded(ctx context.Context, id int64) error
	MarkConsumeRecordFailed(ctx context.Context, id int64, message string) error
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
	OutboxRecords  []*dalmodel.PaymentOutbox
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
			if isActivePaymentUniqueConstraintError(err) {
				logx.L(ctx).Warn("create payment rejected by active payment unique guard",
					zap.Error(err),
					zap.Int64("payment_id", payment.PaymentID),
					zap.Int64("order_id", payment.OrderID),
					zap.Int64("user_id", payment.UserID),
					zap.String("request_id", payment.RequestID),
				)
				return ErrActivePaymentExists
			}
			if isUniqueConstraintError(err) {
				logx.L(ctx).Warn("create payment duplicate key",
					zap.Error(err),
					zap.Int64("payment_id", payment.PaymentID),
					zap.Int64("order_id", payment.OrderID),
					zap.Int64("user_id", payment.UserID),
					zap.String("request_id", payment.RequestID),
				)
				return ErrActionRecordExists
			}
			logx.L(ctx).Error("create payment insert failed",
				zap.Error(err),
				zap.Int64("payment_id", payment.PaymentID),
				zap.Int64("order_id", payment.OrderID),
				zap.Int64("user_id", payment.UserID),
				zap.String("payment_method", payment.PaymentMethod),
			)
			return err
		}
		if err := tx.Create(&dalmodel.PaymentStatusLog{
			ID:         payment.PaymentID,
			PaymentID:  payment.PaymentID,
			FromStatus: 0,
			ToStatus:   payment.Status,
			ActionType: "create",
			Reason:     "payment_created",
		}).Error; err != nil {
			logx.L(ctx).Error("create payment status log failed",
				zap.Error(err),
				zap.Int64("payment_id", payment.PaymentID),
				zap.Int32("to_status", payment.Status),
			)
			return err
		}
		return nil
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
		logx.L(ctx).Error("get payment by payment_id failed", zap.Error(err), zap.Int64("payment_id", paymentID))
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
		logx.L(ctx).Error("get payment by payment_id and user_id failed",
			zap.Error(err),
			zap.Int64("payment_id", paymentID),
			zap.Int64("user_id", userID),
		)
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
		logx.L(ctx).Error("list payments by order_id failed",
			zap.Error(err),
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", userID),
		)
		return nil, err
	}
	return payments, nil
}

func (r *MySQLPaymentRepository) GetLatestActiveByOrderID(ctx context.Context, orderID, userID int64) (*dalmodel.Payment, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var payment dalmodel.Payment
	if err := r.db.WithContext(ctx).
		Where("active_order_id = ? AND user_id = ?", orderID, userID).
		Order("created_at DESC, payment_id DESC").
		Take(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		logx.L(ctx).Error("get latest active payment by order_id failed",
			zap.Error(err),
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", userID),
		)
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
				logx.L(ctx).Warn("payment transition rejected by current status",
					zap.Int64("payment_id", transition.PaymentID),
					zap.Any("from_statuses", transition.FromStatuses),
					zap.Int32("to_status", transition.ToStatus),
					zap.String("action_type", transition.ActionType),
				)
				return ErrPaymentStateConflict
			}
			logx.L(ctx).Error("load payment for transition failed",
				zap.Error(err),
				zap.Int64("payment_id", transition.PaymentID),
				zap.Any("from_statuses", transition.FromStatuses),
				zap.Int32("to_status", transition.ToStatus),
			)
			return err
		}

		updates := map[string]any{
			"status":      transition.ToStatus,
			"fail_reason": transition.FailReason,
		}
		if transition.ToStatus == 1 {
			updates["active_order_id"] = payment.OrderID
		} else {
			updates["active_order_id"] = nil
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
			logx.L(ctx).Error("update payment status failed",
				zap.Error(err),
				zap.Int64("payment_id", transition.PaymentID),
				zap.Int32("from_status", payment.Status),
				zap.Int32("to_status", transition.ToStatus),
				zap.String("action_type", transition.ActionType),
			)
			return err
		}
		if err := tx.Create(&dalmodel.PaymentStatusLog{
			ID:          time.Now().UnixNano(),
			PaymentID:   transition.PaymentID,
			FromStatus:  payment.Status,
			ToStatus:    transition.ToStatus,
			ActionType:  transition.ActionType,
			Reason:      transition.Reason,
			ExternalRef: transition.ExternalRef,
		}).Error; err != nil {
			logx.L(ctx).Error("create payment status log failed",
				zap.Error(err),
				zap.Int64("payment_id", transition.PaymentID),
				zap.Int32("from_status", payment.Status),
				zap.Int32("to_status", transition.ToStatus),
				zap.String("action_type", transition.ActionType),
			)
			return err
		}
		for _, record := range transition.OutboxRecords {
			if record == nil || record.ID <= 0 || strings.TrimSpace(record.Topic) == "" || strings.TrimSpace(record.EventName) == "" || strings.TrimSpace(record.Producer) == "" || len(record.Body) == 0 {
				return ErrInvalidPayment
			}
			if err := tx.Create(record).Error; err != nil {
				logx.L(ctx).Error("create payment outbox failed",
					zap.Error(err),
					zap.Int64("payment_id", transition.PaymentID),
					zap.Int64("outbox_id", record.ID),
					zap.String("topic", record.Topic),
					zap.String("event_name", record.EventName),
				)
				return err
			}
		}
		return nil
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
		logx.L(ctx).Error("get payment action record failed",
			zap.Error(err),
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
		)
		return nil, err
	}
	return &record, nil
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func (r *MySQLPaymentRepository) CreateActionRecord(ctx context.Context, record *dalmodel.PaymentActionRecord) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if record == nil || record.ID <= 0 || strings.TrimSpace(record.ActionType) == "" || strings.TrimSpace(record.ActionKey) == "" {
		return ErrInvalidPayment
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		if isUniqueConstraintError(err) {
			logx.L(ctx).Warn("create payment action record duplicate key",
				zap.Error(err),
				zap.String("action_type", record.ActionType),
				zap.String("action_key", record.ActionKey),
				zap.Int64("payment_id", record.PaymentID),
				zap.Int64("order_id", record.OrderID),
			)
			return ErrActionRecordExists
		}
		logx.L(ctx).Error("create payment action record failed",
			zap.Error(err),
			zap.String("action_type", record.ActionType),
			zap.String("action_key", record.ActionKey),
			zap.Int64("payment_id", record.PaymentID),
			zap.Int64("order_id", record.OrderID),
		)
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
		logx.L(ctx).Error("mark payment action record succeeded failed",
			zap.Error(result.Error),
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
			zap.Int64("payment_id", paymentID),
			zap.Int64("order_id", orderID),
		)
		return result.Error
	}
	if result.RowsAffected == 0 {
		logx.L(ctx).Warn("mark payment action record succeeded missed record",
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
			zap.Int64("payment_id", paymentID),
			zap.Int64("order_id", orderID),
		)
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
		logx.L(ctx).Error("mark payment action record failed status failed",
			zap.Error(result.Error),
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
			zap.String("error_message", truncateString(errorMessage, 255)),
		)
		return result.Error
	}
	if result.RowsAffected == 0 {
		logx.L(ctx).Warn("mark payment action record failed missed record",
			zap.String("action_type", actionType),
			zap.String("action_key", actionKey),
			zap.String("error_message", truncateString(errorMessage, 255)),
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

func isActivePaymentUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "uk_payments_active_order_id") ||
		strings.Contains(msg, "payments.active_order_id") ||
		strings.Contains(msg, "payments`.`active_order_id")
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
