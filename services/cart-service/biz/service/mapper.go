package service

import (
	"errors"

	"meshcart/app/common"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	"meshcart/services/cart-service/biz/errno"
	"meshcart/services/cart-service/biz/repository"
	dalmodel "meshcart/services/cart-service/dal/model"
)

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
