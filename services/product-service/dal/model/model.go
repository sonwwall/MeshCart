package model

import "time"

type Product struct {
	ID          int64        `gorm:"column:id;primaryKey"`
	Title       string       `gorm:"column:title;type:varchar(255);not null"`
	SubTitle    string       `gorm:"column:sub_title;type:varchar(255);not null;default:''"`
	CategoryID  int64        `gorm:"column:category_id;not null;default:0"`
	Brand       string       `gorm:"column:brand;type:varchar(128);not null;default:''"`
	Description string       `gorm:"column:description;type:text"`
	Status      int32        `gorm:"column:status;type:tinyint;not null;default:0"`
	CreatedAt   time.Time    `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time    `gorm:"column:updated_at;autoUpdateTime"`
	Skus        []ProductSKU `gorm:"foreignKey:SPUID;references:ID"`
}

func (Product) TableName() string {
	return "products"
}

type ProductSKU struct {
	ID          int64            `gorm:"column:id;primaryKey"`
	SPUID       int64            `gorm:"column:spu_id;index:idx_spu_id;not null"`
	SKUCode     string           `gorm:"column:sku_code;type:varchar(64);uniqueIndex:uk_sku_code;not null"`
	Title       string           `gorm:"column:title;type:varchar(255);not null"`
	SalePrice   int64            `gorm:"column:sale_price;not null"`
	MarketPrice int64            `gorm:"column:market_price;not null;default:0"`
	Status      int32            `gorm:"column:status;type:tinyint;not null;default:0"`
	CoverURL    string           `gorm:"column:cover_url;type:varchar(512);not null;default:''"`
	CreatedAt   time.Time        `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time        `gorm:"column:updated_at;autoUpdateTime"`
	Attrs       []ProductSKUAttr `gorm:"foreignKey:SKUID;references:ID"`
}

func (ProductSKU) TableName() string {
	return "product_skus"
}

type ProductSKUAttr struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	SKUID     int64     `gorm:"column:sku_id;index:idx_sku_id;not null"`
	AttrName  string    `gorm:"column:attr_name;type:varchar(64);not null"`
	AttrValue string    `gorm:"column:attr_value;type:varchar(128);not null"`
	Sort      int32     `gorm:"column:sort;not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ProductSKUAttr) TableName() string {
	return "product_sku_attrs"
}
