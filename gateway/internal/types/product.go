package types

type ProductSkuAttrInput struct {
	AttrName  string `json:"attr_name"`
	AttrValue string `json:"attr_value"`
	Sort      int32  `json:"sort"`
}

type ProductSkuInput struct {
	ID           *int64                `json:"id,omitempty"`
	SKUCode      string                `json:"sku_code"`
	Title        string                `json:"title"`
	SalePrice    int64                 `json:"sale_price"`
	MarketPrice  int64                 `json:"market_price"`
	Status       int32                 `json:"status"`
	CoverURL     string                `json:"cover_url"`
	InitialStock *int64                `json:"initial_stock,omitempty"`
	Attrs        []ProductSkuAttrInput `json:"attrs"`
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
	Page       int32  `query:"page" form:"page"`
	PageSize   int32  `query:"page_size" form:"page_size"`
	Status     *int32 `query:"status" form:"status"`
	CategoryID *int64 `query:"category_id" form:"category_id"`
	Keyword    string `query:"keyword" form:"keyword"`
}

type CreateProductData struct {
	ProductID int64                   `json:"product_id"`
	SKUs      []CreatedProductSKUData `json:"skus"`
}

type CreatedProductSKUData struct {
	ID      int64  `json:"id"`
	SKUCode string `json:"sku_code"`
}

type ListProductsData struct {
	Products []ProductListItemData `json:"products"`
	Total    int64                 `json:"total"`
}

type ProductListItemData struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	SubTitle     string `json:"sub_title"`
	CategoryID   int64  `json:"category_id"`
	Brand        string `json:"brand"`
	Status       int32  `json:"status"`
	MinSalePrice int64  `json:"min_sale_price"`
	CoverURL     string `json:"cover_url"`
}

type ProductDetailData struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	SubTitle    string           `json:"sub_title"`
	CategoryID  int64            `json:"category_id"`
	Brand       string           `json:"brand"`
	Description string           `json:"description"`
	Status      int32            `json:"status"`
	SKUs        []ProductSKUData `json:"skus"`
}

type AdminProductDetailData struct {
	ID          int64                 `json:"id"`
	Title       string                `json:"title"`
	SubTitle    string                `json:"sub_title"`
	CategoryID  int64                 `json:"category_id"`
	Brand       string                `json:"brand"`
	Description string                `json:"description"`
	Status      int32                 `json:"status"`
	CreatorID   int64                 `json:"creator_id"`
	SKUs        []AdminProductSKUData `json:"skus"`
}

type ProductSKUData struct {
	ID          int64                `json:"id"`
	SPUID       int64                `json:"spu_id"`
	SKUCode     string               `json:"sku_code"`
	Title       string               `json:"title"`
	SalePrice   int64                `json:"sale_price"`
	MarketPrice int64                `json:"market_price"`
	Status      int32                `json:"status"`
	CoverURL    string               `json:"cover_url"`
	Attrs       []ProductSKUAttrData `json:"attrs"`
}

type ProductSKUAttrData struct {
	ID        int64  `json:"id"`
	SKUID     int64  `json:"sku_id"`
	AttrName  string `json:"attr_name"`
	AttrValue string `json:"attr_value"`
	Sort      int32  `json:"sort"`
}

type AdminProductSKUData struct {
	ID          int64                `json:"id"`
	SPUID       int64                `json:"spu_id"`
	SKUCode     string               `json:"sku_code"`
	Title       string               `json:"title"`
	SalePrice   int64                `json:"sale_price"`
	MarketPrice int64                `json:"market_price"`
	Status      int32                `json:"status"`
	CoverURL    string               `json:"cover_url"`
	Attrs       []ProductSKUAttrData `json:"attrs"`
	Inventory   *InventoryStockData  `json:"inventory"`
}
