package model

import "time"

type PaymentConsumeRecord struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	ConsumerGroup string    `gorm:"column:consumer_group;type:varchar(128);not null;uniqueIndex:uk_payment_consumer_event,priority:1"`
	EventID       string    `gorm:"column:event_id;type:varchar(128);not null;uniqueIndex:uk_payment_consumer_event,priority:2"`
	EventName     string    `gorm:"column:event_name;type:varchar(128);not null"`
	Status        string    `gorm:"column:status;type:varchar(16);not null;default:'pending'"`
	ErrorMessage  string    `gorm:"column:error_message;type:varchar(255);not null;default:''"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (PaymentConsumeRecord) TableName() string {
	return "payment_consume_records"
}
