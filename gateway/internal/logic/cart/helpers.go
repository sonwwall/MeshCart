package cart

import (
	"meshcart/gateway/internal/types"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	productpb "meshcart/kitex_gen/meshcart/product"
)

const (
	productStatusOnline            int32 = 2
	skuStatusActive                int32 = 1
	inventoryCodeInsufficientStock int32 = 2050002
)

func toCartData(items []*cartpb.CartItem) *types.CartData {
	result := make([]types.CartItemData, 0, len(items))
	for _, item := range items {
		result = append(result, types.CartItemData{
			ID:                item.GetId(),
			ProductID:         item.GetProductId(),
			SKUID:             item.GetSkuId(),
			Quantity:          item.GetQuantity(),
			Checked:           item.GetChecked(),
			TitleSnapshot:     item.GetTitleSnapshot(),
			SKUTitleSnapshot:  item.GetSkuTitleSnapshot(),
			SalePriceSnapshot: item.GetSalePriceSnapshot(),
			CoverURLSnapshot:  item.GetCoverUrlSnapshot(),
		})
	}
	return &types.CartData{Items: result}
}

func findSKU(product *productpb.Product, skuID int64) *productpb.ProductSku {
	if product == nil {
		return nil
	}
	for _, sku := range product.GetSkus() {
		if sku.GetId() == skuID {
			return sku
		}
	}
	return nil
}

func findCartItem(items []*cartpb.CartItem, itemID int64) *cartpb.CartItem {
	for _, item := range items {
		if item != nil && item.GetId() == itemID {
			return item
		}
	}
	return nil
}
