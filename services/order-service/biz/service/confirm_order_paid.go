package service

import (
	"context"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/errno"
	"meshcart/services/order-service/biz/repository"
	dalmodel "meshcart/services/order-service/dal/model"

	"go.uber.org/zap"
)

func (s *OrderService) ConfirmOrderPaid(ctx context.Context, req *orderpb.ConfirmOrderPaidRequest) (*orderpb.Order, *common.BizError) {
	if req == nil || req.GetOrderId() <= 0 || strings.TrimSpace(req.GetPaymentId()) == "" {
		orderID := int64(0)
		paymentID := ""
		if req != nil {
			orderID = req.GetOrderId()
			paymentID = req.GetPaymentId()
		}
		logx.L(ctx).Warn("confirm order paid rejected by invalid request",
			zap.Int64("order_id", orderID),
			zap.String("payment_id", paymentID),
		)
		return nil, common.ErrInvalidParam
	}

	paymentID := strings.TrimSpace(req.GetPaymentId())
	paymentMethod := normalizePaymentMethod(req.GetPaymentMethod())
	paymentTradeNo := strings.TrimSpace(req.GetPaymentTradeNo())
	actionKey := paymentActionKey(req)
	logx.L(ctx).Info("confirm order paid start",
		zap.Int64("order_id", req.GetOrderId()),
		zap.String("payment_id", paymentID),
		zap.String("payment_method", paymentMethod),
		zap.String("payment_trade_no", paymentTradeNo),
		zap.String("action_key", actionKey),
		zap.Int64("paid_at", req.GetPaidAt()),
	)

	var actionRecord *dalmodel.OrderActionRecord
	record, created, bizErr := s.resolvePendingActionRecord(ctx, actionTypePay, actionKey, req.GetOrderId(), 0)
	if bizErr != nil {
		return nil, bizErr
	}
	if !created {
		logx.L(ctx).Info("confirm order paid reused succeeded action record",
			zap.String("action_key", actionKey),
			zap.Int64("order_id", record.OrderID),
			zap.String("payment_id", paymentID),
		)
		return s.loadOrderByActionRecord(ctx, record)
	}
	actionRecord = record

	order, err := s.repo.GetByID(ctx, req.GetOrderId())
	if err != nil {
		bizErr = mapRepositoryError(err)
		logx.L(ctx).Warn("confirm order paid load order failed",
			zap.Error(err),
			zap.Int64("order_id", req.GetOrderId()),
			zap.String("payment_id", paymentID),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, bizErr)
		return nil, bizErr
	}
	if order.Status == OrderStatusPaid {
		if order.PaymentID == paymentID &&
			!paymentConflict(order.PaymentMethod, paymentMethod) &&
			!paymentConflict(order.PaymentTradeNo, paymentTradeNo) {
			logx.L(ctx).Info("confirm order paid treated as idempotent success",
				zap.Int64("order_id", order.OrderID),
				zap.String("payment_id", paymentID),
				zap.String("payment_method", paymentMethod),
				zap.String("payment_trade_no", paymentTradeNo),
			)
			s.markActionSucceeded(ctx, actionRecord, actionTypePay, actionKey, order.OrderID)
			return toRPCOrder(order), nil
		}
		logx.L(ctx).Warn("confirm order paid rejected by payment conflict",
			zap.Int64("order_id", order.OrderID),
			zap.String("payment_id", paymentID),
			zap.String("existing_payment_id", order.PaymentID),
			zap.String("payment_method", paymentMethod),
			zap.String("existing_payment_method", order.PaymentMethod),
			zap.String("payment_trade_no", paymentTradeNo),
			zap.String("existing_payment_trade_no", order.PaymentTradeNo),
		)
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, errno.ErrOrderPaymentConflict)
		return nil, errno.ErrOrderPaymentConflict
	}
	if order.Status == OrderStatusClosed || order.Status == OrderStatusCancelled {
		logx.L(ctx).Warn("confirm order paid rejected by terminal order status",
			zap.Int64("order_id", order.OrderID),
			zap.String("payment_id", paymentID),
			zap.Int32("status", order.Status),
		)
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, errno.ErrOrderStateConflict)
		return nil, errno.ErrOrderStateConflict
	}
	if order.Status != OrderStatusReserved {
		logx.L(ctx).Warn("confirm order paid rejected by unsupported order status",
			zap.Int64("order_id", order.OrderID),
			zap.String("payment_id", paymentID),
			zap.Int32("status", order.Status),
		)
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, errno.ErrOrderStateConflict)
		return nil, errno.ErrOrderStateConflict
	}
	if !order.ExpireAt.IsZero() && !s.now().Before(order.ExpireAt) {
		logx.L(ctx).Warn("confirm order paid rejected by expired order",
			zap.Int64("order_id", order.OrderID),
			zap.String("payment_id", paymentID),
			zap.Time("expire_at", order.ExpireAt),
			zap.Time("now", s.now()),
		)
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, errno.ErrOrderStateConflict)
		return nil, errno.ErrOrderStateConflict
	}

	confirmResp, confirmErr := s.inventoryClient.ConfirmDeductReservedSkuStocks(ctx, &inventory.ConfirmDeductReservedSkuStocksRequest{
		BizType: orderReserveBizType,
		BizId:   s.reserveBizID(order.OrderID),
		Items:   buildReleaseItems(order),
	})
	if confirmErr != nil {
		logx.L(ctx).Error("confirm deduct inventory failed", zap.Error(confirmErr), zap.Int64("order_id", order.OrderID), zap.String("payment_id", paymentID), zap.String("payment_trade_no", paymentTradeNo))
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if confirmResp.Code != 0 {
		bizErr = mapInventoryRPCError(confirmResp.Code)
		logx.L(ctx).Warn("confirm deduct inventory returned business error",
			zap.Int64("order_id", order.OrderID),
			zap.String("payment_id", paymentID),
			zap.Int32("inventory_rpc_code", confirmResp.Code),
			zap.String("inventory_rpc_message", confirmResp.Message),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, bizErr)
		return nil, bizErr
	}

	paidAt := s.now()
	if req.GetPaidAt() > 0 {
		paidAt = time.Unix(req.GetPaidAt(), 0)
	}
	externalRef := paymentID
	if paymentTradeNo != "" {
		externalRef = paymentTradeNo
	}
	updated, updateErr := s.repo.TransitionStatus(ctx, repository.OrderTransition{
		OrderID:        order.OrderID,
		FromStatuses:   []int32{OrderStatusReserved},
		ToStatus:       OrderStatusPaid,
		CancelReason:   "",
		PaymentID:      paymentID,
		PaymentMethod:  paymentMethod,
		PaymentTradeNo: paymentTradeNo,
		PaidAt:         &paidAt,
		ActionType:     actionTypePay,
		Reason:         "payment_confirmed",
		ExternalRef:    externalRef,
	})
	if updateErr != nil {
		bizErr = mapRepositoryError(updateErr)
		logx.L(ctx).Error("confirm order paid transition failed",
			zap.Error(updateErr),
			zap.Int64("order_id", order.OrderID),
			zap.String("payment_id", paymentID),
			zap.String("payment_trade_no", paymentTradeNo),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		s.markActionFailed(ctx, actionRecord, actionTypePay, actionKey, bizErr)
		return nil, bizErr
	}
	var paidAtLog any
	if updated.PaidAt != nil {
		paidAtLog = *updated.PaidAt
	}
	logx.L(ctx).Info("confirm order paid completed",
		zap.Int64("order_id", updated.OrderID),
		zap.String("payment_id", paymentID),
		zap.String("payment_method", updated.PaymentMethod),
		zap.String("payment_trade_no", updated.PaymentTradeNo),
		zap.Int32("status", updated.Status),
		zap.Any("paid_at", paidAtLog),
	)
	s.markActionSucceeded(ctx, actionRecord, actionTypePay, actionKey, updated.OrderID)
	return toRPCOrder(updated), nil
}
