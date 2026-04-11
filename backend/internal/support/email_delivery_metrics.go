package support

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	emailDeliveryMetricsOnce sync.Once

	emailDeliveryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "magpie_email_delivery_total",
			Help: "Outbound email delivery attempts grouped by kind and result.",
		},
		[]string{"kind", "result"},
	)

	emailDeliveryQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "magpie_email_delivery_queue_depth",
			Help: "Current number of queued outbound emails.",
		},
	)
)

func initEmailDeliveryMetrics() {
	emailDeliveryMetricsOnce.Do(func() {
		prometheus.MustRegister(
			emailDeliveryTotal,
			emailDeliveryQueueDepth,
		)
	})
}

func RecordEmailDeliveryMetric(kind, result string) {
	initEmailDeliveryMetrics()
	emailDeliveryTotal.WithLabelValues(normalizeEmailMetricLabel(kind), normalizeEmailMetricLabel(result)).Inc()
}

func SetEmailDeliveryQueueDepth(depth int) {
	initEmailDeliveryMetrics()
	emailDeliveryQueueDepth.Set(float64(depth))
}

func normalizeEmailMetricLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	return value
}
