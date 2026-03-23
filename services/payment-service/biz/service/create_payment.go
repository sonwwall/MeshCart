package service

import (
	"context"

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
		orderID := int64(0)
		userID := int64(0)
		if req != nil {
			orderID = req.GetOrderId()
			userID = req.GetUserId()
		}
		logx.L(ctx).Warn("create payment rejected by invalid request",
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", userID),
		)
		return nil, common.ErrInvalidParam
	}
	method := normalizePaymentMethod(req.GetPaymentMethod())
	if bizErr := validatePaymentMethod(method); bizErr != nil {
		logx.L(ctx).Warn("create payment rejected by unsupported method",
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.String("payment_method", req.GetPaymentMethod()),
			zap.Int32("code", bizErr.Code),
		)
		return nil, bizErr
	}

	requestID, bizErr := requireRequestID(req.GetRequestId())
	if bizErr != nil {
		logx.L(ctx).Warn("create payment rejected by missing request_id",
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.String("payment_method", method),
		)
		return nil, bizErr
	}
	logx.L(ctx).Info("create payment start",
		zap.Int64("order_id", req.GetOrderId()),
		zap.Int64("user_id", req.GetUserId()),
		zap.String("payment_method", method),
		zap.String("request_id", requestID),
	)
	existing, bizErr := s.findActionRecord(ctx, actionTypeCreate, requestID)
	if bizErr != nil {
		return nil, bizErr
	}
	if existing != nil {
		switch existing.Status {
		case "succeeded":
			logx.L(ctx).Info("create payment hit succeeded action record",
				zap.String("request_id", requestID),
				zap.Int64("payment_id", existing.PaymentID),
				zap.Int64("order_id", existing.OrderID),
			)
			return s.loadPaymentByActionRecord(ctx, existing)
		case actionStatusPending:
			logx.L(ctx).Warn("create payment blocked by pending action record",
				zap.String("request_id", requestID),
				zap.Int64("order_id", req.GetOrderId()),
			)
			return nil, errno.ErrPaymentIdempotencyBusy
		default:
			logx.L(ctx).Warn("create payment rejected by failed action record",
				zap.String("request_id", requestID),
				zap.Int64("order_id", req.GetOrderId()),
				zap.String("status", existing.Status),
			)
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

	existingPayments, err := s.repo.ListByOrderID(ctx, req.GetOrderId(), req.GetUserId())
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Error("create payment list existing payments failed",
			zap.Error(err),
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int32("mapped_code", bizErr.Code),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	for _, existingPayment := range existingPayments {
		switch existingPayment.Status {
		case PaymentStatusSucceeded:
			logx.L(ctx).Info("create payment reused succeeded payment",
				zap.Int64("payment_id", existingPayment.PaymentID),
				zap.Int64("order_id", existingPayment.OrderID),
				zap.Int64("user_id", existingPayment.UserID),
			)
			s.markActionSucceeded(ctx, actionTypeCreate, requestID, existingPayment.PaymentID, existingPayment.OrderID)
			return toRPCPayment(existingPayment), nil
		case PaymentStatusPending:
			if s.isPaymentExpired(existingPayment) {
				logx.L(ctx).Warn("create payment found expired pending payment",
					zap.Int64("payment_id", existingPayment.PaymentID),
					zap.Int64("order_id", existingPayment.OrderID),
					zap.Time("expire_at", existingPayment.ExpireAt),
					zap.Time("now", s.now()),
				)
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
			logx.L(ctx).Info("create payment reused pending payment",
				zap.Int64("payment_id", existingPayment.PaymentID),
				zap.Int64("order_id", existingPayment.OrderID),
				zap.Time("expire_at", existingPayment.ExpireAt),
			)
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
		logx.L(ctx).Warn("create payment blocked by order rpc business error",
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int32("order_rpc_code", orderResp.Code),
			zap.String("order_rpc_message", orderResp.Message),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	if bizErr := s.validateOrderForPayment(orderResp.Order); bizErr != nil {
		logx.L(ctx).Warn("create payment rejected by order state",
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int32("order_status", orderResp.Order.GetStatus()),
			zap.Int64("order_expire_at", orderResp.Order.GetExpireAt()),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	expireAt := s.calculatePaymentExpireAt(orderResp.Order)

	model := &dalmodel.Payment{
		PaymentID:     s.node.Generate().Int64(),
		OrderID:       req.GetOrderId(),
		ActiveOrderID: int64Pointer(req.GetOrderId()),
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
		if createErr == repository.ErrActivePaymentExists {
			activePayment, activeErr := s.repo.GetLatestActiveByOrderID(ctx, req.GetOrderId(), req.GetUserId())
			if activeErr == nil {
				logx.L(ctx).Info("create payment reused active payment after unique guard conflict",
					zap.Int64("payment_id", activePayment.PaymentID),
					zap.Int64("order_id", activePayment.OrderID),
					zap.Int64("user_id", activePayment.UserID),
					zap.String("request_id", requestID),
				)
				s.markActionSucceeded(ctx, actionTypeCreate, requestID, activePayment.PaymentID, activePayment.OrderID)
				return toRPCPayment(activePayment), nil
			}
			logx.L(ctx).Error("create payment load active payment after unique guard conflict failed",
				zap.Error(activeErr),
				zap.Int64("order_id", req.GetOrderId()),
				zap.Int64("user_id", req.GetUserId()),
				zap.String("request_id", requestID),
			)
		}
		bizErr := mapRepositoryError(createErr)
		logx.L(ctx).Error("create payment persist failed",
			zap.Error(createErr),
			zap.Int64("payment_id", model.PaymentID),
			zap.Int64("order_id", model.OrderID),
			zap.Int64("user_id", model.UserID),
			zap.Int32("mapped_code", bizErr.Code),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	logx.L(ctx).Info("create payment completed",
		zap.Int64("payment_id", created.PaymentID),
		zap.Int64("order_id", created.OrderID),
		zap.Int64("user_id", created.UserID),
		zap.Time("expire_at", created.ExpireAt),
		zap.Int32("status", created.Status),
	)
	s.markActionSucceeded(ctx, actionTypeCreate, requestID, created.PaymentID, created.OrderID)
	return toRPCPayment(created), nil
}
