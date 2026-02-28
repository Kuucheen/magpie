package server

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsOnce sync.Once

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "magpie_http_requests_total",
			Help: "Total number of HTTP requests handled by the backend.",
		},
		[]string{"method", "route", "status"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "magpie_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)

	authFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "magpie_auth_failures_total",
			Help: "Total number of authentication failures.",
		},
		[]string{"reason"},
	)

	rateLimitBlocksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "magpie_rate_limit_blocks_total",
			Help: "Total number of blocked requests due to rate limiting.",
		},
		[]string{"scope"},
	)

	rotatingProxyErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "magpie_rotating_proxy_errors_total",
			Help: "Total rotating proxy API errors grouped by category.",
		},
		[]string{"category"},
	)
)

func initMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(
			httpRequestsTotal,
			httpRequestDurationSeconds,
			authFailuresTotal,
			rateLimitBlocksTotal,
			rotatingProxyErrorsTotal,
		)
	})
}

func metricsHandler() http.Handler {
	initMetrics()
	return promhttp.Handler()
}

func observeRequestMetrics(r *http.Request, statusCode int, duration time.Duration) {
	if r == nil {
		return
	}
	initMetrics()

	route := strings.TrimSpace(r.Pattern)
	if route == "" {
		route = r.URL.Path
	}
	status := strconv.Itoa(statusCode)

	httpRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
	httpRequestDurationSeconds.WithLabelValues(r.Method, route, status).Observe(duration.Seconds())
}

func recordAuthFailureMetric(reason string) {
	initMetrics()
	authFailuresTotal.WithLabelValues(normalizeMetricLabel(reason, "unknown")).Inc()
}

func recordRateLimitBlockMetric(scope string) {
	initMetrics()
	rateLimitBlocksTotal.WithLabelValues(normalizeMetricLabel(scope, "unknown")).Inc()
}

func recordRotatingProxyErrorMetric(category string) {
	initMetrics()
	rotatingProxyErrorsTotal.WithLabelValues(normalizeMetricLabel(category, "unknown")).Inc()
}

func normalizeMetricLabel(value string, fallback string) string {
	label := strings.TrimSpace(strings.ToLower(value))
	if label == "" {
		return fallback
	}
	return label
}
