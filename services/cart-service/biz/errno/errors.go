package errno

import "meshcart/app/common"

const (
	CodeCartItemNotFound int32 = 2030001
)

var (
	ErrCartItemNotFound = common.NewBizError(CodeCartItemNotFound, "购物车项不存在")
)
