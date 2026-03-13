package lifecycle

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestRunTCPPreflight(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := RunTCPPreflight(ctx, PreflightCheck{
		Name:    "local-listener",
		Address: ln.Addr().String(),
	}); err != nil {
		t.Fatalf("expected preflight to pass, got %v", err)
	}
}

func TestRunTCPPreflightFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := RunTCPPreflight(ctx, PreflightCheck{
		Name:    "unreachable",
		Address: "127.0.0.1:1",
	})
	if err == nil {
		t.Fatal("expected preflight to fail")
	}
}
