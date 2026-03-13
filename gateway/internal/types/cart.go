package types

type AddCartItemRequest struct {
	ProductID int64 `json:"product_id"`
	SKUID     int64 `json:"sku_id"`
	Quantity  int32 `json:"quantity"`
	Checked   *bool `json:"checked,omitempty"`
}

type UpdateCartItemRequest struct {
	Quantity int32 `json:"quantity"`
	Checked  *bool `json:"checked,omitempty"`
}

type CartData struct {
	Items []CartItemData `json:"items"`
}

type CartItemData struct {
	ID                int64  `json:"id"`
	ProductID         int64  `json:"product_id"`
	SKUID             int64  `json:"sku_id"`
	Quantity          int32  `json:"quantity"`
	Checked           bool   `json:"checked"`
	TitleSnapshot     string `json:"title_snapshot"`
	SKUTitleSnapshot  string `json:"sku_title_snapshot"`
	SalePriceSnapshot int64  `json:"sale_price_snapshot"`
	CoverURLSnapshot  string `json:"cover_url_snapshot"`
}
