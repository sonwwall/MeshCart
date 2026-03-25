package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"
	dalmodel "meshcart/services/product-service/dal/model"

	"go.uber.org/zap"
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
			logx.L(ctx).Warn("list products rejected by invalid status", zap.Int32("status", status))
			return nil, 0, common.ErrInvalidParam
		}
		filter.Status = &status
	}
	if req.IsSetCategoryId() {
		categoryID := req.GetCategoryId()
		if categoryID < 0 {
			logx.L(ctx).Warn("list products rejected by invalid category_id", zap.Int64("category_id", categoryID))
			return nil, 0, common.ErrInvalidParam
		}
		filter.CategoryID = &categoryID
	}
	if req.IsSetCreatorId() {
		creatorID := req.GetCreatorId()
		if creatorID <= 0 {
			logx.L(ctx).Warn("list products rejected by invalid creator_id", zap.Int64("creator_id", creatorID))
			return nil, 0, common.ErrInvalidParam
		}
		filter.CreatorID = &creatorID
	}

	cacheKey := productListCacheKey(req, page, pageSize)
	if s.cache != nil {
		items, total, hit, err := s.cache.GetProductList(ctx, cacheKey)
		if err != nil {
			logx.L(ctx).Warn("list products cache read failed", zap.Error(err), zap.String("cache_key", cacheKey))
		} else if hit {
			logx.L(ctx).Info("list products cache hit",
				zap.Int32("page", page),
				zap.Int32("page_size", pageSize),
				zap.Int64("total", total),
				zap.Int("item_count", len(items)),
			)
			return items, total, nil
		}
	}

	products, total, err := s.repo.List(ctx, filter)
	if err != nil {
		logx.L(ctx).Error("list products repository list failed",
			zap.Error(err),
			zap.Int32("page", page),
			zap.Int32("page_size", pageSize),
			zap.String("keyword", req.GetKeyword()),
		)
		return nil, 0, common.ErrInternalError
	}

	productIDs := make([]int64, 0, len(products))
	for _, item := range products {
		productIDs = append(productIDs, item.ID)
	}
	skus, err := s.repo.ListSKUsByProductIDs(ctx, productIDs)
	if err != nil {
		logx.L(ctx).Error("list products repository list skus failed",
			zap.Error(err),
			zap.Int64s("product_ids", productIDs),
		)
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
	logx.L(ctx).Info("list products completed",
		zap.Int32("page", page),
		zap.Int32("page_size", pageSize),
		zap.Int64("total", total),
		zap.Int("item_count", len(items)),
	)
	if s.cache != nil {
		if err := s.cache.SetProductList(ctx, cacheKey, items, total); err != nil {
			logx.L(ctx).Warn("list products cache write failed", zap.Error(err), zap.String("cache_key", cacheKey))
		}
	}
	return items, total, nil
}
