package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	"meshcart/services/payment-service/biz/errno"
	"meshcart/services/payment-service/biz/repository"

	"go.uber.org/zap"
)

func (s *PaymentService) ClosePayment(ctx context.Context, req *paymentpb.ClosePaymentRequest) (*paymentpb.Payment, *common.BizError) {
	if req == nil || req.GetPaymentId() <= 0 || req.GetUserId() <= 0 {
		return nil, common.ErrInvalidParam
	}

	actionKey := closeActionKey(req.GetPaymentId(), req.GetRequestId())
	if existing, bizErr := s.findActionRecord(ctx, actionTypeClose, actionKey); bizErr != nil {
		return nil, bizErr
	} else if existing != nil {
		switch existing.Status {
		case "succeeded":
			return s.loadPaymentByActionRecord(ctx, existing)
		case actionStatusPending:
			return nil, errno.ErrPaymentIdempotencyBusy
		}
	}
	record, bizErr := s.createPendingActionRecord(ctx, actionTypeClose, actionKey, req.GetPaymentId(), 0)
	if bizErr != nil {
		return nil, bizErr
	}
	if record != nil && record.Status != actionStatusPending {
		if record.Status == "succeeded" {
			return s.loadPaymentByActionRecord(ctx, record)
		}
		return nil, errno.ErrPaymentIdempotencyBusy
	}

	payment, err := s.repo.GetByPaymentIDUser(ctx, req.GetPaymentId(), req.GetUserId())
	if err != nil {
		bizErr = mapRepositoryError(err)
		s.markActionFailed(ctx, actionTypeClose, actionKey, bizErr)
		return nil, bizErr
	}
	if payment.Status == PaymentStatusSucceeded {
		s.markActionFailed(ctx, actionTypeClose, actionKey, errno.ErrPaymentStateConflict)
		return nil, errno.ErrPaymentStateConflict
	}
	if payment.Status == PaymentStatusClosed {
		s.markActionSucceeded(ctx, actionTypeClose, actionKey, payment.PaymentID, payment.OrderID)
		return toRPCPayment(payment), nil
	}
	if payment.Status != PaymentStatusPending {
		s.markActionFailed(ctx, actionTypeClose, actionKey, errno.ErrPaymentStateConflict)
		return nil, errno.ErrPaymentStateConflict
	}

	closedAt := s.now()
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" {
		reason = "payment_closed"
	}
	updated, updateErr := s.repo.TransitionStatus(ctx, repository.PaymentTransition{
		PaymentID:    payment.PaymentID,
		FromStatuses: []int32{PaymentStatusPending},
		ToStatus:     PaymentStatusClosed,
		ClosedAt:     &closedAt,
		FailReason:   reason,
		ActionType:   actionTypeClose,
		Reason:       reason,
		ExternalRef:  "user",
	})
	if updateErr != nil {
		bizErr = mapRepositoryError(updateErr)
		logx.L(ctx).Warn("close payment transition failed", zap.Error(updateErr), zap.Int64("payment_id", payment.PaymentID), zap.Int64("user_id", req.GetUserId()))
		s.markActionFailed(ctx, actionTypeClose, actionKey, bizErr)
		return nil, bizErr
	}
	s.markActionSucceeded(ctx, actionTypeClose, actionKey, updated.PaymentID, updated.OrderID)
	return toRPCPayment(updated), nil
}
