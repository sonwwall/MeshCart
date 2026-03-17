package service

import (
	"errors"

	"meshcart/app/common"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/errno"
	"meshcart/services/order-service/biz/repository"
	dalmodel "meshcart/services/order-service/dal/model"
)

func mapRepositoryError(err error) *common.BizError {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrOrderNotFound):
		return errno.ErrOrderNotFound
	case errors.Is(err, repository.ErrInvalidOrder):
		return errno.ErrInvalidOrderData
	default:
		return common.ErrInternalError
	}
}

func toRPCOrder(order *dalmodel.Order) *orderpb.Order {
	if order == nil {
		return nil
	}
	items := make([]*orderpb.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &orderpb.OrderItem{
			ItemId:               item.ID,
			OrderId:              item.OrderID,
			ProductId:            item.ProductID,
			SkuId:                item.SKUID,
			ProductTitleSnapshot: item.ProductTitleSnapshot,
			SkuTitleSnapshot:     item.SKUTitleSnapshot,
			SalePriceSnapshot:    item.SalePriceSnapshot,
			Quantity:             item.Quantity,
			SubtotalAmount:       item.SubtotalAmount,
		})
	}
	expireAt := int64(0)
	if !order.ExpireAt.IsZero() {
		expireAt = order.ExpireAt.Unix()
	}
	return &orderpb.Order{
		OrderId:     order.OrderID,
		UserId:      order.UserID,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
		PayAmount:   order.PayAmount,
		ExpireAt:    expireAt,
		Items:       items,
	}
}

func toRPCOrders(orders []*dalmodel.Order) []*orderpb.Order {
	result := make([]*orderpb.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, toRPCOrder(order))
	}
	return result
}
