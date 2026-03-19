package errno

import "meshcart/app/common"

const (
	CodePaymentNotFound           int32 = 2060001
	CodeInvalidPaymentData        int32 = 2060002
	CodeUnsupportedPaymentMethod  int32 = 2060003
	CodePaymentStateConflict      int32 = 2060004
	CodePaymentOrderNotFound      int32 = 2060005
	CodePaymentOrderStateConflict int32 = 2060006
	CodePaymentConflict           int32 = 2060007
	CodePaymentIdempotencyBusy    int32 = 2060008
)

var (
	ErrPaymentNotFound           = common.NewBizError(CodePaymentNotFound, "支付单不存在")
	ErrInvalidPaymentData        = common.NewBizError(CodeInvalidPaymentData, "支付数据不合法")
	ErrUnsupportedPaymentMethod  = common.NewBizError(CodeUnsupportedPaymentMethod, "暂不支持该支付方式")
	ErrPaymentStateConflict      = common.NewBizError(CodePaymentStateConflict, "支付状态不允许当前操作")
	ErrPaymentOrderNotFound      = common.NewBizError(CodePaymentOrderNotFound, "关联订单不存在")
	ErrPaymentOrderStateConflict = common.NewBizError(CodePaymentOrderStateConflict, "订单状态不允许发起支付")
	ErrPaymentConflict           = common.NewBizError(CodePaymentConflict, "支付信息冲突")
	ErrPaymentIdempotencyBusy    = common.NewBizError(CodePaymentIdempotencyBusy, "请求正在处理中，请稍后重试")
)
