package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	inventoryReservationRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meshcart_inventory_reservation_requests_total",
			Help: "Total number of inventory reservation requests grouped by service, action, outcome, and reason.",
		},
		[]string{"service", "action", "outcome", "reason"},
	)
	inventoryReservationDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "meshcart_inventory_reservation_duration_seconds",
			Help:    "Inventory reservation request duration grouped by service, action, and outcome.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "action", "outcome"},
	)
)

func ObserveInventoryReservation(service, action, outcome, reason string, duration time.Duration) {
	register()
	inventoryReservationRequestsTotal.WithLabelValues(service, action, outcome, reason).Inc()
	inventoryReservationDurationSeconds.WithLabelValues(service, action, outcome).Observe(duration.Seconds())
}
