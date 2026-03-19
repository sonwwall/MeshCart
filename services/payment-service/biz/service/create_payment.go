package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
	"meshcart/services/payment-service/biz/errno"
	"meshcart/services/payment-service/biz/repository"
	dalmodel "meshcart/services/payment-service/dal/model"

	"go.uber.org/zap"
)

func (s *PaymentService) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*paymentpb.Payment, *common.BizError) {
	if req == nil || req.GetOrderId() <= 0 || req.GetUserId() <= 0 {
		return nil, common.ErrInvalidParam
	}
	method := normalizePaymentMethod(req.GetPaymentMethod())
	if bizErr := validatePaymentMethod(method); bizErr != nil {
		return nil, bizErr
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
				return s.loadPaymentByActionRecord(ctx, existing)
			case actionStatusPending:
				return nil, errno.ErrPaymentIdempotencyBusy
			default:
				return nil, errno.ErrPaymentStateConflict
			}
		}
		record, bizErr := s.createPendingActionRecord(ctx, actionTypeCreate, requestID, 0, req.GetOrderId())
		if bizErr != nil {
			return nil, bizErr
		}
		if record != nil && record.Status != actionStatusPending {
			if record.Status == "succeeded" {
				return s.loadPaymentByActionRecord(ctx, record)
			}
			return nil, errno.ErrPaymentIdempotencyBusy
		}
	}

	existingPayments, err := s.repo.ListByOrderID(ctx, req.GetOrderId(), req.GetUserId())
	if err != nil {
		bizErr := mapRepositoryError(err)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	for _, existingPayment := range existingPayments {
		switch existingPayment.Status {
		case PaymentStatusSucceeded:
			s.markActionSucceeded(ctx, actionTypeCreate, requestID, existingPayment.PaymentID, existingPayment.OrderID)
			return toRPCPayment(existingPayment), nil
		case PaymentStatusPending:
			if s.isPaymentExpired(existingPayment) {
				closedAt := s.now()
				if _, closeErr := s.repo.TransitionStatus(ctx, repository.PaymentTransition{
					PaymentID:    existingPayment.PaymentID,
					FromStatuses: []int32{PaymentStatusPending},
					ToStatus:     PaymentStatusClosed,
					ClosedAt:     &closedAt,
					FailReason:   "payment_expired",
					ActionType:   actionTypeClose,
					Reason:       "payment_expired",
					ExternalRef:  "system",
				}); closeErr != nil && closeErr != repository.ErrPaymentStateConflict {
					logx.L(ctx).Warn("close expired pending payment before create failed", zap.Error(closeErr), zap.Int64("payment_id", existingPayment.PaymentID), zap.Int64("order_id", existingPayment.OrderID))
				}
				continue
			}
			s.markActionSucceeded(ctx, actionTypeCreate, requestID, existingPayment.PaymentID, existingPayment.OrderID)
			return toRPCPayment(existingPayment), nil
		}
	}

	orderResp, rpcErr := s.orderClient.GetOrder(ctx, req.GetUserId(), req.GetOrderId())
	if rpcErr != nil {
		logx.L(ctx).Error("get order for payment failed", zap.Error(rpcErr), zap.Int64("order_id", req.GetOrderId()), zap.Int64("user_id", req.GetUserId()))
		s.markActionFailed(ctx, actionTypeCreate, requestID, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if orderResp.Code != 0 {
		bizErr := mapOrderGetFailure(orderResp.Code)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	if bizErr := s.validateOrderForPayment(orderResp.Order); bizErr != nil {
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	expireAt := s.calculatePaymentExpireAt(orderResp.Order)

	model := &dalmodel.Payment{
		PaymentID:     s.node.Generate().Int64(),
		OrderID:       req.GetOrderId(),
		UserID:        req.GetUserId(),
		Status:        PaymentStatusPending,
		PaymentMethod: method,
		Amount:        orderResp.Order.GetPayAmount(),
		Currency:      "CNY",
		RequestID:     requestID,
		ExpireAt:      expireAt,
	}
	created, createErr := s.repo.Create(ctx, model)
	if createErr != nil {
		bizErr := mapRepositoryError(createErr)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	s.markActionSucceeded(ctx, actionTypeCreate, requestID, created.PaymentID, created.OrderID)
	return toRPCPayment(created), nil
}
