package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type StopFunc func(context.Context) error

func RunUntilSignal(run func() error, stop StopFunc, timeout time.Duration) error {
	signalCtx, cancelSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelSignal()

	errCh := make(chan error, 1)
	go func() {
		errCh <- run()
	}()

	select {
	case err := <-errCh:
		return err
	case <-signalCtx.Done():
	}

	if stop != nil {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), timeout)
		defer cancelShutdown()

		if err := stop(shutdownCtx); err != nil {
			return err
		}

		select {
		case err := <-errCh:
			return err
		case <-shutdownCtx.Done():
			return fmt.Errorf("shutdown timed out after %s: %w", timeout, shutdownCtx.Err())
		}
	}

	return <-errCh
}
