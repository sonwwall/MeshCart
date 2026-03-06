package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func PromHandler() http.Handler {
	register()
	return promhttp.Handler()
}
