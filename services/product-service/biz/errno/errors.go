package errno

import "meshcart/app/common"

const (
	CodeProductNotFound     int32 = 2020001
	CodeSKUNotFound         int32 = 2020002
	CodeProductStatusError  int32 = 2020003
	CodeSKUStatusError      int32 = 2020004
	CodeSKUCodeExists       int32 = 2020005
	CodeProductDataConflict int32 = 2020006
)

var (
	ErrProductNotFound     = common.NewBizError(CodeProductNotFound, "商品不存在")
	ErrSKUNotFound         = common.NewBizError(CodeSKUNotFound, "SKU 不存在")
	ErrProductStatusError  = common.NewBizError(CodeProductStatusError, "商品状态不合法")
	ErrSKUStatusError      = common.NewBizError(CodeSKUStatusError, "SKU 状态不合法")
	ErrSKUCodeExists       = common.NewBizError(CodeSKUCodeExists, "SKU 编码已存在")
	ErrProductDataConflict = common.NewBizError(CodeProductDataConflict, "商品数据写入冲突")
)
