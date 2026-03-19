package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	dalmodel "meshcart/services/cart-service/dal/model"

	"go.uber.org/zap"
)

func (s *CartService) AddCartItem(ctx context.Context, req *cartpb.AddCartItemRequest) (*cartpb.CartItem, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || req.GetProductId() <= 0 || req.GetSkuId() <= 0 || req.GetQuantity() <= 0 {
		logx.L(ctx).Warn("add cart item rejected by invalid request",
			zap.Int64("user_id", req.GetUserId()),
			zap.Int64("product_id", req.GetProductId()),
			zap.Int64("sku_id", req.GetSkuId()),
			zap.Int32("quantity", req.GetQuantity()),
		)
		return nil, common.ErrInvalidParam
	}

	title := strings.TrimSpace(req.GetTitleSnapshot())
	skuTitle := strings.TrimSpace(req.GetSkuTitleSnapshot())
	if title == "" || skuTitle == "" || req.GetSalePriceSnapshot() < 0 {
		logx.L(ctx).Warn("add cart item rejected by invalid snapshot",
			zap.Int64("user_id", req.GetUserId()),
			zap.Int64("product_id", req.GetProductId()),
			zap.Int64("sku_id", req.GetSkuId()),
		)
		return nil, common.ErrInvalidParam
	}

	checked := true
	if req.IsSetChecked() {
		checked = req.GetChecked()
	}

	item, err := s.repo.AddOrAccumulate(ctx, &dalmodel.CartItem{
		ID:                s.node.Generate().Int64(),
		UserID:            req.GetUserId(),
		ProductID:         req.GetProductId(),
		SKUID:             req.GetSkuId(),
		Quantity:          req.GetQuantity(),
		Checked:           checked,
		TitleSnapshot:     title,
		SKUTitleSnapshot:  skuTitle,
		SalePriceSnapshot: req.GetSalePriceSnapshot(),
		CoverURLSnapshot:  strings.TrimSpace(req.GetCoverUrlSnapshot()),
	})
	if err != nil {
		logx.L(ctx).Error("add cart item repository failed",
			zap.Error(err),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int64("product_id", req.GetProductId()),
			zap.Int64("sku_id", req.GetSkuId()),
		)
		return nil, common.ErrInternalError
	}
	logx.L(ctx).Info("add cart item completed",
		zap.Int64("user_id", item.UserID),
		zap.Int64("item_id", item.ID),
		zap.Int64("sku_id", item.SKUID),
		zap.Int32("quantity", item.Quantity),
	)
	return toRPCCartItem(item), nil
}
