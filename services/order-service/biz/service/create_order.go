package service

import (
	"context"
	"strings"
	"time"

	"meshcart/app/common"
	orderpb "meshcart/kitex_gen/meshcart/order"
	dalmodel "meshcart/services/order-service/dal/model"
)

func (s *OrderService) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderpb.Order, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || len(req.GetItems()) == 0 {
		return nil, common.ErrInvalidParam
	}

	orderID := s.node.Generate().Int64()
	totalAmount := int64(0)
	items := make([]*dalmodel.OrderItem, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		if item == nil ||
			item.GetProductId() <= 0 ||
			item.GetSkuId() <= 0 ||
			item.GetQuantity() <= 0 ||
			item.GetSalePriceSnapshot() < 0 ||
			strings.TrimSpace(item.GetProductTitleSnapshot()) == "" ||
			strings.TrimSpace(item.GetSkuTitleSnapshot()) == "" {
			return nil, common.ErrInvalidParam
		}

		subtotal := item.GetSalePriceSnapshot() * int64(item.GetQuantity())
		totalAmount += subtotal
		items = append(items, &dalmodel.OrderItem{
			ID:                   s.node.Generate().Int64(),
			OrderID:              orderID,
			ProductID:            item.GetProductId(),
			SKUID:                item.GetSkuId(),
			ProductTitleSnapshot: strings.TrimSpace(item.GetProductTitleSnapshot()),
			SKUTitleSnapshot:     strings.TrimSpace(item.GetSkuTitleSnapshot()),
			SalePriceSnapshot:    item.GetSalePriceSnapshot(),
			Quantity:             item.GetQuantity(),
			SubtotalAmount:       subtotal,
		})
	}

	orderModel := &dalmodel.Order{
		OrderID:     orderID,
		UserID:      req.GetUserId(),
		Status:      OrderStatusPending,
		TotalAmount: totalAmount,
		PayAmount:   totalAmount,
		ExpireAt:    time.Now().Add(30 * time.Minute),
	}

	order, err := s.repo.CreateWithItems(ctx, orderModel, items)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCOrder(order), nil
}
