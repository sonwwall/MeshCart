package service

import (
	"errors"
	"fmt"

	"meshcart/app/common"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/errno"
	"meshcart/services/product-service/biz/repository"
	dalmodel "meshcart/services/product-service/dal/model"
)

func mapRepositoryError(err error) *common.BizError {
	switch err {
	case nil:
		return nil
	case repository.ErrProductNotFound:
		return errno.ErrProductNotFound
	case repository.ErrSKUNotFound:
		return errno.ErrSKUNotFound
	case repository.ErrSKUCodeExists:
		return errno.ErrSKUCodeExists
	default:
		var duplicateErr *repository.DuplicateKeyError
		if errors.As(err, &duplicateErr) {
			if duplicateErr.Key != "" {
				return common.NewBizError(errno.CodeProductDataConflict, fmt.Sprintf("商品数据写入冲突: %s", duplicateErr.Key))
			}
			return errno.ErrProductDataConflict
		}
		return common.ErrInternalError
	}
}

func toRPCProduct(productModel *dalmodel.Product) *productpb.Product {
	if productModel == nil {
		return nil
	}

	skus := make([]*productpb.ProductSku, 0, len(productModel.Skus))
	for idx := range productModel.Skus {
		sku := productModel.Skus[idx]
		skus = append(skus, toRPCSKU(&sku))
	}

	return &productpb.Product{
		Id:          productModel.ID,
		Title:       productModel.Title,
		SubTitle:    productModel.SubTitle,
		CategoryId:  productModel.CategoryID,
		Brand:       productModel.Brand,
		Description: productModel.Description,
		Status:      productModel.Status,
		Skus:        skus,
	}
}

func toRPCSKU(skuModel *dalmodel.ProductSKU) *productpb.ProductSku {
	if skuModel == nil {
		return nil
	}

	attrs := make([]*productpb.ProductSkuAttr, 0, len(skuModel.Attrs))
	for idx := range skuModel.Attrs {
		attr := skuModel.Attrs[idx]
		attrs = append(attrs, &productpb.ProductSkuAttr{
			Id:        attr.ID,
			SkuId:     attr.SKUID,
			AttrName:  attr.AttrName,
			AttrValue: attr.AttrValue,
			Sort:      attr.Sort,
		})
	}

	return &productpb.ProductSku{
		Id:          skuModel.ID,
		SpuId:       skuModel.SPUID,
		SkuCode:     skuModel.SKUCode,
		Title:       skuModel.Title,
		SalePrice:   skuModel.SalePrice,
		MarketPrice: skuModel.MarketPrice,
		Status:      skuModel.Status,
		CoverUrl:    skuModel.CoverURL,
		Attrs:       attrs,
	}
}

func toRPCProductListItem(productModel *dalmodel.Product, skus []*dalmodel.ProductSKU) *productpb.ProductListItem {
	item := &productpb.ProductListItem{
		Id:         productModel.ID,
		Title:      productModel.Title,
		SubTitle:   productModel.SubTitle,
		CategoryId: productModel.CategoryID,
		Brand:      productModel.Brand,
		Status:     productModel.Status,
	}

	if len(skus) == 0 {
		return item
	}

	item.MinSalePrice = skus[0].SalePrice
	item.CoverUrl = skus[0].CoverURL
	for _, sku := range skus[1:] {
		if sku.SalePrice < item.MinSalePrice {
			item.MinSalePrice = sku.SalePrice
		}
		if item.CoverUrl == "" && sku.CoverURL != "" {
			item.CoverUrl = sku.CoverURL
		}
	}
	return item
}
