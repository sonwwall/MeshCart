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

	active, err := s.repo.GetLatestActiveByOrderID(ctx, req.GetOrderId(), req.GetUserId())
	if err == nil {
		s.markActionSucceeded(ctx, actionTypeCreate, requestID, active.PaymentID, active.OrderID)
		return toRPCPayment(active), nil
	}
	if err != nil && err != repository.ErrPaymentNotFound {
		bizErr := mapRepositoryError(err)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
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
	if bizErr := validateOrderForPayment(orderResp.Order); bizErr != nil {
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}

	model := &dalmodel.Payment{
		PaymentID:     s.node.Generate().Int64(),
		OrderID:       req.GetOrderId(),
		UserID:        req.GetUserId(),
		Status:        PaymentStatusPending,
		PaymentMethod: method,
		Amount:        orderResp.Order.GetPayAmount(),
		Currency:      "CNY",
		RequestID:     requestID,
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
