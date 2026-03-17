package model

import "time"

type Order struct {
	OrderID      int64       `gorm:"column:order_id;primaryKey"`
	UserID       int64       `gorm:"column:user_id;not null;index:idx_orders_user_id_status,priority:1"`
	Status       int32       `gorm:"column:status;type:tinyint;not null;default:1;index:idx_orders_user_id_status,priority:2;index:idx_orders_status_expire_at,priority:1"`
	TotalAmount  int64       `gorm:"column:total_amount;not null;default:0"`
	PayAmount    int64       `gorm:"column:pay_amount;not null;default:0"`
	ExpireAt     time.Time   `gorm:"column:expire_at;not null;index:idx_orders_status_expire_at,priority:2"`
	CancelReason string      `gorm:"column:cancel_reason;type:varchar(255);not null;default:''"`
	PaymentID    string      `gorm:"column:payment_id;type:varchar(128);not null;default:''"`
	PaidAt       *time.Time  `gorm:"column:paid_at"`
	CreatedAt    time.Time   `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time   `gorm:"column:updated_at;autoUpdateTime;index:idx_orders_updated_at"`
	Items        []OrderItem `gorm:"foreignKey:OrderID;references:OrderID"`
}

func (Order) TableName() string {
	return "orders"
}

type OrderItem struct {
	ID                   int64     `gorm:"column:id;primaryKey"`
	OrderID              int64     `gorm:"column:order_id;not null;index:idx_order_items_order_id"`
	ProductID            int64     `gorm:"column:product_id;not null;default:0"`
	SKUID                int64     `gorm:"column:sku_id;not null;index:idx_order_items_sku_id"`
	ProductTitleSnapshot string    `gorm:"column:product_title_snapshot;type:varchar(255);not null;default:''"`
	SKUTitleSnapshot     string    `gorm:"column:sku_title_snapshot;type:varchar(255);not null;default:''"`
	SalePriceSnapshot    int64     `gorm:"column:sale_price_snapshot;not null;default:0"`
	Quantity             int32     `gorm:"column:quantity;not null;default:1"`
	SubtotalAmount       int64     `gorm:"column:subtotal_amount;not null;default:0"`
	CreatedAt            time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (OrderItem) TableName() string {
	return "order_items"
}

type OrderActionRecord struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	ActionType   string    `gorm:"column:action_type;type:varchar(32);not null;uniqueIndex:uk_order_action_type_key,priority:1"`
	ActionKey    string    `gorm:"column:action_key;type:varchar(128);not null;uniqueIndex:uk_order_action_type_key,priority:2"`
	OrderID      int64     `gorm:"column:order_id;not null;default:0;index:idx_order_action_order_id"`
	UserID       int64     `gorm:"column:user_id;not null;default:0"`
	Status       string    `gorm:"column:status;type:varchar(16);not null;default:'pending'"`
	ErrorMessage string    `gorm:"column:error_message;type:varchar(255);not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (OrderActionRecord) TableName() string {
	return "order_action_records"
}

type OrderStatusLog struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	OrderID     int64     `gorm:"column:order_id;not null;index:idx_order_status_logs_order_id"`
	FromStatus  int32     `gorm:"column:from_status;type:tinyint;not null;default:0"`
	ToStatus    int32     `gorm:"column:to_status;type:tinyint;not null"`
	ActionType  string    `gorm:"column:action_type;type:varchar(32);not null;default:''"`
	Reason      string    `gorm:"column:reason;type:varchar(255);not null;default:''"`
	ExternalRef string    `gorm:"column:external_ref;type:varchar(128);not null;default:''"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (OrderStatusLog) TableName() string {
	return "order_status_logs"
}
