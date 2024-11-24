package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RulesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clickhouse_alertmanager_processed_rules_total",
			Help: "The total number of processed rules",
		},
		[]string{"group", "rule"},
	)
	AlertsSent = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "clickhouse_alertmanager_alerts_sent_total",
			Help: "The total number of alerts sent to alertmanager",
		},
	)
	RulesActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "clickhouse_alertmanager_active_rules",
			Help: "The number of current active rules",
		},
	)
	QueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "clickhouse_alertmanager_query_duration_seconds",
			Help:    "The duration of clickhouse queries",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
	)
)

func ListenAndServe() {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":3030", nil)
}
