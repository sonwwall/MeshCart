package service

import (
	"context"
	"sort"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	dalmodel "meshcart/services/order-service/dal/model"

	"go.uber.org/zap"
)

func (s *OrderService) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderpb.Order, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || len(req.GetItems()) == 0 {
		return nil, common.ErrInvalidParam
	}

	orderID := s.node.Generate().Int64()
	validatedItems, reserveItems, totalAmount, bizErr := s.validateAndBuildSnapshots(ctx, req.GetItems())
	if bizErr != nil {
		return nil, bizErr
	}
	sort.SliceStable(reserveItems, func(i, j int) bool { return reserveItems[i].GetSkuId() < reserveItems[j].GetSkuId() })

	bizID := s.reserveBizID(orderID)
	reserveResp, err := s.inventoryClient.ReserveSkuStocks(ctx, &inventory.ReserveSkuStocksRequest{
		BizType: orderReserveBizType,
		BizId:   bizID,
		Items:   reserveItems,
	})
	if err != nil {
		logx.L(ctx).Error("reserve inventory failed", zap.Error(err), zap.Int64("order_id", orderID), zap.Int64("user_id", req.GetUserId()))
		return nil, common.ErrServiceUnavailable
	}
	if reserveResp.Code != 0 {
		return nil, mapInventoryRPCError(reserveResp.Code)
	}

	items := make([]*dalmodel.OrderItem, 0, len(validatedItems))
	for _, item := range validatedItems {
		items = append(items, &dalmodel.OrderItem{
			ID:                   s.node.Generate().Int64(),
			OrderID:              orderID,
			ProductID:            item.productID,
			SKUID:                item.skuID,
			ProductTitleSnapshot: item.productTitleSnapshot,
			SKUTitleSnapshot:     item.skuTitleSnapshot,
			SalePriceSnapshot:    item.salePriceSnapshot,
			Quantity:             item.quantity,
			SubtotalAmount:       item.subtotalAmount,
		})
	}

	orderModel := &dalmodel.Order{
		OrderID:      orderID,
		UserID:       req.GetUserId(),
		Status:       OrderStatusReserved,
		TotalAmount:  totalAmount,
		PayAmount:    totalAmount,
		ExpireAt:     s.now().Add(30 * time.Minute),
		CancelReason: "",
	}

	order, err := s.repo.CreateWithItems(ctx, orderModel, items)
	if err != nil {
		releaseResp, releaseErr := s.inventoryClient.ReleaseReservedSkuStocks(ctx, &inventory.ReleaseReservedSkuStocksRequest{
			BizType: orderReserveBizType,
			BizId:   bizID,
			Items:   reserveItems,
		})
		if releaseErr != nil {
			logx.L(ctx).Error("release inventory after create order failure failed", zap.Error(releaseErr), zap.Int64("order_id", orderID))
		} else if releaseResp.Code != 0 {
			logx.L(ctx).Error("release inventory after create order failure returned biz error", zap.Int32("code", releaseResp.Code), zap.String("message", releaseResp.Message), zap.Int64("order_id", orderID))
		}
		return nil, mapRepositoryError(err)
	}
	return toRPCOrder(order), nil
}
