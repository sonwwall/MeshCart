package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	orderpb "meshcart/kitex_gen/meshcart/order"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	"meshcart/services/payment-service/biz/errno"
	"meshcart/services/payment-service/biz/repository"
	dalmodel "meshcart/services/payment-service/dal/model"

	"go.uber.org/zap"
)

const (
	paymentMethodMock   = "mock"
	actionStatusPending = "pending"
	actionTypeCreate    = "create"
	actionTypeConfirm   = "confirm_success"
	actionTypeClose     = "close"
	paymentTTL          = 15 * time.Minute
)

func (s *PaymentService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

func normalizePaymentMethod(method string) string {
	return strings.ToLower(strings.TrimSpace(method))
}

func validatePaymentMethod(method string) *common.BizError {
	if normalizePaymentMethod(method) != paymentMethodMock {
		return errno.ErrUnsupportedPaymentMethod
	}
	return nil
}

func confirmActionKey(req *paymentpb.ConfirmPaymentSuccessRequest) string {
	if req == nil {
		return ""
	}
	if requestID := strings.TrimSpace(req.GetRequestId()); requestID != "" {
		return requestID
	}
	return strconv.FormatInt(req.GetPaymentId(), 10)
}

func paymentConflict(existing, incoming string) bool {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	return existing != "" && incoming != "" && existing != incoming
}

func closeActionKey(paymentID int64, requestID string) string {
	if trimmed := strings.TrimSpace(requestID); trimmed != "" {
		return trimmed
	}
	return strconv.FormatInt(paymentID, 10)
}

func (s *PaymentService) findActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.PaymentActionRecord, *common.BizError) {
	record, err := s.repo.GetActionRecord(ctx, actionType, actionKey)
	if err == nil {
		logx.L(ctx).Info("payment action record found", zap.String("action_type", actionType), zap.String("action_key", actionKey), zap.String("status", record.Status), zap.Int64("payment_id", record.PaymentID), zap.Int64("order_id", record.OrderID))
		return record, nil
	}
	if err == repository.ErrActionRecordNotFound {
		logx.L(ctx).Debug("payment action record not found", zap.String("action_type", actionType), zap.String("action_key", actionKey))
		return nil, nil
	}
	logx.L(ctx).Error("get payment action record failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey))
	return nil, common.ErrInternalError
}

func (s *PaymentService) createPendingActionRecord(ctx context.Context, actionType, actionKey string, paymentID, orderID int64) (*dalmodel.PaymentActionRecord, *common.BizError) {
	if strings.TrimSpace(actionKey) == "" {
		return nil, nil
	}
	record := &dalmodel.PaymentActionRecord{
		ID:         s.node.Generate().Int64(),
		ActionType: actionType,
		ActionKey:  actionKey,
		PaymentID:  paymentID,
		OrderID:    orderID,
		Status:     actionStatusPending,
	}
	if err := s.repo.CreateActionRecord(ctx, record); err != nil {
		if err == repository.ErrActionRecordExists {
			logx.L(ctx).Warn("payment action record already exists", zap.String("action_type", actionType), zap.String("action_key", actionKey), zap.Int64("payment_id", paymentID), zap.Int64("order_id", orderID))
			existing, bizErr := s.findActionRecord(ctx, actionType, actionKey)
			if bizErr != nil {
				return nil, bizErr
			}
			if existing != nil {
				return existing, nil
			}
		}
		logx.L(ctx).Error("create payment action record failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey))
		return nil, common.ErrInternalError
	}
	logx.L(ctx).Info("payment action record created", zap.String("action_type", actionType), zap.String("action_key", actionKey), zap.Int64("payment_id", paymentID), zap.Int64("order_id", orderID))
	return record, nil
}

func (s *PaymentService) markActionSucceeded(ctx context.Context, actionType, actionKey string, paymentID, orderID int64) {
	if strings.TrimSpace(actionKey) == "" {
		return
	}
	if err := s.repo.MarkActionRecordSucceeded(ctx, actionType, actionKey, paymentID, orderID); err != nil {
		logx.L(ctx).Error("mark payment action succeeded failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey), zap.Int64("payment_id", paymentID), zap.Int64("order_id", orderID))
	}
}

func (s *PaymentService) markActionFailed(ctx context.Context, actionType, actionKey string, bizErr *common.BizError) {
	if strings.TrimSpace(actionKey) == "" {
		return
	}
	message := ""
	if bizErr != nil {
		message = bizErr.Msg
	}
	if err := s.repo.MarkActionRecordFailed(ctx, actionType, actionKey, message); err != nil {
		logx.L(ctx).Error("mark payment action failed failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey))
	}
}

func (s *PaymentService) loadPaymentByActionRecord(ctx context.Context, record *dalmodel.PaymentActionRecord) (*paymentpb.Payment, *common.BizError) {
	if record == nil || record.PaymentID <= 0 {
		return nil, errno.ErrPaymentIdempotencyBusy
	}
	payment, err := s.repo.GetByPaymentID(ctx, record.PaymentID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCPayment(payment), nil
}

func toRPCPayment(payment *dalmodel.Payment) *paymentpb.Payment {
	if payment == nil {
		return nil
	}
	succeededAt := int64(0)
	if payment.SucceededAt != nil && !payment.SucceededAt.IsZero() {
		succeededAt = payment.SucceededAt.Unix()
	}
	closedAt := int64(0)
	if payment.ClosedAt != nil && !payment.ClosedAt.IsZero() {
		closedAt = payment.ClosedAt.Unix()
	}
	return &paymentpb.Payment{
		PaymentId:      payment.PaymentID,
		OrderId:        payment.OrderID,
		UserId:         payment.UserID,
		Status:         payment.Status,
		PaymentMethod:  payment.PaymentMethod,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
		PaymentTradeNo: payment.PaymentTradeNo,
		ExpireAt:       payment.ExpireAt.Unix(),
		SucceededAt:    succeededAt,
		ClosedAt:       closedAt,
		FailReason:     payment.FailReason,
	}
}

func toRPCPayments(payments []*dalmodel.Payment) []*paymentpb.Payment {
	result := make([]*paymentpb.Payment, 0, len(payments))
	for _, payment := range payments {
		result = append(result, toRPCPayment(payment))
	}
	return result
}

func mapRepositoryError(err error) *common.BizError {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrPaymentNotFound):
		return errno.ErrPaymentNotFound
	case errors.Is(err, repository.ErrInvalidPayment):
		return errno.ErrInvalidPaymentData
	case errors.Is(err, repository.ErrPaymentStateConflict):
		return errno.ErrPaymentStateConflict
	default:
		return common.ErrInternalError
	}
}

func mapOrderRPCError(code int32) *common.BizError {
	switch code {
	case 2040001:
		return errno.ErrPaymentOrderNotFound
	case 2040006, 2040007, 2040008:
		return errno.ErrPaymentOrderStateConflict
	default:
		return common.ErrServiceUnavailable
	}
}

func buildOrderPaymentID(paymentID int64) string {
	return fmt.Sprintf("%d", paymentID)
}

func paidAtUnixPointer(ts int64) *int64 {
	if ts <= 0 {
		return nil
	}
	value := ts
	return &value
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := value
	return &v
}

func mapOrderGetFailure(code int32) *common.BizError {
	if code == 2040001 {
		return errno.ErrPaymentOrderNotFound
	}
	return common.ErrServiceUnavailable
}

func (s *PaymentService) validateOrderForPayment(order *orderpb.Order) *common.BizError {
	if order == nil {
		return common.ErrServiceUnavailable
	}
	if order.GetExpireAt() > 0 && !s.now().Before(time.Unix(order.GetExpireAt(), 0)) {
		return errno.ErrPaymentOrderStateConflict
	}
	if order.GetStatus() == 3 {
		return errno.ErrPaymentOrderStateConflict
	}
	if order.GetStatus() != 2 {
		return errno.ErrPaymentOrderStateConflict
	}
	return nil
}

func (s *PaymentService) calculatePaymentExpireAt(order *orderpb.Order) time.Time {
	expireAt := s.now().Add(paymentTTL)
	if order != nil && order.GetExpireAt() > 0 {
		orderExpireAt := time.Unix(order.GetExpireAt(), 0)
		if orderExpireAt.Before(expireAt) {
			expireAt = orderExpireAt
		}
	}
	return expireAt
}

func (s *PaymentService) isPaymentExpired(payment *dalmodel.Payment) bool {
	if payment == nil || payment.ExpireAt.IsZero() {
		return false
	}
	return !s.now().Before(payment.ExpireAt)
}
