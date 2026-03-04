package common

type HTTPResponse struct {
	Code    int32       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

func Success(data interface{}, traceID string) HTTPResponse {
	return HTTPResponse{
		Code:    CodeOK,
		Message: "成功",
		Data:    data,
		TraceID: traceID,
	}
}

func Fail(err *BizError, traceID string) HTTPResponse {
	if err == nil {
		err = ErrInternalError
	}
	return HTTPResponse{
		Code:    err.Code,
		Message: err.Msg,
		TraceID: traceID,
	}
}
