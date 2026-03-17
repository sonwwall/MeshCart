package order

import (
	"meshcart/gateway/internal/types"
	orderpb "meshcart/kitex_gen/meshcart/order"
)

func toOrderData(order *orderpb.Order) *types.OrderData {
	if order == nil {
		return nil
	}
	items := make([]types.OrderItemData, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		items = append(items, types.OrderItemData{
			ItemID:               item.GetItemId(),
			OrderID:              item.GetOrderId(),
			ProductID:            item.GetProductId(),
			SKUID:                item.GetSkuId(),
			ProductTitleSnapshot: item.GetProductTitleSnapshot(),
			SKUTitleSnapshot:     item.GetSkuTitleSnapshot(),
			SalePriceSnapshot:    item.GetSalePriceSnapshot(),
			Quantity:             item.GetQuantity(),
			SubtotalAmount:       item.GetSubtotalAmount(),
		})
	}
	return &types.OrderData{
		OrderID:      order.GetOrderId(),
		UserID:       order.GetUserId(),
		Status:       order.GetStatus(),
		TotalAmount:  order.GetTotalAmount(),
		PayAmount:    order.GetPayAmount(),
		ExpireAt:     order.GetExpireAt(),
		CancelReason: order.GetCancelReason(),
		PaymentID:    order.GetPaymentId(),
		PaidAt:       order.GetPaidAt(),
		Items:        items,
	}
}

func toOrderListData(orders []*orderpb.Order, total int64) *types.OrderListData {
	items := make([]types.OrderData, 0, len(orders))
	for _, order := range orders {
		if data := toOrderData(order); data != nil {
			items = append(items, *data)
		}
	}
	return &types.OrderListData{Orders: items, Total: total}
}
