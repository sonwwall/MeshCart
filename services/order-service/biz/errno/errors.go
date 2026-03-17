package errno

import "meshcart/app/common"

const (
	CodeOrderNotFound    int32 = 2040001
	CodeInvalidOrderData int32 = 2040002
)

var (
	ErrOrderNotFound    = common.NewBizError(CodeOrderNotFound, "订单不存在")
	ErrInvalidOrderData = common.NewBizError(CodeInvalidOrderData, "订单数据不合法")
)
