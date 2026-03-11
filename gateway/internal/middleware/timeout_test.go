package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"meshcart/app/common"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestRequestTimeout_ReturnsServiceBusyWhenHandlerRespectsContext(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := server.New(server.WithHostPorts(addr))
	h.Use(RequestTimeout(50 * time.Millisecond))
	h.GET("/slow", func(ctx context.Context, c *app.RequestContext) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
			c.String(http.StatusOK, "late")
		}
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

	resp, err := http.Get("http://" + addr + "/slow")
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected http 200, got %d", resp.StatusCode)
	}

	var body common.HTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Code != common.CodeInternalError {
		t.Fatalf("expected internal error code, got %d", body.Code)
	}
	if body.Message != common.ErrServiceBusy.Msg {
		t.Fatalf("expected service busy message, got %q", body.Message)
	}
}

func TestRequestTimeout_DoesNotOverrideExistingResponse(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := server.New(server.WithHostPorts(addr))
	h.Use(RequestTimeout(50 * time.Millisecond))
	h.GET("/handled", func(ctx context.Context, c *app.RequestContext) {
		<-ctx.Done()
		c.JSON(http.StatusOK, common.Fail(common.ErrServiceUnavailable, "trace-test"))
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

	resp, err := http.Get("http://" + addr + "/handled")
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	var body common.HTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Message != common.ErrServiceUnavailable.Msg {
		t.Fatalf("expected existing response to be preserved, got %q", body.Message)
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
