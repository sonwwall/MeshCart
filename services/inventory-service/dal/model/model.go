package model

import "time"

type InventoryStock struct {
	ID             int64     `gorm:"column:id;primaryKey"`
	SKUID          int64     `gorm:"column:sku_id;not null;uniqueIndex:uk_sku_id"`
	TotalStock     int64     `gorm:"column:total_stock;not null;default:0"`
	ReservedStock  int64     `gorm:"column:reserved_stock;not null;default:0"`
	AvailableStock int64     `gorm:"column:available_stock;not null;default:0"`
	Status         int32     `gorm:"column:status;type:tinyint;not null;default:1;index:idx_status"`
	Version        int64     `gorm:"column:version;not null;default:1"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime;index:idx_updated_at"`
}

func (InventoryStock) TableName() string {
	return "inventory_stocks"
}

type InventoryTxBranch struct {
	ID              int64     `gorm:"column:id;primaryKey"`
	GlobalTxID      string    `gorm:"column:global_tx_id;type:varchar(128);not null;uniqueIndex:uk_inventory_tx_branch_action,priority:1"`
	BranchID        string    `gorm:"column:branch_id;type:varchar(128);not null;uniqueIndex:uk_inventory_tx_branch_action,priority:2"`
	Action          string    `gorm:"column:action;type:varchar(64);not null;uniqueIndex:uk_inventory_tx_branch_action,priority:3"`
	BizID           int64     `gorm:"column:biz_id;not null;default:0;index:idx_inventory_tx_branch_biz_id"`
	Status          string    `gorm:"column:status;type:varchar(32);not null"`
	PayloadSnapshot string    `gorm:"column:payload_snapshot;type:text;not null"`
	ErrorMessage    string    `gorm:"column:error_message;type:varchar(255);not null;default:''"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (InventoryTxBranch) TableName() string {
	return "inventory_tx_branches"
}
