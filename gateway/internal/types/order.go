package types

type CreateOrderRequest struct {
	RequestID string                 `json:"request_id"`
	Items     []CreateOrderItemInput `json:"items"`
}

type CreateOrderItemInput struct {
	ProductID int64 `json:"product_id"`
	SKUID     int64 `json:"sku_id"`
	Quantity  int32 `json:"quantity"`
}

type CancelOrderRequest struct {
	RequestID    string `json:"request_id"`
	CancelReason string `json:"cancel_reason"`
}

type ListOrdersRequest struct {
	Page     int32 `query:"page" form:"page"`
	PageSize int32 `query:"page_size" form:"page_size"`
}

type OrderListData struct {
	Orders []OrderSummaryData `json:"orders"`
	Total  int64              `json:"total"`
}

type OrderData struct {
	OrderID      int64           `json:"order_id"`
	UserID       int64           `json:"user_id"`
	Status       int32           `json:"status"`
	TotalAmount  int64           `json:"total_amount"`
	PayAmount    int64           `json:"pay_amount"`
	ExpireAt     int64           `json:"expire_at"`
	CancelReason string          `json:"cancel_reason"`
	PaymentID    string          `json:"payment_id"`
	PaidAt       int64           `json:"paid_at"`
	Items        []OrderItemData `json:"items"`
}

type OrderSummaryData struct {
	OrderID      int64  `json:"order_id"`
	UserID       int64  `json:"user_id"`
	Status       int32  `json:"status"`
	TotalAmount  int64  `json:"total_amount"`
	PayAmount    int64  `json:"pay_amount"`
	ExpireAt     int64  `json:"expire_at"`
	CancelReason string `json:"cancel_reason"`
	PaymentID    string `json:"payment_id"`
	PaidAt       int64  `json:"paid_at"`
	ItemCount    int32  `json:"item_count"`
}

type OrderItemData struct {
	ItemID               int64  `json:"item_id"`
	OrderID              int64  `json:"order_id"`
	ProductID            int64  `json:"product_id"`
	SKUID                int64  `json:"sku_id"`
	ProductTitleSnapshot string `json:"product_title_snapshot"`
	SKUTitleSnapshot     string `json:"sku_title_snapshot"`
	SalePriceSnapshot    int64  `json:"sale_price_snapshot"`
	Quantity             int32  `json:"quantity"`
	SubtotalAmount       int64  `json:"subtotal_amount"`
}
