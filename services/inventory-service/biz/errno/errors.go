package errno

import "meshcart/app/common"

const (
	CodeInventoryStockNotFound int32 = 2050001
	CodeInsufficientStock      int32 = 2050002
	CodeStockAlreadyExists     int32 = 2050003
	CodeInvalidStockQuantity   int32 = 2050004
	CodeStockFrozen            int32 = 2050005
	CodeReservationConflict    int32 = 2050006
	CodeReservationNotFound    int32 = 2050007
)

var (
	ErrInventoryStockNotFound = common.NewBizError(CodeInventoryStockNotFound, "库存记录不存在")
	ErrInsufficientStock      = common.NewBizError(CodeInsufficientStock, "库存不足")
	ErrStockAlreadyExists     = common.NewBizError(CodeStockAlreadyExists, "库存记录已存在")
	ErrInvalidStockQuantity   = common.NewBizError(CodeInvalidStockQuantity, "库存数量不合法")
	ErrStockFrozen            = common.NewBizError(CodeStockFrozen, "库存已冻结")
	ErrReservationConflict    = common.NewBizError(CodeReservationConflict, "库存预占状态冲突")
	ErrReservationNotFound    = common.NewBizError(CodeReservationNotFound, "库存预占记录不存在")
)
