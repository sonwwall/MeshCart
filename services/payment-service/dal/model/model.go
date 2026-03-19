package model

import "time"

type Payment struct {
	PaymentID      int64      `gorm:"column:payment_id;primaryKey"`
	OrderID        int64      `gorm:"column:order_id;not null;index:idx_payments_order_id_status,priority:1"`
	UserID         int64      `gorm:"column:user_id;not null;index:idx_payments_user_id"`
	Status         int32      `gorm:"column:status;type:tinyint;not null;default:1;index:idx_payments_order_id_status,priority:2;index:idx_payments_status_updated_at,priority:1"`
	PaymentMethod  string     `gorm:"column:payment_method;type:varchar(32);not null;default:''"`
	Amount         int64      `gorm:"column:amount;not null;default:0"`
	Currency       string     `gorm:"column:currency;type:varchar(8);not null;default:'CNY'"`
	PaymentTradeNo string     `gorm:"column:payment_trade_no;type:varchar(128);not null;default:''"`
	RequestID      string     `gorm:"column:request_id;type:varchar(128);not null;default:''"`
	ExpireAt       time.Time  `gorm:"column:expire_at;not null;index:idx_payments_status_expire_at,priority:2"`
	SucceededAt    *time.Time `gorm:"column:succeeded_at"`
	ClosedAt       *time.Time `gorm:"column:closed_at"`
	FailReason     string     `gorm:"column:fail_reason;type:varchar(255);not null;default:''"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime;index:idx_payments_status_updated_at,priority:2;index:idx_payments_status_expire_at,priority:1"`
}

func (Payment) TableName() string {
	return "payments"
}

type PaymentActionRecord struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	ActionType   string    `gorm:"column:action_type;type:varchar(32);not null;uniqueIndex:uk_payment_action_type_key,priority:1"`
	ActionKey    string    `gorm:"column:action_key;type:varchar(128);not null;uniqueIndex:uk_payment_action_type_key,priority:2"`
	PaymentID    int64     `gorm:"column:payment_id;not null;default:0;index:idx_payment_action_payment_id"`
	OrderID      int64     `gorm:"column:order_id;not null;default:0;index:idx_payment_action_order_id"`
	Status       string    `gorm:"column:status;type:varchar(16);not null;default:'pending'"`
	ErrorMessage string    `gorm:"column:error_message;type:varchar(255);not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (PaymentActionRecord) TableName() string {
	return "payment_action_records"
}

type PaymentStatusLog struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	PaymentID   int64     `gorm:"column:payment_id;not null;index:idx_payment_status_logs_payment_id"`
	FromStatus  int32     `gorm:"column:from_status;type:tinyint;not null;default:0"`
	ToStatus    int32     `gorm:"column:to_status;type:tinyint;not null"`
	ActionType  string    `gorm:"column:action_type;type:varchar(32);not null;default:''"`
	Reason      string    `gorm:"column:reason;type:varchar(255);not null;default:''"`
	ExternalRef string    `gorm:"column:external_ref;type:varchar(128);not null;default:''"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (PaymentStatusLog) TableName() string {
	return "payment_status_logs"
}
