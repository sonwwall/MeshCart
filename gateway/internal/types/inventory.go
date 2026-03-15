package types

type InventoryBatchGetRequest struct {
	SKUIDs []int64 `json:"sku_ids"`
}

type AdjustInventoryStockRequest struct {
	TotalStock int64  `json:"total_stock"`
	Reason     string `json:"reason"`
}

type InventoryStockData struct {
	SKUID          int64 `json:"sku_id"`
	TotalStock     int64 `json:"total_stock"`
	ReservedStock  int64 `json:"reserved_stock"`
	AvailableStock int64 `json:"available_stock"`
	SaleableStock  int64 `json:"saleable_stock"`
	Status         int32 `json:"status"`
}

type InventoryBatchData struct {
	Stocks []InventoryStockData `json:"stocks"`
}
