package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	bizErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meshcart_biz_errors_total",
			Help: "Total number of business errors grouped by service, module, action, stage, and code.",
		},
		[]string{"service", "module", "action", "stage", "code"},
	)
)

func ObserveBizError(service, module, action, stage string, code int32) {
	register()
	codeLabel := strconv.FormatInt(int64(code), 10)
	bizErrorsTotal.WithLabelValues(service, module, action, stage, codeLabel).Inc()
}
