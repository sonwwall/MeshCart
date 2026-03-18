package common

import (
	"testing"
	"time"
)

func TestNewReadFailureRetryPolicy(t *testing.T) {
	policy := NewReadFailureRetryPolicy(2 * time.Second)
	if policy == nil {
		t.Fatal("expected non-nil policy")
	}
	if policy.StopPolicy.MaxRetryTimes != 1 {
		t.Fatalf("expected max retry times 1, got %d", policy.StopPolicy.MaxRetryTimes)
	}
	if policy.BackOffPolicy == nil {
		t.Fatal("expected backoff policy")
	}
	if policy.StopPolicy.MaxDurationMS != 1900 {
		t.Fatalf("expected max duration 1900ms, got %d", policy.StopPolicy.MaxDurationMS)
	}
	if policy.ShouldResultRetry == nil || policy.ShouldResultRetry.ErrorRetryWithCtx == nil {
		t.Fatal("expected error retry predicate")
	}
}
