package types

import productpb "meshcart/kitex_gen/meshcart/product"

type ProductSkuAttrInput struct {
	AttrName  string `json:"attr_name"`
	AttrValue string `json:"attr_value"`
	Sort      int32  `json:"sort"`
}

type ProductSkuInput struct {
	ID          *int64                `json:"id,omitempty"`
	SKUCode     string                `json:"sku_code"`
	Title       string                `json:"title"`
	SalePrice   int64                 `json:"sale_price"`
	MarketPrice int64                 `json:"market_price"`
	Status      int32                 `json:"status"`
	CoverURL    string                `json:"cover_url"`
	Attrs       []ProductSkuAttrInput `json:"attrs"`
}

type CreateProductRequest struct {
	Title       string            `json:"title"`
	SubTitle    string            `json:"sub_title"`
	CategoryID  int64             `json:"category_id"`
	Brand       string            `json:"brand"`
	Description string            `json:"description"`
	Status      int32             `json:"status"`
	SKUs        []ProductSkuInput `json:"skus"`
}

type UpdateProductRequest struct {
	Title       string            `json:"title"`
	SubTitle    string            `json:"sub_title"`
	CategoryID  int64             `json:"category_id"`
	Brand       string            `json:"brand"`
	Description string            `json:"description"`
	Status      int32             `json:"status"`
	SKUs        []ProductSkuInput `json:"skus"`
}

type ChangeProductStatusRequest struct {
	Status int32 `json:"status"`
}

type ListProductsRequest struct {
	Page       int32  `query:"page"`
	PageSize   int32  `query:"page_size"`
	Status     *int32 `query:"status"`
	CategoryID *int64 `query:"category_id"`
	Keyword    string `query:"keyword"`
}

type CreateProductData struct {
	ProductID int64 `json:"product_id"`
}

type ListProductsData struct {
	Products []*productpb.ProductListItem `json:"products"`
	Total    int64                        `json:"total"`
}
