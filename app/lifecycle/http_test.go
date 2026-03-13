package lifecycle

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHTTPMux_Healthz(t *testing.T) {
	mux := NewHTTPMux("gateway", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "service=gateway") {
		t.Fatalf("expected gateway in response, got %q", rec.Body.String())
	}
}

func TestNewHTTPMux_Readyz(t *testing.T) {
	mux := NewHTTPMux("user-service", nil, func(context.Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ready service=user-service") {
		t.Fatalf("unexpected response body %q", rec.Body.String())
	}
}

func TestNewHTTPMux_ReadyzFailure(t *testing.T) {
	mux := NewHTTPMux("product-service", nil, func(context.Context) error { return errors.New("mysql down") })
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "mysql down") {
		t.Fatalf("expected error in response, got %q", rec.Body.String())
	}
}
