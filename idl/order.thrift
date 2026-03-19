namespace go meshcart.order

include "base.thrift"

struct OrderItemInput {
    1: i64 product_id
    2: i64 sku_id
    3: i32 quantity
}

struct OrderItem {
    1: i64 item_id
    2: i64 order_id
    3: i64 product_id
    4: i64 sku_id
    5: string product_title_snapshot
    6: string sku_title_snapshot
    7: i64 sale_price_snapshot
    8: i32 quantity
    9: i64 subtotal_amount
}

struct Order {
    1: i64 order_id
    2: i64 user_id
    3: i32 status
    4: i64 total_amount
    5: i64 pay_amount
    6: i64 expire_at
    7: list<OrderItem> items
    8: string cancel_reason
    9: string payment_id
    10: i64 paid_at
    11: string payment_method
    12: string payment_trade_no
}

struct CreateOrderRequest {
    1: i64 user_id
    2: list<OrderItemInput> items
    3: optional string request_id
}

struct CreateOrderResponse {
    1: Order order
    255: base.BaseResponse base
}

struct CancelOrderRequest {
    1: i64 user_id
    2: i64 order_id
    3: optional string cancel_reason
    4: optional string request_id
}

struct CancelOrderResponse {
    1: Order order
    255: base.BaseResponse base
}

struct ConfirmOrderPaidRequest {
    1: i64 order_id
    2: string payment_id
    3: optional string request_id
    4: optional string payment_method
    5: optional string payment_trade_no
    6: optional i64 paid_at
}

struct ConfirmOrderPaidResponse {
    1: Order order
    255: base.BaseResponse base
}

struct GetOrderRequest {
    1: i64 user_id
    2: i64 order_id
}

struct GetOrderResponse {
    1: Order order
    255: base.BaseResponse base
}

struct ListOrdersRequest {
    1: i64 user_id
    2: i32 page
    3: i32 page_size
}

struct ListOrdersResponse {
    1: list<Order> orders
    2: i64 total
    255: base.BaseResponse base
}

struct CloseExpiredOrdersRequest {
    1: optional i32 limit
}

struct CloseExpiredOrdersResponse {
    1: i32 closed_count
    2: list<i64> order_ids
    255: base.BaseResponse base
}

service OrderService {
    CreateOrderResponse createOrder(1: CreateOrderRequest request)
    CancelOrderResponse cancelOrder(1: CancelOrderRequest request)
    ConfirmOrderPaidResponse confirmOrderPaid(1: ConfirmOrderPaidRequest request)
    GetOrderResponse getOrder(1: GetOrderRequest request)
    ListOrdersResponse listOrders(1: ListOrdersRequest request)
    CloseExpiredOrdersResponse closeExpiredOrders(1: CloseExpiredOrdersRequest request)
}
