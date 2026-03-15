namespace go meshcart.inventory

include "base.thrift"

struct SkuStock {
    1: i64 sku_id
    2: i64 total_stock
    3: i64 reserved_stock
    4: i64 available_stock
    5: i64 saleable_stock
    6: i32 status
}

struct GetSkuStockRequest {
    1: i64 sku_id
}

struct GetSkuStockResponse {
    1: SkuStock stock
    255: base.BaseResponse base
}

struct BatchGetSkuStockRequest {
    1: list<i64> sku_ids
}

struct BatchGetSkuStockResponse {
    1: list<SkuStock> stocks
    255: base.BaseResponse base
}

struct CheckSaleableStockRequest {
    1: i64 sku_id
    2: i32 quantity
}

struct CheckSaleableStockResponse {
    1: bool saleable
    2: i64 available_stock
    255: base.BaseResponse base
}

struct InitSkuStockItem {
    1: i64 sku_id
    2: i64 total_stock
}

struct InitSkuStocksRequest {
    1: list<InitSkuStockItem> stocks
}

struct InitSkuStocksResponse {
    1: list<SkuStock> stocks
    255: base.BaseResponse base
}

struct FreezeSkuStocksRequest {
    1: list<i64> sku_ids
    2: i64 operator_id
    3: optional string reason
}

struct FreezeSkuStocksResponse {
    1: list<SkuStock> stocks
    255: base.BaseResponse base
}

struct AdjustStockRequest {
    1: i64 sku_id
    2: i64 total_stock
    3: optional string reason
}

struct AdjustStockResponse {
    1: SkuStock stock
    255: base.BaseResponse base
}

service InventoryService {
    GetSkuStockResponse getSkuStock(1: GetSkuStockRequest request)
    BatchGetSkuStockResponse batchGetSkuStock(1: BatchGetSkuStockRequest request)
    CheckSaleableStockResponse checkSaleableStock(1: CheckSaleableStockRequest request)
    InitSkuStocksResponse initSkuStocks(1: InitSkuStocksRequest request)
    FreezeSkuStocksResponse freezeSkuStocks(1: FreezeSkuStocksRequest request)
    AdjustStockResponse adjustStock(1: AdjustStockRequest request)
}
