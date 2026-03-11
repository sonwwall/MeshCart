package component

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	gatewayconfig "meshcart/gateway/config"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestNewGatewayServer_AppliesHTTPTimeouts(t *testing.T) {
	cfg := gatewayconfig.Config{
		Metrics: gatewayconfig.MetricsConfig{
			Addr: ":0",
			Path: "/metrics",
		},
		Server: gatewayconfig.ServerConfig{
			Addr:         "127.0.0.1:0",
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 4 * time.Second,
			IdleTimeout:  45 * time.Second,
		},
	}

	h := NewGatewayServer(cfg, &svc.ServiceContext{})
	opt := h.GetOptions()

	if opt.ReadTimeout != cfg.Server.ReadTimeout {
		t.Fatalf("expected read timeout %s, got %s", cfg.Server.ReadTimeout, opt.ReadTimeout)
	}
	if opt.WriteTimeout != cfg.Server.WriteTimeout {
		t.Fatalf("expected write timeout %s, got %s", cfg.Server.WriteTimeout, opt.WriteTimeout)
	}
	if opt.IdleTimeout != cfg.Server.IdleTimeout {
		t.Fatalf("expected idle timeout %s, got %s", cfg.Server.IdleTimeout, opt.IdleTimeout)
	}
}

func TestHTTPReadTimeoutReturnsFrameworkBadRequest(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := server.New(
		server.WithHostPorts(addr),
		server.WithReadTimeout(100*time.Millisecond),
		server.WithWriteTimeout(time.Second),
		server.WithIdleTimeout(time.Second),
	)
	h.GET("/echo", func(_ context.Context, c *app.RequestContext) {
		c.String(200, "ok")
	})

	go func() {
		_ = h.Run()
	}()
	waitForServer(t, addr)

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = h.Shutdown(shutdownCtx)
	})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("GET /echo HTTP/1.1\r\nHost: test\r\n")); err != nil {
		t.Fatalf("write partial request: %v", err)
	}

	time.Sleep(250 * time.Millisecond)
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	resp, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read timeout response: %v", err)
	}

	t.Logf("http read timeout response: %q", string(resp))

	if !strings.Contains(string(resp), "400 Bad Request") {
		t.Fatalf("expected framework bad request response, got %q", string(resp))
	}
	if !strings.Contains(string(resp), "Error when parsing request") {
		t.Fatalf("expected framework parse error body, got %q", string(resp))
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	defer ln.Close()

	return ln.Addr().String()
}

func waitForServer(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("server did not start listening at %s", addr)
}
