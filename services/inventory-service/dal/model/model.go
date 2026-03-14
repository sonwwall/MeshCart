package model

import "time"

type InventoryStock struct {
	ID             int64     `gorm:"column:id;primaryKey"`
	SKUID          int64     `gorm:"column:sku_id;not null;uniqueIndex:uk_sku_id"`
	TotalStock     int64     `gorm:"column:total_stock;not null;default:0"`
	ReservedStock  int64     `gorm:"column:reserved_stock;not null;default:0"`
	AvailableStock int64     `gorm:"column:available_stock;not null;default:0"`
	Version        int64     `gorm:"column:version;not null;default:1"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime;index:idx_updated_at"`
}

func (InventoryStock) TableName() string {
	return "inventory_stocks"
}
