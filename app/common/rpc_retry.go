package common

import (
	"time"

	"github.com/cloudwego/kitex/pkg/retry"
)

const readRetryBackoff = 30 * time.Millisecond

// NewReadFailureRetryPolicy builds a conservative retry policy for idempotent read RPCs.
func NewReadFailureRetryPolicy(rpcTimeout time.Duration) *retry.FailurePolicy {
	policy := retry.NewFailurePolicyWithResultRetry(retry.AllErrorRetry())
	policy.WithMaxRetryTimes(1)
	policy.WithFixedBackOff(int(readRetryBackoff / time.Millisecond))
	policy.DisableChainRetryStop()
	if maxDuration := readRetryMaxDuration(rpcTimeout); maxDuration > 0 {
		policy.WithMaxDurationMS(uint32(maxDuration / time.Millisecond))
	}
	return policy
}

func readRetryMaxDuration(rpcTimeout time.Duration) time.Duration {
	if rpcTimeout <= 0 {
		return 0
	}
	maxDuration := rpcTimeout - 100*time.Millisecond
	if maxDuration < 100*time.Millisecond {
		return rpcTimeout
	}
	return maxDuration
}
