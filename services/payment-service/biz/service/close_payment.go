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
		paymentID := int64(0)
		userID := int64(0)
		if req != nil {
			paymentID = req.GetPaymentId()
			userID = req.GetUserId()
		}
		logx.L(ctx).Warn("close payment rejected by invalid request",
			zap.Int64("payment_id", paymentID),
			zap.Int64("user_id", userID),
		)
		return nil, common.ErrInvalidParam
	}

	actionKey := closeActionKey(req.GetPaymentId(), req.GetRequestId())
	logx.L(ctx).Info("close payment start",
		zap.Int64("payment_id", req.GetPaymentId()),
		zap.Int64("user_id", req.GetUserId()),
		zap.String("action_key", actionKey),
		zap.String("reason", strings.TrimSpace(req.GetReason())),
	)
	if existing, bizErr := s.findActionRecord(ctx, actionTypeClose, actionKey); bizErr != nil {
		return nil, bizErr
	} else if existing != nil {
		switch existing.Status {
		case "succeeded":
			logx.L(ctx).Info("close payment hit succeeded action record",
				zap.String("action_key", actionKey),
				zap.Int64("payment_id", existing.PaymentID),
				zap.Int64("order_id", existing.OrderID),
			)
			return s.loadPaymentByActionRecord(ctx, existing)
		case actionStatusPending:
			logx.L(ctx).Warn("close payment blocked by pending action record",
				zap.String("action_key", actionKey),
				zap.Int64("payment_id", req.GetPaymentId()),
			)
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
		logx.L(ctx).Warn("close payment load payment failed",
			zap.Int64("payment_id", req.GetPaymentId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.Error(err),
			zap.Int32("mapped_code", bizErr.Code),
		)
		s.markActionFailed(ctx, actionTypeClose, actionKey, bizErr)
		return nil, bizErr
	}
	if payment.Status == PaymentStatusSucceeded {
		logx.L(ctx).Warn("close payment rejected by succeeded status",
			zap.Int64("payment_id", payment.PaymentID),
			zap.Int64("order_id", payment.OrderID),
			zap.Int32("status", payment.Status),
		)
		s.markActionFailed(ctx, actionTypeClose, actionKey, errno.ErrPaymentStateConflict)
		return nil, errno.ErrPaymentStateConflict
	}
	if payment.Status == PaymentStatusClosed {
		logx.L(ctx).Info("close payment treated as idempotent success",
			zap.Int64("payment_id", payment.PaymentID),
			zap.Int64("order_id", payment.OrderID),
			zap.Int32("status", payment.Status),
		)
		s.markActionSucceeded(ctx, actionTypeClose, actionKey, payment.PaymentID, payment.OrderID)
		return toRPCPayment(payment), nil
	}
	if payment.Status != PaymentStatusPending {
		logx.L(ctx).Warn("close payment rejected by payment status",
			zap.Int64("payment_id", payment.PaymentID),
			zap.Int64("order_id", payment.OrderID),
			zap.Int32("status", payment.Status),
		)
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
		logx.L(ctx).Error("close payment transition failed",
			zap.Error(updateErr),
			zap.Int64("payment_id", payment.PaymentID),
			zap.Int64("order_id", payment.OrderID),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int32("mapped_code", bizErr.Code),
		)
		s.markActionFailed(ctx, actionTypeClose, actionKey, bizErr)
		return nil, bizErr
	}
	logx.L(ctx).Info("close payment completed",
		zap.Int64("payment_id", updated.PaymentID),
		zap.Int64("order_id", updated.OrderID),
		zap.Int32("status", updated.Status),
		zap.String("fail_reason", updated.FailReason),
	)
	s.markActionSucceeded(ctx, actionTypeClose, actionKey, updated.PaymentID, updated.OrderID)
	return toRPCPayment(updated), nil
}
