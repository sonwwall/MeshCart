package service

import (
	"context"
	"errors"
	"strings"

	"github.com/bwmarrin/snowflake"

	"meshcart/app/common"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	"meshcart/services/cart-service/biz/errno"
	"meshcart/services/cart-service/biz/repository"
	dalmodel "meshcart/services/cart-service/dal/model"
)

type CartService struct {
	repo repository.CartRepository
	node *snowflake.Node
}

func NewCartService(repo repository.CartRepository, node *snowflake.Node) *CartService {
	return &CartService{repo: repo, node: node}
}

func (s *CartService) GetCart(ctx context.Context, userID int64) ([]*cartpb.CartItem, *common.BizError) {
	if userID <= 0 {
		return nil, common.ErrInvalidParam
	}
	items, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, common.ErrInternalError
	}
	return toRPCCartItems(items), nil
}

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

func (s *CartService) UpdateCartItem(ctx context.Context, req *cartpb.UpdateCartItemRequest) (*cartpb.CartItem, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || req.GetItemId() <= 0 || req.GetQuantity() <= 0 {
		return nil, common.ErrInvalidParam
	}

	item, err := s.repo.UpdateByID(ctx, req.GetUserId(), req.GetItemId(), req.GetQuantity(), req.Checked)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCCartItem(item), nil
}

func (s *CartService) RemoveCartItem(ctx context.Context, userID, itemID int64) *common.BizError {
	if userID <= 0 || itemID <= 0 {
		return common.ErrInvalidParam
	}
	if err := s.repo.DeleteByID(ctx, userID, itemID); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

func (s *CartService) ClearCart(ctx context.Context, userID int64) *common.BizError {
	if userID <= 0 {
		return common.ErrInvalidParam
	}
	if err := s.repo.ClearByUserID(ctx, userID); err != nil {
		return common.ErrInternalError
	}
	return nil
}

func mapRepositoryError(err error) *common.BizError {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrCartItemNotFound):
		return errno.ErrCartItemNotFound
	default:
		return common.ErrInternalError
	}
}

func toRPCCartItems(items []*dalmodel.CartItem) []*cartpb.CartItem {
	result := make([]*cartpb.CartItem, 0, len(items))
	for _, item := range items {
		result = append(result, toRPCCartItem(item))
	}
	return result
}

func toRPCCartItem(item *dalmodel.CartItem) *cartpb.CartItem {
	if item == nil {
		return nil
	}
	return &cartpb.CartItem{
		Id:                item.ID,
		UserId:            item.UserID,
		ProductId:         item.ProductID,
		SkuId:             item.SKUID,
		Quantity:          item.Quantity,
		Checked:           item.Checked,
		TitleSnapshot:     item.TitleSnapshot,
		SkuTitleSnapshot:  item.SKUTitleSnapshot,
		SalePriceSnapshot: item.SalePriceSnapshot,
		CoverUrlSnapshot:  item.CoverURLSnapshot,
	}
}
