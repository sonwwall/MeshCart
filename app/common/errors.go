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
	CodeInvalidParam  int32 = 1000001
	CodeUnauthorized  int32 = 1000002
	CodeForbidden     int32 = 1000003
	CodeNotFound      int32 = 1000004
	CodeInternalError int32 = 1009999
)

var (
	ErrInvalidParam  = NewBizError(CodeInvalidParam, "请求参数错误")
	ErrUnauthorized  = NewBizError(CodeUnauthorized, "未登录或登录已过期")
	ErrForbidden     = NewBizError(CodeForbidden, "无权限访问")
	ErrNotFound      = NewBizError(CodeNotFound, "资源不存在")
	ErrInternalError = NewBizError(CodeInternalError, "系统内部错误")
)
