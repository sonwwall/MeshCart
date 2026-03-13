package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	dalmodel "meshcart/services/cart-service/dal/model"
)

func (s *CartService) AddCartItem(ctx context.Context, req *cartpb.AddCartItemRequest) (*cartpb.CartItem, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || req.GetProductId() <= 0 || req.GetSkuId() <= 0 || req.GetQuantity() <= 0 {
		return nil, common.ErrInvalidParam
	}

	title := strings.TrimSpace(req.GetTitleSnapshot())
	skuTitle := strings.TrimSpace(req.GetSkuTitleSnapshot())
	if title == "" || skuTitle == "" || req.GetSalePriceSnapshot() < 0 {
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
		return nil, common.ErrInternalError
	}
	return toRPCCartItem(item), nil
}
