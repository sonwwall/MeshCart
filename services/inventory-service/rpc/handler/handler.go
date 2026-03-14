package handler

import "meshcart/services/inventory-service/biz/service"

type InventoryServiceImpl struct {
	svc *service.InventoryService
}

func NewInventoryServiceImpl(svc *service.InventoryService) *InventoryServiceImpl {
	return &InventoryServiceImpl{svc: svc}
}
