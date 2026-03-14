namespace go meshcart.product

include "base.thrift"

struct ProductSkuAttr {
    1: i64 id
    2: i64 sku_id
    3: string attr_name
    4: string attr_value
    5: i32 sort
}

struct ProductSku {
    1: i64 id
    2: i64 spu_id
    3: string sku_code
    4: string title
    5: i64 sale_price
    6: i64 market_price
    7: i32 status
    8: string cover_url
    9: list<ProductSkuAttr> attrs
}

struct Product {
    1: i64 id
    2: string title
    3: string sub_title
    4: i64 category_id
    5: string brand
    6: string description
    7: i32 status
    8: list<ProductSku> skus
    9: i64 creator_id
}

struct ProductListItem {
    1: i64 id
    2: string title
    3: string sub_title
    4: i64 category_id
    5: string brand
    6: i32 status
    7: i64 min_sale_price
    8: string cover_url
    9: i64 creator_id
}

struct ProductSkuAttrInput {
    1: string attr_name
    2: string attr_value
    3: i32 sort
}

struct ProductSkuInput {
    1: optional i64 id
    2: string sku_code
    3: string title
    4: i64 sale_price
    5: i64 market_price
    6: i32 status
    7: string cover_url
    8: list<ProductSkuAttrInput> attrs
}

struct CreateProductRequest {
    1: string title
    2: string sub_title
    3: i64 category_id
    4: string brand
    5: string description
    6: i32 status
    7: list<ProductSkuInput> skus
    8: i64 creator_id
}

struct CreateProductResponse {
    1: i64 product_id
    2: list<ProductSku> skus
    255: base.BaseResponse base
}

struct UpdateProductRequest {
    1: i64 product_id
    2: string title
    3: string sub_title
    4: i64 category_id
    5: string brand
    6: string description
    7: i32 status
    8: list<ProductSkuInput> skus
    9: i64 operator_id
}

struct UpdateProductResponse {
    255: base.BaseResponse base
}

struct ChangeProductStatusRequest {
    1: i64 product_id
    2: i32 status
    3: i64 operator_id
}

struct ChangeProductStatusResponse {
    255: base.BaseResponse base
}

struct GetProductDetailRequest {
    1: i64 product_id
}

struct GetProductDetailResponse {
    1: Product product
    255: base.BaseResponse base
}

struct ListProductsRequest {
    1: i32 page
    2: i32 page_size
    3: optional i32 status
    4: optional i64 category_id
    5: optional string keyword
    6: optional i64 creator_id
}

struct ListProductsResponse {
    1: list<ProductListItem> products
    2: i64 total
    255: base.BaseResponse base
}

struct BatchGetSkuRequest {
    1: list<i64> sku_ids
}

struct BatchGetSkuResponse {
    1: list<ProductSku> skus
    255: base.BaseResponse base
}

service ProductService {
    CreateProductResponse createProduct(1: CreateProductRequest request)
    UpdateProductResponse updateProduct(1: UpdateProductRequest request)
    ChangeProductStatusResponse changeProductStatus(1: ChangeProductStatusRequest request)
    GetProductDetailResponse getProductDetail(1: GetProductDetailRequest request)
    ListProductsResponse listProducts(1: ListProductsRequest request)
    BatchGetSkuResponse batchGetSku(1: BatchGetSkuRequest request)
}
