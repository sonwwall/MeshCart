package inventory

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.uber.org/zap"
)

var errOwnInventoryRequired = common.NewBizError(common.CodeForbidden, "仅可操作自己创建商品的库存")

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

func ensureInventoryOwnership(ctx context.Context, svcCtx *svc.ServiceContext, skuIDs []int64, identity *middleware.AuthIdentity, write bool) *common.BizError {
	if identity == nil {
		return common.ErrUnauthorized
	}
	role := roleOf(identity)
	if role == authz.RoleSuperAdmin {
		return nil
	}
	if role != authz.RoleAdmin {
		return common.ErrForbidden
	}
	if len(skuIDs) == 0 {
		return common.ErrInvalidParam
	}

	skuResp, err := svcCtx.ProductClient.BatchGetSKU(ctx, &productpb.BatchGetSkuRequest{SkuIds: skuIDs})
	if err != nil {
		logx.L(ctx).Error("product rpc batch get sku for inventory ownership failed", zap.Error(err))
		return logicutil.MapRPCError(err)
	}
	if skuResp.Code != common.CodeOK {
		return common.NewBizError(skuResp.Code, skuResp.Message)
	}
	if len(skuResp.Skus) != len(skuIDs) {
		return common.ErrNotFound
	}

	productCreators := make(map[int64]int64, len(skuResp.Skus))
	for _, sku := range skuResp.Skus {
		spuID := sku.GetSpuId()
		if _, ok := productCreators[spuID]; ok {
			continue
		}
		detailResp, err := svcCtx.ProductClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: spuID})
		if err != nil {
			logx.L(ctx).Error("product rpc detail for inventory ownership failed", zap.Error(err), zap.Int64("product_id", spuID))
			return logicutil.MapRPCError(err)
		}
		if detailResp.Code != common.CodeOK || detailResp.Product == nil {
			return common.NewBizError(detailResp.Code, detailResp.Message)
		}
		productCreators[spuID] = detailResp.Product.GetCreatorId()
	}

	for _, sku := range skuResp.Skus {
		if productCreators[sku.GetSpuId()] != identity.UserID {
			return errOwnInventoryRequired
		}
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
		Status:         stock.GetStatus(),
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
