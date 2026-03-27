package repository

import (
	"context"
	"strings"
	"time"

	logx "meshcart/app/log"
	dalmodel "meshcart/services/payment-service/dal/model"

	"go.uber.org/zap"
)

const (
	ConsumeStatusPending   = "pending"
	ConsumeStatusSucceeded = "succeeded"
	ConsumeStatusFailed    = "failed"
)

func (r *MySQLPaymentRepository) CreateConsumeRecord(ctx context.Context, record *dalmodel.PaymentConsumeRecord) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if record == nil || record.ID <= 0 || strings.TrimSpace(record.ConsumerGroup) == "" || strings.TrimSpace(record.EventID) == "" || strings.TrimSpace(record.EventName) == "" {
		return ErrInvalidPayment
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ErrActionRecordExists
		}
		logx.L(ctx).Error("create payment consume record failed",
			zap.Error(err),
			zap.String("consumer_group", record.ConsumerGroup),
			zap.String("event_id", record.EventID),
			zap.String("event_name", record.EventName),
		)
		return err
	}
	return nil
}

func (r *MySQLPaymentRepository) GetConsumeRecord(ctx context.Context, consumerGroup, eventID string) (*dalmodel.PaymentConsumeRecord, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var record dalmodel.PaymentConsumeRecord
	if err := r.db.WithContext(ctx).Where("consumer_group = ? AND event_id = ?", consumerGroup, eventID).Take(&record).Error; err != nil {
		if isRecordNotFound(err) {
			return nil, ErrActionRecordNotFound
		}
		logx.L(ctx).Error("get payment consume record failed",
			zap.Error(err),
			zap.String("consumer_group", consumerGroup),
			zap.String("event_id", eventID),
		)
		return nil, err
	}
	return &record, nil
}

func (r *MySQLPaymentRepository) MarkConsumeRecordSucceeded(ctx context.Context, id int64) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	return r.db.WithContext(ctx).Model(&dalmodel.PaymentConsumeRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":        ConsumeStatusSucceeded,
		"error_message": "",
		"updated_at":    time.Now(),
	}).Error
}

func (r *MySQLPaymentRepository) MarkConsumeRecordFailed(ctx context.Context, id int64, message string) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	return r.db.WithContext(ctx).Model(&dalmodel.PaymentConsumeRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":        ConsumeStatusFailed,
		"error_message": truncateString(message, 255),
		"updated_at":    time.Now(),
	}).Error
}
