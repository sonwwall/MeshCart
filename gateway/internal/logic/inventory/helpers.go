package inventory

import (
	"meshcart/app/common"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/types"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
)

func roleOf(identity *middleware.AuthIdentity) string {
	if identity == nil {
		return authz.RoleGuest
	}
	if identity.Role == "" {
		return authz.RoleUser
	}
	return identity.Role
}

func requireInventoryRead(identity *middleware.AuthIdentity, access *authz.AccessController) *common.BizError {
	if identity == nil || !access.Enforce(roleOf(identity), "inventory", authz.ActionRead, 0, identity.UserID, 0) {
		return common.ErrForbidden
	}
	return nil
}

func requireInventoryWrite(identity *middleware.AuthIdentity, access *authz.AccessController) *common.BizError {
	if identity == nil || !access.Enforce(roleOf(identity), "inventory", authz.ActionWrite, 0, identity.UserID, 0) {
		return common.ErrForbidden
	}
	return nil
}

func toStockData(stock *inventorypb.SkuStock) *types.InventoryStockData {
	if stock == nil {
		return nil
	}
	return &types.InventoryStockData{
		SKUID:          stock.GetSkuId(),
		TotalStock:     stock.GetTotalStock(),
		ReservedStock:  stock.GetReservedStock(),
		AvailableStock: stock.GetAvailableStock(),
		SaleableStock:  stock.GetSaleableStock(),
	}
}

func toStocksData(stocks []*inventorypb.SkuStock) []types.InventoryStockData {
	result := make([]types.InventoryStockData, 0, len(stocks))
	for _, stock := range stocks {
		if data := toStockData(stock); data != nil {
			result = append(result, *data)
		}
	}
	return result
}
