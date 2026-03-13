namespace go meshcart.cart

include "base.thrift"

struct CartItem {
    1: i64 id
    2: i64 user_id
    3: i64 product_id
    4: i64 sku_id
    5: i32 quantity
    6: bool checked
    7: string title_snapshot
    8: string sku_title_snapshot
    9: i64 sale_price_snapshot
    10: string cover_url_snapshot
}

struct GetCartRequest {
    1: i64 user_id
}

struct GetCartResponse {
    1: list<CartItem> items
    255: base.BaseResponse base
}

struct AddCartItemRequest {
    1: i64 user_id
    2: i64 product_id
    3: i64 sku_id
    4: i32 quantity
    5: optional bool checked
    6: string title_snapshot
    7: string sku_title_snapshot
    8: i64 sale_price_snapshot
    9: string cover_url_snapshot
}

struct AddCartItemResponse {
    1: CartItem item
    255: base.BaseResponse base
}

struct UpdateCartItemRequest {
    1: i64 user_id
    2: i64 item_id
    3: i32 quantity
    4: optional bool checked
}

struct UpdateCartItemResponse {
    1: CartItem item
    255: base.BaseResponse base
}

struct RemoveCartItemRequest {
    1: i64 user_id
    2: i64 item_id
}

struct RemoveCartItemResponse {
    255: base.BaseResponse base
}

struct ClearCartRequest {
    1: i64 user_id
}

struct ClearCartResponse {
    255: base.BaseResponse base
}

service CartService {
    GetCartResponse getCart(1: GetCartRequest request)
    AddCartItemResponse addCartItem(1: AddCartItemRequest request)
    UpdateCartItemResponse updateCartItem(1: UpdateCartItemRequest request)
    RemoveCartItemResponse removeCartItem(1: RemoveCartItemRequest request)
    ClearCartResponse clearCart(1: ClearCartRequest request)
}
