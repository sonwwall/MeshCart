namespace go meshcart.payment

include "base.thrift"

struct Payment {
    1: i64 payment_id
    2: i64 order_id
    3: i64 user_id
    4: i32 status
    5: string payment_method
    6: i64 amount
    7: string currency
    8: string payment_trade_no
    9: i64 succeeded_at
    10: i64 closed_at
    11: string fail_reason
}

struct CreatePaymentRequest {
    1: i64 order_id
    2: i64 user_id
    3: string payment_method
    4: optional string request_id
}

struct CreatePaymentResponse {
    1: Payment payment
    255: base.BaseResponse base
}

struct GetPaymentRequest {
    1: i64 payment_id
    2: i64 user_id
}

struct GetPaymentResponse {
    1: Payment payment
    255: base.BaseResponse base
}

struct ListPaymentsByOrderRequest {
    1: i64 order_id
    2: i64 user_id
}

struct ListPaymentsByOrderResponse {
    1: list<Payment> payments
    255: base.BaseResponse base
}

struct ConfirmPaymentSuccessRequest {
    1: i64 payment_id
    2: string payment_method
    3: optional string payment_trade_no
    4: optional i64 paid_at
    5: optional string request_id
}

struct ConfirmPaymentSuccessResponse {
    1: Payment payment
    255: base.BaseResponse base
}

service PaymentService {
    CreatePaymentResponse createPayment(1: CreatePaymentRequest request)
    GetPaymentResponse getPayment(1: GetPaymentRequest request)
    ListPaymentsByOrderResponse listPaymentsByOrder(1: ListPaymentsByOrderRequest request)
    ConfirmPaymentSuccessResponse confirmPaymentSuccess(1: ConfirmPaymentSuccessRequest request)
}
