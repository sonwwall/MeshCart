package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/errno"
	dalmodel "meshcart/services/order-service/dal/model"

	"go.uber.org/zap"
)

func (s *OrderService) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderpb.Order, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || len(req.GetItems()) == 0 {
		return nil, common.ErrInvalidParam
	}

	requestID := strings.TrimSpace(req.GetRequestId())
	if requestID != "" {
		existing, bizErr := s.findActionRecord(ctx, actionTypeCreate, requestID)
		if bizErr != nil {
			return nil, bizErr
		}
		if existing != nil {
			switch existing.Status {
			case "succeeded":
				return s.loadOrderByActionRecord(ctx, existing)
			case actionStatusPending:
				return nil, errno.ErrOrderIdempotencyBusy
			default:
				return nil, errno.ErrOrderStateConflict
			}
		}
		record, bizErr := s.createPendingActionRecord(ctx, actionTypeCreate, requestID, 0, req.GetUserId())
		if bizErr != nil {
			return nil, bizErr
		}
		if record != nil && record.Status != actionStatusPending {
			if record.Status == "succeeded" {
				return s.loadOrderByActionRecord(ctx, record)
			}
			return nil, errno.ErrOrderIdempotencyBusy
		}
	}

	orderID := s.node.Generate().Int64()
	validatedItems, reserveItems, totalAmount, bizErr := s.validateAndBuildSnapshots(ctx, req.GetItems())
	if bizErr != nil {
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
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
		s.markActionFailed(ctx, actionTypeCreate, requestID, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if reserveResp.Code != 0 {
		bizErr = mapInventoryRPCError(reserveResp.Code)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
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
		bizErr = mapRepositoryError(err)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	s.markActionSucceeded(ctx, actionTypeCreate, requestID, order.OrderID)
	return toRPCOrder(order), nil
}
