package errno

import "meshcart/app/common"

const (
	CodeOrderNotFound           int32 = 2040001
	CodeInvalidOrderData        int32 = 2040002
	CodeOrderProductUnavailable int32 = 2040003
	CodeOrderSKUUnavailable     int32 = 2040004
	CodeOrderInsufficientStock  int32 = 2040005
	CodeOrderStateConflict      int32 = 2040006
	CodeOrderPaidImmutable      int32 = 2040007
	CodeOrderPaymentConflict    int32 = 2040008
	CodeOrderIdempotencyBusy    int32 = 2040009
)

var (
	ErrOrderNotFound           = common.NewBizError(CodeOrderNotFound, "订单不存在")
	ErrInvalidOrderData        = common.NewBizError(CodeInvalidOrderData, "订单数据不合法")
	ErrOrderProductUnavailable = common.NewBizError(CodeOrderProductUnavailable, "商品不可下单")
	ErrOrderSKUUnavailable     = common.NewBizError(CodeOrderSKUUnavailable, "SKU 不可下单")
	ErrOrderInsufficientStock  = common.NewBizError(CodeOrderInsufficientStock, "库存不足")
	ErrOrderStateConflict      = common.NewBizError(CodeOrderStateConflict, "订单状态不允许当前操作")
	ErrOrderPaidImmutable      = common.NewBizError(CodeOrderPaidImmutable, "已支付订单不可取消")
	ErrOrderPaymentConflict    = common.NewBizError(CodeOrderPaymentConflict, "订单支付信息冲突")
	ErrOrderIdempotencyBusy    = common.NewBizError(CodeOrderIdempotencyBusy, "请求正在处理中，请稍后重试")
)
