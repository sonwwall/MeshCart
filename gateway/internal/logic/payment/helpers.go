package payment

import (
	"strconv"

	"meshcart/app/common"
	"meshcart/gateway/internal/types"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
)

func parsePaymentID(raw string) (int64, *common.BizError) {
	if raw == "" {
		return 0, common.ErrInvalidParam
	}
	paymentID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || paymentID <= 0 {
		return 0, common.ErrInvalidParam
	}
	return paymentID, nil
}

func parseOrderID(raw string) (int64, *common.BizError) {
	if raw == "" {
		return 0, common.ErrInvalidParam
	}
	orderID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || orderID <= 0 {
		return 0, common.ErrInvalidParam
	}
	return orderID, nil
}

func toPaymentData(payment *paymentpb.Payment) *types.PaymentData {
	if payment == nil {
		return nil
	}
	return &types.PaymentData{
		PaymentID:      payment.GetPaymentId(),
		OrderID:        payment.GetOrderId(),
		UserID:         payment.GetUserId(),
		Status:         payment.GetStatus(),
		PaymentMethod:  payment.GetPaymentMethod(),
		Amount:         payment.GetAmount(),
		Currency:       payment.GetCurrency(),
		PaymentTradeNo: payment.GetPaymentTradeNo(),
		ExpireAt:       payment.GetExpireAt(),
		SucceededAt:    payment.GetSucceededAt(),
		ClosedAt:       payment.GetClosedAt(),
		FailReason:     payment.GetFailReason(),
	}
}

func toPaymentListData(payments []*paymentpb.Payment) *types.PaymentListData {
	items := make([]types.PaymentData, 0, len(payments))
	for _, payment := range payments {
		if data := toPaymentData(payment); data != nil {
			items = append(items, *data)
		}
	}
	return &types.PaymentListData{Payments: items}
}
