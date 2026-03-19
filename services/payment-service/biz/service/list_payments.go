package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	paymentpb "meshcart/kitex_gen/meshcart/payment"

	"go.uber.org/zap"
)

func (s *PaymentService) ListPaymentsByOrder(ctx context.Context, req *paymentpb.ListPaymentsByOrderRequest) ([]*paymentpb.Payment, *common.BizError) {
	if req == nil || req.GetOrderId() <= 0 || req.GetUserId() <= 0 {
		orderID := int64(0)
		userID := int64(0)
		if req != nil {
			orderID = req.GetOrderId()
			userID = req.GetUserId()
		}
		logx.L(ctx).Warn("list payments rejected by invalid request",
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", userID),
		)
		return nil, common.ErrInvalidParam
	}
	payments, err := s.repo.ListByOrderID(ctx, req.GetOrderId(), req.GetUserId())
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Warn("list payments failed",
			zap.Int64("order_id", req.GetOrderId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.Error(err),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
	}
	paymentIDs := make([]int64, 0, len(payments))
	for _, payment := range payments {
		if payment != nil {
			paymentIDs = append(paymentIDs, payment.PaymentID)
		}
	}
	logx.L(ctx).Info("list payments succeeded",
		zap.Int64("order_id", req.GetOrderId()),
		zap.Int64("user_id", req.GetUserId()),
		zap.Int("payment_count", len(payments)),
		zap.Int64s("payment_ids", paymentIDs),
	)
	return toRPCPayments(payments), nil
}
