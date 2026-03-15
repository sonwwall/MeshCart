package service

import "meshcart/services/inventory-service/biz/repository"

const (
	StockStatusFrozen int32 = 0
	StockStatusActive int32 = 1
)

type InventoryService struct {
	repo repository.InventoryRepository
}

func NewInventoryService(repo repository.InventoryRepository) *InventoryService {
	return &InventoryService{repo: repo}
}

func isValidStockStatus(status int32) bool {
	return status == StockStatusFrozen || status == StockStatusActive
}
