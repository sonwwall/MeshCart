package service

import (
	"context"
	"errors"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/errno"
	"meshcart/services/order-service/biz/repository"

	"go.uber.org/zap"
)

func (s *OrderService) CancelOrder(ctx context.Context, req *orderpb.CancelOrderRequest) (*orderpb.Order, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || req.GetOrderId() <= 0 {
		return nil, common.ErrInvalidParam
	}

	order, err := s.repo.GetByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	switch order.Status {
	case OrderStatusCancelled, OrderStatusClosed:
		return toRPCOrder(order), nil
	case OrderStatusPaid:
		return nil, errno.ErrOrderPaidImmutable
	case OrderStatusPending, OrderStatusReserved:
	default:
		return nil, errno.ErrOrderStateConflict
	}

	cancelReason := strings.TrimSpace(req.GetCancelReason())
	if cancelReason == "" {
		cancelReason = "user_cancelled"
	}

	releaseResp, releaseErr := s.inventoryClient.ReleaseReservedSkuStocks(ctx, &inventory.ReleaseReservedSkuStocksRequest{
		BizType: orderReserveBizType,
		BizId:   s.reserveBizID(order.OrderID),
		Items:   buildReleaseItems(order),
	})
	if releaseErr != nil {
		logx.L(ctx).Error("release inventory for cancel order failed", zap.Error(releaseErr), zap.Int64("order_id", order.OrderID))
		return nil, common.ErrServiceUnavailable
	}
	if releaseResp.Code != 0 {
		return nil, mapInventoryRPCError(releaseResp.Code)
	}

	updated, updateErr := s.repo.UpdateStatus(ctx, order.OrderID, []int32{OrderStatusPending, OrderStatusReserved}, OrderStatusCancelled, cancelReason)
	if updateErr == nil {
		return toRPCOrder(updated), nil
	}
	if !errors.Is(updateErr, repository.ErrOrderStateConflict) {
		return nil, mapRepositoryError(updateErr)
	}

	current, currentErr := s.repo.GetByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if currentErr != nil {
		return nil, mapRepositoryError(currentErr)
	}
	switch current.Status {
	case OrderStatusCancelled, OrderStatusClosed:
		return toRPCOrder(current), nil
	case OrderStatusPaid:
		return nil, errno.ErrOrderPaidImmutable
	default:
		return nil, errno.ErrOrderStateConflict
	}
}
