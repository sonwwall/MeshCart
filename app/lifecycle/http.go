package lifecycle

import (
	"context"
	"fmt"
	"net/http"
)

type ProbeFunc func(context.Context) error

func NewHTTPMux(service string, metricsHandler http.Handler, readiness ProbeFunc) *http.ServeMux {
	mux := http.NewServeMux()
	if metricsHandler != nil {
		mux.Handle("/metrics", metricsHandler)
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "ok service=%s\n", service)
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if readiness != nil {
			if err := readiness(r.Context()); err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = fmt.Fprintf(w, "not ready service=%s err=%v\n", service, err)
				return
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "ready service=%s\n", service)
	})

	return mux
}
