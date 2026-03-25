package service

import (
	"context"
	"errors"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	"meshcart/services/inventory-service/biz/errno"
	"meshcart/services/inventory-service/biz/repository"
	dalmodel "meshcart/services/inventory-service/dal/model"
)

func mapRepositoryError(err error) *common.BizError {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrStockNotFound):
		return errno.ErrInventoryStockNotFound
	case errors.Is(err, repository.ErrStockExists):
		return errno.ErrStockAlreadyExists
	case errors.Is(err, repository.ErrInvalidQuantity):
		return errno.ErrInvalidStockQuantity
	case errors.Is(err, repository.ErrInsufficientStock):
		return errno.ErrInsufficientStock
	case errors.Is(err, repository.ErrStockFrozen):
		return errno.ErrStockFrozen
	case errors.Is(err, repository.ErrReservationConflict), errors.Is(err, repository.ErrReservationStateConflict):
		return errno.ErrReservationConflict
	case errors.Is(err, repository.ErrReservationNotFound):
		return errno.ErrReservationNotFound
	case errors.Is(err, repository.ErrReservationTimeout), errors.Is(err, context.DeadlineExceeded):
		return errno.ErrReservationTimeout
	default:
		return common.ErrInternalError
	}
}

func toRPCSkuStock(stock *dalmodel.InventoryStock) *inventorypb.SkuStock {
	if stock == nil {
		return nil
	}
	return &inventorypb.SkuStock{
		SkuId:          stock.SKUID,
		TotalStock:     stock.TotalStock,
		ReservedStock:  stock.ReservedStock,
		AvailableStock: stock.AvailableStock,
		SaleableStock:  stock.AvailableStock,
		Status:         stock.Status,
	}
}

func toRPCSkuStocks(stocks []*dalmodel.InventoryStock) []*inventorypb.SkuStock {
	result := make([]*inventorypb.SkuStock, 0, len(stocks))
	for _, stock := range stocks {
		result = append(result, toRPCSkuStock(stock))
	}
	return result
}
