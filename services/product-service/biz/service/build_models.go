package service

import (
	"strings"

	"meshcart/app/common"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"
	dalmodel "meshcart/services/product-service/dal/model"
)

func (s *ProductService) buildModelsForWrite(
	productID int64,
	title, subTitle string,
	categoryID int64,
	brand, description string,
	status int32,
	skus []*productpb.ProductSkuInput,
	creatorID int64,
	operatorID int64,
) (*dalmodel.Product, []*dalmodel.ProductSKU, *common.BizError) {
	title = strings.TrimSpace(title)
	subTitle = strings.TrimSpace(subTitle)
	brand = strings.TrimSpace(brand)
	description = strings.TrimSpace(description)

	if title == "" || categoryID < 0 || !isValidProductStatus(status) || len(skus) == 0 {
		return nil, nil, common.ErrInvalidParam
	}

	if productID == 0 {
		productID = s.node.Generate().Int64()
	}

	productModel := &dalmodel.Product{
		ID:          productID,
		Title:       title,
		SubTitle:    subTitle,
		CategoryID:  categoryID,
		Brand:       brand,
		Description: description,
		Status:      status,
		CreatorID:   creatorID,
		UpdatedBy:   operatorID,
	}

	skuCodeSet := make(map[string]struct{}, len(skus))
	skuModels := make([]*dalmodel.ProductSKU, 0, len(skus))
	for _, sku := range skus {
		if sku == nil {
			return nil, nil, common.ErrInvalidParam
		}
		skuCode := strings.TrimSpace(sku.SkuCode)
		skuTitle := strings.TrimSpace(sku.Title)
		coverURL := strings.TrimSpace(sku.CoverUrl)
		if skuCode == "" || skuTitle == "" || sku.SalePrice < 0 || sku.MarketPrice < 0 || !isValidSKUStatus(sku.Status) {
			return nil, nil, common.ErrInvalidParam
		}
		if _, exists := skuCodeSet[skuCode]; exists {
			return nil, nil, mapRepositoryError(repository.ErrSKUCodeExists)
		}
		skuCodeSet[skuCode] = struct{}{}

		skuID := int64(0)
		providedID := sku.IsSetId()
		if sku.IsSetId() {
			skuID = sku.GetId()
		}
		if skuID == 0 {
			skuID = s.node.Generate().Int64()
		}

		attrModels := make([]dalmodel.ProductSKUAttr, 0, len(sku.Attrs))
		for _, attr := range sku.Attrs {
			if attr == nil {
				return nil, nil, common.ErrInvalidParam
			}
			attrName := strings.TrimSpace(attr.AttrName)
			attrValue := strings.TrimSpace(attr.AttrValue)
			if attrName == "" || attrValue == "" {
				return nil, nil, common.ErrInvalidParam
			}
			attrModels = append(attrModels, dalmodel.ProductSKUAttr{
				ID:        s.node.Generate().Int64(),
				SKUID:     skuID,
				AttrName:  attrName,
				AttrValue: attrValue,
				Sort:      attr.Sort,
			})
		}

		skuModels = append(skuModels, &dalmodel.ProductSKU{
			ID:          skuID,
			SPUID:       productID,
			SKUCode:     skuCode,
			Title:       skuTitle,
			SalePrice:   sku.SalePrice,
			MarketPrice: sku.MarketPrice,
			Status:      sku.Status,
			CoverURL:    coverURL,
			Attrs:       attrModels,
			ProvidedID:  providedID,
		})
	}

	return productModel, skuModels, nil
}
