package metrics

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	dbOpenConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_open_connections",
			Help: "Current number of open database connections.",
		},
		[]string{"service"},
	)

	dbInUseConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_in_use_connections",
			Help: "Current number of in-use database connections.",
		},
		[]string{"service"},
	)

	dbIdleConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_idle_connections",
			Help: "Current number of idle database connections.",
		},
		[]string{"service"},
	)

	dbWaitCountTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_wait_count_total",
			Help: "Total number of waits for a database connection.",
		},
		[]string{"service"},
	)

	dbWaitDurationSecondsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_wait_duration_seconds_total",
			Help: "Total time blocked waiting for a database connection.",
		},
		[]string{"service"},
	)

	dbMaxIdleClosedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_max_idle_closed_total",
			Help: "Total number of connections closed due to SetMaxIdleConns.",
		},
		[]string{"service"},
	)

	dbMaxIdleTimeClosedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_max_idle_time_closed_total",
			Help: "Total number of connections closed due to SetConnMaxIdleTime.",
		},
		[]string{"service"},
	)

	dbMaxLifetimeClosedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "meshcart_db_max_lifetime_closed_total",
			Help: "Total number of connections closed due to SetConnMaxLifetime.",
		},
		[]string{"service"},
	)
)

func ObserveDBStats(service string, db *sql.DB) {
	register()
	if db == nil {
		return
	}

	stats := db.Stats()
	dbOpenConnections.WithLabelValues(service).Set(float64(stats.OpenConnections))
	dbInUseConnections.WithLabelValues(service).Set(float64(stats.InUse))
	dbIdleConnections.WithLabelValues(service).Set(float64(stats.Idle))
	dbWaitCountTotal.WithLabelValues(service).Set(float64(stats.WaitCount))
	dbWaitDurationSecondsTotal.WithLabelValues(service).Set(stats.WaitDuration.Seconds())
	dbMaxIdleClosedTotal.WithLabelValues(service).Set(float64(stats.MaxIdleClosed))
	dbMaxIdleTimeClosedTotal.WithLabelValues(service).Set(float64(stats.MaxIdleTimeClosed))
	dbMaxLifetimeClosedTotal.WithLabelValues(service).Set(float64(stats.MaxLifetimeClosed))
}

func StartDBStatsCollector(service string, db *sql.DB, interval time.Duration) func() {
	register()
	if db == nil {
		return func() {}
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}

	stop := make(chan struct{})
	ticker := time.NewTicker(interval)

	ObserveDBStats(service, db)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ObserveDBStats(service, db)
			case <-stop:
				return
			}
		}
	}()

	return func() {
		close(stop)
	}
}
