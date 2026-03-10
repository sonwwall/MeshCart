package service

import (
	"context"

	"meshcart/app/common"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"
	dalmodel "meshcart/services/product-service/dal/model"
)

func (s *ProductService) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) ([]*productpb.ProductListItem, int64, *common.BizError) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	filter := repository.ListFilter{
		Page:     page,
		PageSize: pageSize,
		Keyword:  req.GetKeyword(),
	}
	if req.IsSetStatus() {
		status := req.GetStatus()
		if !isValidProductStatus(status) {
			return nil, 0, common.ErrInvalidParam
		}
		filter.Status = &status
	}
	if req.IsSetCategoryId() {
		categoryID := req.GetCategoryId()
		if categoryID < 0 {
			return nil, 0, common.ErrInvalidParam
		}
		filter.CategoryID = &categoryID
	}

	products, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, common.ErrInternalError
	}

	productIDs := make([]int64, 0, len(products))
	for _, item := range products {
		productIDs = append(productIDs, item.ID)
	}
	skus, err := s.repo.ListSKUsByProductIDs(ctx, productIDs)
	if err != nil {
		return nil, 0, common.ErrInternalError
	}

	skuMap := make(map[int64][]*dalmodel.ProductSKU, len(products))
	for _, sku := range skus {
		skuMap[sku.SPUID] = append(skuMap[sku.SPUID], sku)
	}

	items := make([]*productpb.ProductListItem, 0, len(products))
	for _, item := range products {
		items = append(items, toRPCProductListItem(item, skuMap[item.ID]))
	}
	return items, total, nil
}
