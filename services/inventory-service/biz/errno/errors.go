package errno

import "meshcart/app/common"

const (
	CodeInventoryStockNotFound int32 = 2050001
	CodeInsufficientStock      int32 = 2050002
)

var (
	ErrInventoryStockNotFound = common.NewBizError(CodeInventoryStockNotFound, "库存记录不存在")
	ErrInsufficientStock      = common.NewBizError(CodeInsufficientStock, "库存不足")
)
