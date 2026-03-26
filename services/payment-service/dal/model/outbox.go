package model

import "time"

type PaymentOutbox struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	Topic       string     `gorm:"column:topic;type:varchar(128);not null"`
	EventName   string     `gorm:"column:event_name;type:varchar(128);not null"`
	EventKey    string     `gorm:"column:event_key;type:varchar(128);not null;default:'';index:idx_payment_outbox_event_key"`
	Producer    string     `gorm:"column:producer;type:varchar(64);not null"`
	HeadersJSON []byte     `gorm:"column:headers_json;type:json;not null"`
	Body        []byte     `gorm:"column:body;type:json;not null"`
	Status      string     `gorm:"column:status;type:varchar(16);not null;default:'pending';index:idx_payment_outbox_status_retry,priority:1"`
	RetryCount  int32      `gorm:"column:retry_count;not null;default:0"`
	MaxRetries  int32      `gorm:"column:max_retries;not null;default:16"`
	NextRetryAt *time.Time `gorm:"column:next_retry_at;index:idx_payment_outbox_status_retry,priority:2"`
	LastError   string     `gorm:"column:last_error;type:varchar(255);not null;default:''"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (PaymentOutbox) TableName() string {
	return "payment_outbox"
}
