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

	requestID := strings.TrimSpace(req.GetRequestId())
	if requestID != "" {
		existing, bizErr := s.findActionRecord(ctx, actionTypeCancel, requestID)
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
		record, bizErr := s.createPendingActionRecord(ctx, actionTypeCancel, requestID, req.GetOrderId(), req.GetUserId())
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

	order, err := s.repo.GetByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		s.markActionFailed(ctx, actionTypeCancel, requestID, mapRepositoryError(err))
		return nil, mapRepositoryError(err)
	}
	switch order.Status {
	case OrderStatusCancelled, OrderStatusClosed:
		s.markActionSucceeded(ctx, actionTypeCancel, requestID, order.OrderID)
		return toRPCOrder(order), nil
	case OrderStatusPaid:
		s.markActionFailed(ctx, actionTypeCancel, requestID, errno.ErrOrderPaidImmutable)
		return nil, errno.ErrOrderPaidImmutable
	case OrderStatusPending, OrderStatusReserved:
	default:
		s.markActionFailed(ctx, actionTypeCancel, requestID, errno.ErrOrderStateConflict)
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
		s.markActionFailed(ctx, actionTypeCancel, requestID, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if releaseResp.Code != 0 {
		bizErr := mapInventoryRPCError(releaseResp.Code)
		s.markActionFailed(ctx, actionTypeCancel, requestID, bizErr)
		return nil, bizErr
	}

	updated, updateErr := s.repo.TransitionStatus(ctx, repository.OrderTransition{
		OrderID:      order.OrderID,
		FromStatuses: []int32{OrderStatusPending, OrderStatusReserved},
		ToStatus:     OrderStatusCancelled,
		CancelReason: cancelReason,
		ActionType:   actionTypeCancel,
		Reason:       cancelReason,
	})
	if updateErr == nil {
		s.markActionSucceeded(ctx, actionTypeCancel, requestID, updated.OrderID)
		return toRPCOrder(updated), nil
	}
	if !errors.Is(updateErr, repository.ErrOrderStateConflict) {
		s.markActionFailed(ctx, actionTypeCancel, requestID, mapRepositoryError(updateErr))
		return nil, mapRepositoryError(updateErr)
	}

	current, currentErr := s.repo.GetByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if currentErr != nil {
		return nil, mapRepositoryError(currentErr)
	}
	switch current.Status {
	case OrderStatusCancelled, OrderStatusClosed:
		s.markActionSucceeded(ctx, actionTypeCancel, requestID, current.OrderID)
		return toRPCOrder(current), nil
	case OrderStatusPaid:
		s.markActionFailed(ctx, actionTypeCancel, requestID, errno.ErrOrderPaidImmutable)
		return nil, errno.ErrOrderPaidImmutable
	default:
		s.markActionFailed(ctx, actionTypeCancel, requestID, errno.ErrOrderStateConflict)
		return nil, errno.ErrOrderStateConflict
	}
}
