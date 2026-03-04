package common

type BizError struct {
	Code int32
	Msg  string
}

func (e *BizError) Error() string { return e.Msg }

func NewBizError(code int32, msg string) *BizError {
	return &BizError{
		Code: code,
		Msg:  msg,
	}
}

const (
	CodeOK            int32 = 0
	CodeInvalidParam  int32 = 100001
	CodeUnauthorized  int32 = 100002
	CodeForbidden     int32 = 100003
	CodeNotFound      int32 = 100004
	CodeInternalError int32 = 500001
)

var (
	ErrInvalidParam  = NewBizError(CodeInvalidParam, "请求参数错误")
	ErrUnauthorized  = NewBizError(CodeUnauthorized, "未登录或登录已过期")
	ErrForbidden     = NewBizError(CodeForbidden, "无权限访问")
	ErrNotFound      = NewBizError(CodeNotFound, "资源不存在")
	ErrInternalError = NewBizError(CodeInternalError, "系统内部错误")
)
