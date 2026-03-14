package service

import (
	productpb "meshcart/kitex_gen/meshcart/product"
	dalmodel "meshcart/services/product-service/dal/model"
)

func toProductSkusForCreate(skus []*dalmodel.ProductSKU) []*productpb.ProductSku {
	result := make([]*productpb.ProductSku, 0, len(skus))
	for _, sku := range skus {
		if sku == nil {
			continue
		}
		result = append(result, &productpb.ProductSku{
			Id:          sku.ID,
			SpuId:       sku.SPUID,
			SkuCode:     sku.SKUCode,
			Title:       sku.Title,
			SalePrice:   sku.SalePrice,
			MarketPrice: sku.MarketPrice,
			Status:      sku.Status,
			CoverUrl:    sku.CoverURL,
		})
	}
	return result
}
