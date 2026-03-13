package lifecycle

import (
	"context"
	"fmt"
	"net"
	"time"
)

type PreflightCheck struct {
	Name    string
	Address string
}

func RunTCPPreflight(ctx context.Context, checks ...PreflightCheck) error {
	for _, check := range checks {
		if check.Address == "" {
			return fmt.Errorf("preflight %s: empty address", check.Name)
		}
		if err := CheckTCPAddress(ctx, check.Name, check.Address); err != nil {
			return err
		}
	}
	return nil
}

func CheckTCPAddress(ctx context.Context, name, address string) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("preflight %s failed: dial tcp %s: %w", name, address, err)
	}
	_ = conn.Close()
	return nil
}

func TimeoutFromMS(ms int, fallback time.Duration) time.Duration {
	if ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}
