package errno

import "meshcart/app/common"

const (
	CodeUserNotFound    int32 = 2010001
	CodePasswordInvalid int32 = 2010002
	CodeUserLocked      int32 = 2010003
	CodeUserExists      int32 = 2010004
	CodePasswordIllegal int32 = 2010005
)

var (
	ErrUserNotFound    = common.NewBizError(CodeUserNotFound, "用户不存在")
	ErrPasswordInvalid = common.NewBizError(CodePasswordInvalid, "用户名或密码错误")
	ErrUserLocked      = common.NewBizError(CodeUserLocked, "用户已被锁定")
	ErrUserExists      = common.NewBizError(CodeUserExists, "用户名已存在")
	ErrPasswordIllegal = common.NewBizError(CodePasswordIllegal, "密码格式不合法")
)
