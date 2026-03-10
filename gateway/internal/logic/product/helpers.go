package product

import (
	"meshcart/app/common"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	productpb "meshcart/kitex_gen/meshcart/product"
)

var errOwnProductRequired = common.NewBizError(common.CodeForbidden, "仅可操作自己创建的商品")

func roleOf(svcCtx *svc.ServiceContext, identity *middleware.AuthIdentity) string {
	if identity == nil {
		return authz.RoleGuest
	}
	if svcCtx != nil && svcCtx.AccessControl != nil && identity.UserID > 0 {
		return svcCtx.AccessControl.RoleForUser(identity.UserID)
	}
	if identity.Role == "" {
		return authz.RoleGuest
	}
	return identity.Role
}

func toListItemData(item *productpb.ProductListItem) types.ProductListItemData {
	return types.ProductListItemData{
		ID:           item.GetId(),
		Title:        item.GetTitle(),
		SubTitle:     item.GetSubTitle(),
		CategoryID:   item.GetCategoryId(),
		Brand:        item.GetBrand(),
		Status:       item.GetStatus(),
		MinSalePrice: item.GetMinSalePrice(),
		CoverURL:     item.GetCoverUrl(),
	}
}

func toDetailData(product *productpb.Product) *types.ProductDetailData {
	if product == nil {
		return nil
	}

	skus := make([]types.ProductSKUData, 0, len(product.GetSkus()))
	for _, sku := range product.GetSkus() {
		attrs := make([]types.ProductSKUAttrData, 0, len(sku.GetAttrs()))
		for _, attr := range sku.GetAttrs() {
			attrs = append(attrs, types.ProductSKUAttrData{
				ID:        attr.GetId(),
				SKUID:     attr.GetSkuId(),
				AttrName:  attr.GetAttrName(),
				AttrValue: attr.GetAttrValue(),
				Sort:      attr.GetSort(),
			})
		}
		skus = append(skus, types.ProductSKUData{
			ID:          sku.GetId(),
			SPUID:       sku.GetSpuId(),
			SKUCode:     sku.GetSkuCode(),
			Title:       sku.GetTitle(),
			SalePrice:   sku.GetSalePrice(),
			MarketPrice: sku.GetMarketPrice(),
			Status:      sku.GetStatus(),
			CoverURL:    sku.GetCoverUrl(),
			Attrs:       attrs,
		})
	}

	return &types.ProductDetailData{
		ID:          product.GetId(),
		Title:       product.GetTitle(),
		SubTitle:    product.GetSubTitle(),
		CategoryID:  product.GetCategoryId(),
		Brand:       product.GetBrand(),
		Description: product.GetDescription(),
		Status:      product.GetStatus(),
		SKUs:        skus,
	}
}
