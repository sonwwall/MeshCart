package model

import "time"

type Order struct {
	OrderID     int64       `gorm:"column:order_id;primaryKey"`
	UserID      int64       `gorm:"column:user_id;not null;index:idx_orders_user_id_status,priority:1"`
	Status      int32       `gorm:"column:status;type:tinyint;not null;default:1;index:idx_orders_user_id_status,priority:2"`
	TotalAmount int64       `gorm:"column:total_amount;not null;default:0"`
	PayAmount   int64       `gorm:"column:pay_amount;not null;default:0"`
	ExpireAt    time.Time   `gorm:"column:expire_at;not null"`
	CreatedAt   time.Time   `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time   `gorm:"column:updated_at;autoUpdateTime;index:idx_orders_updated_at"`
	Items       []OrderItem `gorm:"foreignKey:OrderID;references:OrderID"`
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
