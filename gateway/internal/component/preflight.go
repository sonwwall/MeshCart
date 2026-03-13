package component

import (
	"context"
	"strings"
	"time"

	"meshcart/app/lifecycle"
	logx "meshcart/app/log"
	"meshcart/gateway/config"

	"go.uber.org/zap"
)

func RunPreflight(cfg config.Config) error {
	timeout := cfg.Server.PreflightTimeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	checks := make([]lifecycle.PreflightCheck, 0, 1)
	if strings.EqualFold(cfg.UserRPC.DiscoveryType, "consul") || strings.EqualFold(cfg.CartRPC.DiscoveryType, "consul") || strings.EqualFold(cfg.ProductRPC.DiscoveryType, "consul") {
		checks = append(checks, lifecycle.PreflightCheck{
			Name:    "consul",
			Address: cfg.UserRPC.ConsulAddress,
		})
	}

	if len(checks) == 0 {
		logx.L(nil).Info("gateway preflight skipped", zap.String("reason", "no remote infra check required"))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := lifecycle.RunTCPPreflight(ctx, checks...); err != nil {
		return err
	}

	logx.L(nil).Info("gateway preflight passed", zap.Duration("timeout", timeout))
	return nil
}
