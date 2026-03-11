package logicutil

import (
	"context"
	"errors"
	"strings"

	"meshcart/app/common"
)

func MapRPCError(err error) *common.BizError {
	if err == nil {
		return nil
	}
	if IsTimeoutError(err) {
		return common.ErrServiceBusy
	}
	if IsServiceUnavailableError(err) {
		return common.ErrServiceUnavailable
	}
	return common.ErrInternalError
}

func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func IsServiceUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no available") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "broken pipe")
}
