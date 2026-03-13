package model

import "time"

type CartItem struct {
	ID                int64     `gorm:"column:id;primaryKey"`
	UserID            int64     `gorm:"column:user_id;not null;index:idx_user_id"`
	ProductID         int64     `gorm:"column:product_id;not null;default:0"`
	SKUID             int64     `gorm:"column:sku_id;not null"`
	Quantity          int32     `gorm:"column:quantity;not null;default:1"`
	Checked           bool      `gorm:"column:checked;not null;default:true"`
	TitleSnapshot     string    `gorm:"column:title_snapshot;type:varchar(255);not null;default:''"`
	SKUTitleSnapshot  string    `gorm:"column:sku_title_snapshot;type:varchar(255);not null;default:''"`
	SalePriceSnapshot int64     `gorm:"column:sale_price_snapshot;not null;default:0"`
	CoverURLSnapshot  string    `gorm:"column:cover_url_snapshot;type:varchar(512);not null;default:''"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (CartItem) TableName() string {
	return "cart_items"
}
