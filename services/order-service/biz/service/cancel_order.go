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
		userID := int64(0)
		orderID := int64(0)
		if req != nil {
			userID = req.GetUserId()
			orderID = req.GetOrderId()
		}
		logx.L(ctx).Warn("cancel order rejected by invalid request",
			zap.Int64("user_id", userID),
			zap.Int64("order_id", orderID),
		)
		return nil, common.ErrInvalidParam
	}

	requestID := strings.TrimSpace(req.GetRequestId())
	logx.L(ctx).Info("cancel order start",
		zap.Int64("user_id", req.GetUserId()),
		zap.Int64("order_id", req.GetOrderId()),
		zap.String("request_id", requestID),
	)
	if requestID != "" {
		existing, bizErr := s.findActionRecord(ctx, actionTypeCancel, requestID)
		if bizErr != nil {
			return nil, bizErr
		}
		if existing != nil {
			switch existing.Status {
			case "succeeded":
				logx.L(ctx).Info("cancel order hit succeeded action record",
					zap.String("request_id", requestID),
					zap.Int64("order_id", existing.OrderID),
					zap.Int64("user_id", req.GetUserId()),
				)
				return s.loadOrderByActionRecord(ctx, existing)
			case actionStatusPending:
				logx.L(ctx).Warn("cancel order blocked by pending action record",
					zap.String("request_id", requestID),
					zap.Int64("order_id", req.GetOrderId()),
				)
				return nil, errno.ErrOrderIdempotencyBusy
			default:
				logx.L(ctx).Warn("cancel order rejected by failed action record",
					zap.String("request_id", requestID),
					zap.Int64("order_id", req.GetOrderId()),
					zap.String("status", existing.Status),
				)
				return nil, errno.ErrOrderStateConflict
			}
		}
		record, bizErr := s.createPendingActionRecord(ctx, actionTypeCancel, requestID, req.GetOrderId(), req.GetUserId())
		if bizErr != nil {
			return nil, bizErr
		}
		if record != nil && record.Status != actionStatusPending {
			if record.Status == "succeeded" {
				logx.L(ctx).Info("cancel order reused succeeded action record after create attempt",
					zap.String("request_id", requestID),
					zap.Int64("order_id", record.OrderID),
				)
				return s.loadOrderByActionRecord(ctx, record)
			}
			logx.L(ctx).Warn("cancel order blocked by non-pending action record after create attempt",
				zap.String("request_id", requestID),
				zap.Int64("order_id", req.GetOrderId()),
				zap.String("status", record.Status),
			)
			return nil, errno.ErrOrderIdempotencyBusy
		}
	}

	order, err := s.repo.GetByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Warn("cancel order load order failed",
			zap.Error(err),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionTypeCancel, requestID, bizErr)
		return nil, bizErr
	}
	switch order.Status {
	case OrderStatusCancelled, OrderStatusClosed:
		logx.L(ctx).Info("cancel order treated as idempotent success",
			zap.Int64("order_id", order.OrderID),
			zap.Int64("user_id", order.UserID),
			zap.Int32("status", order.Status),
		)
		s.markActionSucceeded(ctx, actionTypeCancel, requestID, order.OrderID)
		return toRPCOrder(order), nil
	case OrderStatusPaid:
		logx.L(ctx).Warn("cancel order rejected by paid status",
			zap.Int64("order_id", order.OrderID),
			zap.Int64("user_id", order.UserID),
			zap.Int32("status", order.Status),
		)
		s.markActionFailed(ctx, actionTypeCancel, requestID, errno.ErrOrderPaidImmutable)
		return nil, errno.ErrOrderPaidImmutable
	case OrderStatusPending, OrderStatusReserved:
	default:
		logx.L(ctx).Warn("cancel order rejected by unsupported status",
			zap.Int64("order_id", order.OrderID),
			zap.Int64("user_id", order.UserID),
			zap.Int32("status", order.Status),
		)
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
		logx.L(ctx).Error("release inventory for cancel order failed",
			zap.Error(releaseErr),
			zap.Int64("order_id", order.OrderID),
			zap.Int64("user_id", order.UserID),
			zap.String("biz_id", s.reserveBizID(order.OrderID)),
		)
		s.markActionFailed(ctx, actionTypeCancel, requestID, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if releaseResp.Code != 0 {
		bizErr := mapInventoryRPCError(releaseResp.Code)
		logx.L(ctx).Warn("release inventory for cancel order returned business error",
			zap.Int64("order_id", order.OrderID),
			zap.Int64("user_id", order.UserID),
			zap.Int32("inventory_rpc_code", releaseResp.Code),
			zap.String("inventory_rpc_message", releaseResp.Message),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
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
		logx.L(ctx).Info("cancel order completed",
			zap.Int64("order_id", updated.OrderID),
			zap.Int64("user_id", updated.UserID),
			zap.Int32("status", updated.Status),
			zap.String("cancel_reason", updated.CancelReason),
		)
		s.markActionSucceeded(ctx, actionTypeCancel, requestID, updated.OrderID)
		return toRPCOrder(updated), nil
	}
	if !errors.Is(updateErr, repository.ErrOrderStateConflict) {
		bizErr := mapRepositoryError(updateErr)
		logx.L(ctx).Error("cancel order transition failed",
			zap.Error(updateErr),
			zap.Int64("order_id", order.OrderID),
			zap.Int64("user_id", order.UserID),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionTypeCancel, requestID, bizErr)
		return nil, bizErr
	}

	current, currentErr := s.repo.GetByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if currentErr != nil {
		bizErr := mapRepositoryError(currentErr)
		logx.L(ctx).Error("cancel order reload after state conflict failed",
			zap.Error(currentErr),
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
	}
	switch current.Status {
	case OrderStatusCancelled, OrderStatusClosed:
		logx.L(ctx).Info("cancel order reconciled to terminal success",
			zap.Int64("order_id", current.OrderID),
			zap.Int64("user_id", current.UserID),
			zap.Int32("status", current.Status),
		)
		s.markActionSucceeded(ctx, actionTypeCancel, requestID, current.OrderID)
		return toRPCOrder(current), nil
	case OrderStatusPaid:
		logx.L(ctx).Warn("cancel order reconciled to paid status",
			zap.Int64("order_id", current.OrderID),
			zap.Int64("user_id", current.UserID),
			zap.Int32("status", current.Status),
		)
		s.markActionFailed(ctx, actionTypeCancel, requestID, errno.ErrOrderPaidImmutable)
		return nil, errno.ErrOrderPaidImmutable
	default:
		logx.L(ctx).Warn("cancel order reconciled to unsupported status",
			zap.Int64("order_id", current.OrderID),
			zap.Int64("user_id", current.UserID),
			zap.Int32("status", current.Status),
		)
		s.markActionFailed(ctx, actionTypeCancel, requestID, errno.ErrOrderStateConflict)
		return nil, errno.ErrOrderStateConflict
	}
}
