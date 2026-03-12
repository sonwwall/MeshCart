package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"meshcart/app/common"
	gatewayconfig "meshcart/gateway/config"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestRateLimit_ReturnsTooManyRequestsWhenBucketExhausted(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := server.New(server.WithHostPorts(addr))

	store := NewRateLimitStore(gatewayconfig.RateLimitConfig{
		EntryTTL:        time.Minute,
		CleanupInterval: time.Minute,
	})
	h.GET("/limited", RateLimit(store, RateLimitRule{RatePerSecond: 1, Burst: 1}, IPRouteKey), func(_ context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, common.Success(map[string]string{"ok": "1"}, "trace-test"))
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

	first := doJSONRequest(t, http.MethodGet, "http://"+addr+"/limited", "")
	if first.Code != common.CodeOK {
		t.Fatalf("expected first request success, got code=%d message=%q", first.Code, first.Message)
	}

	second := doJSONRequest(t, http.MethodGet, "http://"+addr+"/limited", "")
	if second.Code != common.CodeTooManyReq {
		t.Fatalf("expected rate limit code, got code=%d message=%q", second.Code, second.Message)
	}
	if second.Message != common.ErrTooManyRequests.Msg {
		t.Fatalf("expected rate limit message, got %q", second.Message)
	}
}

func TestRateLimit_DifferentKeysHaveIndependentBuckets(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := server.New(server.WithHostPorts(addr))

	store := NewRateLimitStore(gatewayconfig.RateLimitConfig{
		EntryTTL:        time.Minute,
		CleanupInterval: time.Minute,
	})
	h.GET("/limited", RateLimit(store, RateLimitRule{RatePerSecond: 1, Burst: 1}, func(_ context.Context, c *app.RequestContext) (string, bool) {
		return string(c.Request.Header.Peek("X-Rate-Key")), true
	}), func(_ context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, common.Success(map[string]string{"ok": "1"}, "trace-test"))
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

	firstA := doJSONRequest(t, http.MethodGet, "http://"+addr+"/limited", "A")
	if firstA.Code != common.CodeOK {
		t.Fatalf("expected first A request success, got code=%d", firstA.Code)
	}
	firstB := doJSONRequest(t, http.MethodGet, "http://"+addr+"/limited", "B")
	if firstB.Code != common.CodeOK {
		t.Fatalf("expected first B request success, got code=%d", firstB.Code)
	}
	secondA := doJSONRequest(t, http.MethodGet, "http://"+addr+"/limited", "A")
	if secondA.Code != common.CodeTooManyReq {
		t.Fatalf("expected second A request limited, got code=%d", secondA.Code)
	}
}

func TestRateLimit_IPKeySharesBucketAcrossRoutes(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := server.New(server.WithHostPorts(addr))

	store := NewRateLimitStore(gatewayconfig.RateLimitConfig{
		EntryTTL:        time.Minute,
		CleanupInterval: time.Minute,
	})
	globalMiddleware := RateLimit(store, RateLimitRule{RatePerSecond: 1, Burst: 1}, IPKey)
	h.GET("/route-a", globalMiddleware, func(_ context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, common.Success(map[string]string{"route": "a"}, "trace-test"))
	})
	h.GET("/route-b", globalMiddleware, func(_ context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, common.Success(map[string]string{"route": "b"}, "trace-test"))
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

	first := doJSONRequest(t, http.MethodGet, "http://"+addr+"/route-a", "")
	if first.Code != common.CodeOK {
		t.Fatalf("expected first request success, got code=%d", first.Code)
	}

	second := doJSONRequest(t, http.MethodGet, "http://"+addr+"/route-b", "")
	if second.Code != common.CodeTooManyReq {
		t.Fatalf("expected second route request to share same ip bucket, got code=%d", second.Code)
	}
}

func TestRateLimitStore_CleansExpiredEntries(t *testing.T) {
	now := time.Unix(100, 0)
	store := NewRateLimitStore(gatewayconfig.RateLimitConfig{
		EntryTTL:        10 * time.Second,
		CleanupInterval: 5 * time.Second,
	})
	store.now = func() time.Time { return now }
	store.lastCleanup = now

	rule := RateLimitRule{RatePerSecond: 1, Burst: 1}
	if !store.Allow("old", rule) {
		t.Fatalf("expected initial request allowed")
	}
	if len(store.limiters) != 1 {
		t.Fatalf("expected 1 limiter entry, got %d", len(store.limiters))
	}

	now = now.Add(20 * time.Second)
	if !store.Allow("new", rule) {
		t.Fatalf("expected new key request allowed")
	}

	if _, ok := store.limiters["old"]; ok {
		t.Fatalf("expected expired limiter entry to be cleaned up")
	}
	if _, ok := store.limiters["new"]; !ok {
		t.Fatalf("expected active limiter entry to remain")
	}
}

func doJSONRequest(t *testing.T, method, url, rateKey string) common.HTTPResponse {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if rateKey != "" {
		req.Header.Set("X-Rate-Key", rateKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	var body common.HTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}
