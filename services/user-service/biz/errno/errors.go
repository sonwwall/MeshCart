package errno

import "meshcart/app/common"

const (
	CodeUserNotFound    int32 = 201001
	CodePasswordInvalid int32 = 201002
	CodeUserLocked      int32 = 201003
)

var (
	ErrUserNotFound    = common.NewBizError(CodeUserNotFound, "用户不存在")
	ErrPasswordInvalid = common.NewBizError(CodePasswordInvalid, "用户名或密码错误")
	ErrUserLocked      = common.NewBizError(CodeUserLocked, "用户已被锁定")
)
