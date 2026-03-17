package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/errno"
	"meshcart/services/order-service/biz/repository"

	"go.uber.org/zap"
)

func (s *OrderService) ConfirmOrderPaid(ctx context.Context, req *orderpb.ConfirmOrderPaidRequest) (*orderpb.Order, *common.BizError) {
	if req == nil || req.GetOrderId() <= 0 || strings.TrimSpace(req.GetPaymentId()) == "" {
		return nil, common.ErrInvalidParam
	}

	actionKey := strings.TrimSpace(req.GetPaymentId())
	if requestID := strings.TrimSpace(req.GetRequestId()); requestID != "" {
		actionKey = requestID
	}

	existing, bizErr := s.findActionRecord(ctx, actionTypePay, actionKey)
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
	record, bizErr := s.createPendingActionRecord(ctx, actionTypePay, actionKey, req.GetOrderId(), 0)
	if bizErr != nil {
		return nil, bizErr
	}
	if record != nil && record.Status != actionStatusPending {
		if record.Status == "succeeded" {
			return s.loadOrderByActionRecord(ctx, record)
		}
		return nil, errno.ErrOrderIdempotencyBusy
	}

	order, err := s.repo.GetByID(ctx, req.GetOrderId())
	if err != nil {
		bizErr = mapRepositoryError(err)
		s.markActionFailed(ctx, actionTypePay, actionKey, bizErr)
		return nil, bizErr
	}
	if order.Status == OrderStatusPaid {
		if order.PaymentID == strings.TrimSpace(req.GetPaymentId()) {
			s.markActionSucceeded(ctx, actionTypePay, actionKey, order.OrderID)
			return toRPCOrder(order), nil
		}
		s.markActionFailed(ctx, actionTypePay, actionKey, errno.ErrOrderPaymentConflict)
		return nil, errno.ErrOrderPaymentConflict
	}
	if order.Status == OrderStatusClosed || order.Status == OrderStatusCancelled {
		s.markActionFailed(ctx, actionTypePay, actionKey, errno.ErrOrderStateConflict)
		return nil, errno.ErrOrderStateConflict
	}
	if order.Status != OrderStatusReserved {
		s.markActionFailed(ctx, actionTypePay, actionKey, errno.ErrOrderStateConflict)
		return nil, errno.ErrOrderStateConflict
	}

	confirmResp, confirmErr := s.inventoryClient.ConfirmDeductReservedSkuStocks(ctx, &inventory.ConfirmDeductReservedSkuStocksRequest{
		BizType: orderReserveBizType,
		BizId:   s.reserveBizID(order.OrderID),
		Items:   buildReleaseItems(order),
	})
	if confirmErr != nil {
		logx.L(ctx).Error("confirm deduct inventory failed", zap.Error(confirmErr), zap.Int64("order_id", order.OrderID), zap.String("payment_id", req.GetPaymentId()))
		s.markActionFailed(ctx, actionTypePay, actionKey, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if confirmResp.Code != 0 {
		bizErr = mapInventoryRPCError(confirmResp.Code)
		s.markActionFailed(ctx, actionTypePay, actionKey, bizErr)
		return nil, bizErr
	}

	paidAt := s.now()
	updated, updateErr := s.repo.TransitionStatus(ctx, repository.OrderTransition{
		OrderID:      order.OrderID,
		FromStatuses: []int32{OrderStatusReserved},
		ToStatus:     OrderStatusPaid,
		CancelReason: "",
		PaymentID:    strings.TrimSpace(req.GetPaymentId()),
		PaidAt:       &paidAt,
		ActionType:   actionTypePay,
		Reason:       "payment_confirmed",
		ExternalRef:  strings.TrimSpace(req.GetPaymentId()),
	})
	if updateErr != nil {
		bizErr = mapRepositoryError(updateErr)
		s.markActionFailed(ctx, actionTypePay, actionKey, bizErr)
		return nil, bizErr
	}
	s.markActionSucceeded(ctx, actionTypePay, actionKey, updated.OrderID)
	return toRPCOrder(updated), nil
}
