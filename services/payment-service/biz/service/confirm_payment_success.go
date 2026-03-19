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
		return nil, bizErr
	}
	tradeNo := strings.TrimSpace(req.GetPaymentTradeNo())
	if tradeNo == "" {
		tradeNo = "mock-" + buildOrderPaymentID(req.GetPaymentId())
	}
	actionKey := confirmActionKey(req)

	existing, bizErr := s.findActionRecord(ctx, actionTypeConfirm, actionKey)
	if bizErr != nil {
		return nil, bizErr
	}
	if existing != nil {
		switch existing.Status {
		case "succeeded":
			return s.loadPaymentByActionRecord(ctx, existing)
		case actionStatusPending:
			return nil, errno.ErrPaymentIdempotencyBusy
		default:
			return nil, errno.ErrPaymentStateConflict
		}
	}
	record, bizErr := s.createPendingActionRecord(ctx, actionTypeConfirm, actionKey, req.GetPaymentId(), 0)
	if bizErr != nil {
		return nil, bizErr
	}
	if record != nil && record.Status != actionStatusPending {
		if record.Status == "succeeded" {
			return s.loadPaymentByActionRecord(ctx, record)
		}
		return nil, errno.ErrPaymentIdempotencyBusy
	}

	payment, err := s.repo.GetByPaymentID(ctx, req.GetPaymentId())
	if err != nil {
		bizErr = mapRepositoryError(err)
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, bizErr)
		return nil, bizErr
	}
	if payment.Status == PaymentStatusSucceeded {
		if paymentConflict(payment.PaymentMethod, method) || paymentConflict(payment.PaymentTradeNo, tradeNo) {
			s.markActionFailed(ctx, actionTypeConfirm, actionKey, errno.ErrPaymentConflict)
			return nil, errno.ErrPaymentConflict
		}
		s.markActionSucceeded(ctx, actionTypeConfirm, actionKey, payment.PaymentID, payment.OrderID)
		return toRPCPayment(payment), nil
	}
	if payment.Status != PaymentStatusPending {
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, errno.ErrPaymentStateConflict)
		return nil, errno.ErrPaymentStateConflict
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
		s.markActionFailed(ctx, actionTypeConfirm, actionKey, bizErr)
		return nil, bizErr
	}
	s.markActionSucceeded(ctx, actionTypeConfirm, actionKey, updated.PaymentID, updated.OrderID)
	return toRPCPayment(updated), nil
}
