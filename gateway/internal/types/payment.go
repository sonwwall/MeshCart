package types

type CreatePaymentRequest struct {
	OrderID       int64  `json:"order_id"`
	PaymentMethod string `json:"payment_method"`
	RequestID     string `json:"request_id"`
}

type ConfirmMockPaymentRequest struct {
	RequestID      string `json:"request_id"`
	PaymentTradeNo string `json:"payment_trade_no"`
	PaidAt         int64  `json:"paid_at"`
}

type PaymentData struct {
	PaymentID      int64  `json:"payment_id"`
	OrderID        int64  `json:"order_id"`
	UserID         int64  `json:"user_id"`
	Status         int32  `json:"status"`
	PaymentMethod  string `json:"payment_method"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	PaymentTradeNo string `json:"payment_trade_no"`
	SucceededAt    int64  `json:"succeeded_at"`
	ClosedAt       int64  `json:"closed_at"`
	FailReason     string `json:"fail_reason"`
}

type PaymentListData struct {
	Payments []PaymentData `json:"payments"`
}
