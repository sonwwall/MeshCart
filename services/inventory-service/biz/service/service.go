package service

import "meshcart/services/inventory-service/biz/repository"

const (
	StockStatusFrozen int32 = 0
	StockStatusActive int32 = 1
)

type InventoryService struct {
	repo         repository.InventoryRepository
	reserveGuard *skuGuard
}

type Option func(*InventoryService)

func WithReserveMaxConcurrencyPerSKU(limit int) Option {
	return func(s *InventoryService) {
		s.reserveGuard = newSKUGuard(limit)
	}
}

func NewInventoryService(repo repository.InventoryRepository, opts ...Option) *InventoryService {
	svc := &InventoryService{repo: repo, reserveGuard: newSKUGuard(0)}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func isValidStockStatus(status int32) bool {
	return status == StockStatusFrozen || status == StockStatusActive
}
