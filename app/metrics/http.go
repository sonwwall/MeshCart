package metrics

import (
	"bytes"
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

var (
	registerOnce sync.Once

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meshcart_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"service", "method", "path", "status"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "meshcart_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)

	httpRequestErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meshcart_http_request_errors_total",
			Help: "Total number of HTTP requests with status >= 400.",
		},
		[]string{"service", "method", "path", "status"},
	)
)

func register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(httpRequestsTotal)
		prometheus.MustRegister(httpRequestDurationSeconds)
		prometheus.MustRegister(httpRequestErrorsTotal)
		prometheus.MustRegister(rpcRequestsTotal)
		prometheus.MustRegister(rpcRequestDurationSeconds)
		prometheus.MustRegister(rpcErrorsTotal)
		prometheus.MustRegister(dbOpenConnections)
		prometheus.MustRegister(dbInUseConnections)
		prometheus.MustRegister(dbIdleConnections)
		prometheus.MustRegister(dbWaitCountTotal)
		prometheus.MustRegister(dbWaitDurationSecondsTotal)
		prometheus.MustRegister(dbMaxIdleClosedTotal)
		prometheus.MustRegister(dbMaxIdleTimeClosedTotal)
		prometheus.MustRegister(dbMaxLifetimeClosedTotal)
		prometheus.MustRegister(bizErrorsTotal)
	})
}

func HTTPMiddleware(service string) app.HandlerFunc {
	register()
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		method := string(c.Method())
		path := string(c.Path())

		c.Next(ctx)

		statusCode := c.Response.StatusCode()
		status := strconv.Itoa(statusCode)
		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(service, method, path, status).Inc()
		httpRequestDurationSeconds.WithLabelValues(service, method, path).Observe(duration)
		if statusCode >= 400 {
			httpRequestErrorsTotal.WithLabelValues(service, method, path, status).Inc()
		}
	}
}

func Handler() app.HandlerFunc {
	register()
	return func(_ context.Context, c *app.RequestContext) {
		metricFamilies, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			c.SetStatusCode(500)
			_, _ = c.WriteString("gather metrics failed")
			return
		}

		var buf bytes.Buffer
		for _, mf := range metricFamilies {
			_, _ = expfmt.MetricFamilyToText(&buf, mf)
		}

		c.Response.Header.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		c.SetStatusCode(200)
		_, _ = c.Write(buf.Bytes())
	}
}
