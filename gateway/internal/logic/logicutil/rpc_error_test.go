package logicutil

import (
	"context"
	"errors"
	"testing"

	"meshcart/app/common"
)

func TestMapRPCError_Timeout(t *testing.T) {
	bizErr := MapRPCError(context.DeadlineExceeded)
	if bizErr != common.ErrServiceBusy {
		t.Fatalf("expected ErrServiceBusy, got %+v", bizErr)
	}
}

func TestMapRPCError_ServiceUnavailable(t *testing.T) {
	bizErr := MapRPCError(errors.New("dial tcp 127.0.0.1:8888: connect: connection refused"))
	if bizErr != common.ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable, got %+v", bizErr)
	}
}

func TestMapRPCError_FallbackInternalError(t *testing.T) {
	bizErr := MapRPCError(errors.New("unexpected rpc failure"))
	if bizErr != common.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %+v", bizErr)
	}
}
