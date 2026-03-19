package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	paymentpb "meshcart/kitex_gen/meshcart/payment"

	"go.uber.org/zap"
)

func (s *PaymentService) GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*paymentpb.Payment, *common.BizError) {
	if req == nil || req.GetPaymentId() <= 0 || req.GetUserId() <= 0 {
		paymentID := int64(0)
		userID := int64(0)
		if req != nil {
			paymentID = req.GetPaymentId()
			userID = req.GetUserId()
		}
		logx.L(ctx).Warn("get payment rejected by invalid request",
			zap.Int64("payment_id", paymentID),
			zap.Int64("user_id", userID),
		)
		return nil, common.ErrInvalidParam
	}
	payment, err := s.repo.GetByPaymentIDUser(ctx, req.GetPaymentId(), req.GetUserId())
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Warn("get payment failed",
			zap.Int64("payment_id", req.GetPaymentId()),
			zap.Int64("user_id", req.GetUserId()),
			zap.Error(err),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
	}
	logx.L(ctx).Info("get payment succeeded",
		zap.Int64("payment_id", payment.PaymentID),
		zap.Int64("order_id", payment.OrderID),
		zap.Int64("user_id", payment.UserID),
		zap.Int32("status", payment.Status),
	)
	return toRPCPayment(payment), nil
}
