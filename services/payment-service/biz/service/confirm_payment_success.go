package service

import (
	"context"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	orderpb "meshcart/kitex_gen/meshcart/order"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	"meshcart/services/payment-service/biz/errno"
	"meshcart/services/payment-service/biz/repository"

	"go.uber.org/zap"
)

func (s *PaymentService) ConfirmPaymentSuccess(ctx context.Context, req *paymentpb.ConfirmPaymentSuccessRequest) (*paymentpb.Payment, *common.BizError) {
	if req == nil || req.GetPaymentId() <= 0 {
		return nil, common.ErrInvalidParam
	}
	method := normalizePaymentMethod(req.GetPaymentMethod())
	if bizErr := validatePaymentMethod(method); bizErr != nil {
		logx.L(ctx).Warn("confirm payment success rejected by unsupported method", zap.Int64("payment_id", req.GetPaymentId()), zap.String("payment_method", req.GetPaymentMethod()))
		return nil, bizErr
	}
	tradeNo := strings.TrimSpace(req.GetPaymentTradeNo())
	if tradeNo == "" {
		tradeNo = "mock-" + buildOrderPaymentID(req.GetPaymentId())
	}
	actionKey := confirmActionKey(req)
	logx.L(ctx).Info("confirm payment success start",
		zap.Int64("payment_id", req.GetPaymentId()),
		zap.String("payment_method", method),
		zap.String("payment_trade_no", tradeNo),
		zap.String("action_key", actionKey),
		zap.Int64("paid_at", req.GetPaidAt()),
	)

	existing, bizErr := s.findActionRecord(ctx, actionTypeConfirm, actionKey)
	if bizErr != nil {
		return nil, bizErr
	}
	if existing != nil {
		logx.L(ctx).Info("confirm payment success found existing action record",
			zap.String("action_type", existing.ActionType),
			zap.String("action_key", existing.ActionKey),
			zap.String("status", existing.Status),
			zap.Int64("payment_id", existing.PaymentID),
			zap.Int64("order_id", existing.OrderID),
		)
		switch existing.Status {
		case "succeeded":
			return s.loadPaymentByActionRecord(ctx, existing)
		case actionStatusPending:
			logx.L(ctx).Warn("confirm payment success blocked by pending action record", zap.String("action_key", actionKey), zap.Int64("payment_id", req.GetPaymentId()))
			return nil, errno.ErrPaymentIdempotencyBusy
		default:
			logx.L(ctx).Warn("confirm payment success will retry after failed action record", zap.String("action_key", actionKey), zap.Int64("payment_id", req.GetPaymentId()))
		}
	}
	record, bizErr := s.createPendingActionRecord(ctx, actionTypeConfirm, actionKey, req.GetPaymentId(), 0)
	if bizErr != nil {
		return nil, bizErr
	}
	if record != nil && record.Status != actionStatusPending {
		logx.L(ctx).Info("confirm payment success reused non-pending action record",
			zap.String("action_key", actionKey),
			zap.String("status", record.Status),
			zap.Int64("payment_id", record.PaymentID),
			zap.Int64("order_id", record.OrderID),
		)
		if record.Status == "succeeded" {
			return s.loadPaymentByActionRecord(ctx, record)
		}
		if record.Status == "failed" {
			logx.L(ctx).Warn("confirm payment success will retry with previously failed action record", zap.String("action_key", actionKey), zap.Int64("payment_id", req.GetPaymentId()))
		} else {
			return nil, errno.ErrPaymentIdempotencyBusy
		}
	}

	payment, err := s.repo.GetByPaymentID(ctx, req.GetPaymentId())
	if err != nil {
		bizErr = mapRepositoryError(err)
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, bizErr)
		return nil, bizErr
	}
	logx.L(ctx).Info("confirm payment success loaded payment",
		zap.Int64("payment_id", payment.PaymentID),
		zap.Int64("order_id", payment.OrderID),
		zap.Int64("user_id", payment.UserID),
		zap.Int32("status", payment.Status),
		zap.String("payment_method", payment.PaymentMethod),
		zap.String("payment_trade_no", payment.PaymentTradeNo),
	)
	if payment.Status == PaymentStatusSucceeded {
		if paymentConflict(payment.PaymentMethod, method) || paymentConflict(payment.PaymentTradeNo, tradeNo) {
			logx.L(ctx).Warn("confirm payment success conflict on succeeded payment",
				zap.Int64("payment_id", payment.PaymentID),
				zap.String("stored_method", payment.PaymentMethod),
				zap.String("incoming_method", method),
				zap.String("stored_trade_no", payment.PaymentTradeNo),
				zap.String("incoming_trade_no", tradeNo),
			)
			s.markActionFailed(ctx, actionTypeConfirm, actionKey, errno.ErrPaymentConflict)
			return nil, errno.ErrPaymentConflict
		}
		logx.L(ctx).Info("confirm payment success treated as idempotent success", zap.Int64("payment_id", payment.PaymentID), zap.Int64("order_id", payment.OrderID))
		s.markActionSucceeded(ctx, actionTypeConfirm, actionKey, payment.PaymentID, payment.OrderID)
		return toRPCPayment(payment), nil
	}
	if payment.Status == PaymentStatusClosed {
		logx.L(ctx).Warn("confirm payment success rejected by closed payment", zap.Int64("payment_id", payment.PaymentID), zap.Int64("order_id", payment.OrderID))
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, errno.ErrPaymentStateConflict)
		return nil, errno.ErrPaymentStateConflict
	}
	if payment.Status != PaymentStatusPending {
		logx.L(ctx).Warn("confirm payment success rejected by payment status", zap.Int64("payment_id", payment.PaymentID), zap.Int32("status", payment.Status), zap.Int64("order_id", payment.OrderID))
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, errno.ErrPaymentStateConflict)
		return nil, errno.ErrPaymentStateConflict
	}
	if s.isPaymentExpired(payment) {
		logx.L(ctx).Warn("confirm payment success rejected by payment expiry",
			zap.Int64("payment_id", payment.PaymentID),
			zap.Int64("order_id", payment.OrderID),
			zap.Time("expire_at", payment.ExpireAt),
			zap.Time("now", s.now()),
		)
		closedAt := s.now()
		if _, closeErr := s.repo.TransitionStatus(ctx, repository.PaymentTransition{
			PaymentID:    payment.PaymentID,
			FromStatuses: []int32{PaymentStatusPending},
			ToStatus:     PaymentStatusClosed,
			ClosedAt:     &closedAt,
			FailReason:   "payment_expired",
			ActionType:   actionTypeClose,
			Reason:       "payment_expired",
			ExternalRef:  "system",
		}); closeErr != nil && closeErr != repository.ErrPaymentStateConflict {
			logx.L(ctx).Warn("confirm payment success close expired payment failed", zap.Error(closeErr), zap.Int64("payment_id", payment.PaymentID))
		}
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, errno.ErrPaymentExpired)
		return nil, errno.ErrPaymentExpired
	}

	orderReq := &orderpb.ConfirmOrderPaidRequest{
		OrderId:        payment.OrderID,
		PaymentId:      buildOrderPaymentID(payment.PaymentID),
		RequestId:      stringPointer(strings.TrimSpace(req.GetRequestId())),
		PaymentMethod:  stringPointer(method),
		PaymentTradeNo: stringPointer(tradeNo),
		PaidAt:         paidAtUnixPointer(req.GetPaidAt()),
	}
	orderResp, orderErr := s.orderClient.ConfirmOrderPaid(ctx, orderReq)
	if orderErr != nil {
		logx.L(ctx).Error("confirm order paid failed", zap.Error(orderErr), zap.Int64("payment_id", payment.PaymentID), zap.Int64("order_id", payment.OrderID))
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if orderResp.Code != 0 {
		bizErr = mapOrderRPCError(orderResp.Code)
		logx.L(ctx).Warn("confirm payment success blocked by order rpc business error",
			zap.Int64("payment_id", payment.PaymentID),
			zap.Int64("order_id", payment.OrderID),
			zap.Int32("order_rpc_code", orderResp.Code),
			zap.String("order_rpc_message", orderResp.Message),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, bizErr)
		return nil, bizErr
	}

	succeededAt := s.now()
	if req.GetPaidAt() > 0 {
		succeededAt = time.Unix(req.GetPaidAt(), 0)
	}
	updated, updateErr := s.repo.TransitionStatus(ctx, repository.PaymentTransition{
		PaymentID:      payment.PaymentID,
		FromStatuses:   []int32{PaymentStatusPending},
		ToStatus:       PaymentStatusSucceeded,
		PaymentMethod:  method,
		PaymentTradeNo: tradeNo,
		SucceededAt:    &succeededAt,
		ActionType:     actionTypeConfirm,
		Reason:         "payment_confirmed",
		ExternalRef:    tradeNo,
	})
	if updateErr != nil {
		bizErr = mapRepositoryError(updateErr)
		logx.L(ctx).Error("confirm payment success update payment status failed",
			zap.Error(updateErr),
			zap.Int64("payment_id", payment.PaymentID),
			zap.Int64("order_id", payment.OrderID),
			zap.String("payment_trade_no", tradeNo),
		)
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, bizErr)
		return nil, bizErr
	}
	logx.L(ctx).Info("confirm payment success completed",
		zap.Int64("payment_id", updated.PaymentID),
		zap.Int64("order_id", updated.OrderID),
		zap.Int32("status", updated.Status),
		zap.String("payment_trade_no", updated.PaymentTradeNo),
	)
	s.markActionSucceeded(ctx, actionTypeConfirm, actionKey, updated.PaymentID, updated.OrderID)
	return toRPCPayment(updated), nil
}
