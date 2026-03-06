package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	rpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meshcart_rpc_requests_total",
			Help: "Total number of RPC requests.",
		},
		[]string{"service", "method", "code"},
	)

	rpcRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "meshcart_rpc_request_duration_seconds",
			Help:    "RPC request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)

	rpcErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meshcart_rpc_errors_total",
			Help: "Total number of RPC requests with business error code != 0.",
		},
		[]string{"service", "method", "code"},
	)
)

func observeRPC(service, method string, code int32, duration time.Duration) {
	register()

	codeLabel := strconv.FormatInt(int64(code), 10)
	rpcRequestsTotal.WithLabelValues(service, method, codeLabel).Inc()
	rpcRequestDurationSeconds.WithLabelValues(service, method).Observe(duration.Seconds())
	if code != 0 {
		rpcErrorsTotal.WithLabelValues(service, method, codeLabel).Inc()
	}
}

func ObserveRPC(service, method string, code int32, duration time.Duration) {
	observeRPC(service, method, code, duration)
}
