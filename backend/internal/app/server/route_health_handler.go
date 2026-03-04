package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"magpie/internal/app/bootstrap"
	"magpie/internal/app/version"
	"magpie/internal/database"
	"magpie/internal/support"
)

const (
	envReadyzAllowRedisDegraded = "READYZ_ALLOW_REDIS_DEGRADED"
	componentStatusUp           = "up"
	componentStatusDown         = "down"
	componentStatusStarting     = "starting"
	componentStatusDegraded     = "degraded"
)

type probeComponent struct {
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

type probeResponse struct {
	Status      string                    `json:"status"`
	Timestamp   string                    `json:"timestamp"`
	RequestID   string                    `json:"request_id,omitempty"`
	InstanceID  string                    `json:"instance_id"`
	Version     string                    `json:"version"`
	BuiltAt     string                    `json:"built_at"`
	Degraded    bool                      `json:"degraded"`
	Components  map[string]probeComponent `json:"components"`
	StartupDone bool                      `json:"startup_queue_bootstrap_completed"`
}

func healthz(w http.ResponseWriter, r *http.Request) {
	build := version.Get()

	writeJSON(w, http.StatusOK, probeResponse{
		Status:      "ok",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		RequestID:   requestIDFromRequest(r),
		InstanceID:  support.GetInstanceID(),
		Version:     build.BuildVersion,
		BuiltAt:     build.BuiltAt,
		Degraded:    false,
		StartupDone: bootstrap.StartupQueueBootstrapCompleted(),
		Components: map[string]probeComponent{
			"process": {
				Status: componentStatusUp,
			},
		},
	})
}

func readyz(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}

	components := map[string]probeComponent{
		"database":                checkDatabaseComponent(ctx),
		"redis":                   checkRedisComponent(ctx),
		"startup_queue_bootstrap": checkStartupBootstrapComponent(),
	}

	redisRequired := !support.GetEnvBool(envReadyzAllowRedisDegraded, false)
	ready := components["database"].Status == componentStatusUp &&
		components["startup_queue_bootstrap"].Status == componentStatusUp &&
		(components["redis"].Status == componentStatusUp || !redisRequired)

	degraded := components["redis"].Status != componentStatusUp
	if redisRequired && degraded {
		degraded = false
	}

	responseStatus := http.StatusOK
	overallStatus := "ready"
	if !ready {
		responseStatus = http.StatusServiceUnavailable
		overallStatus = "not_ready"
	}
	if ready && degraded {
		overallStatus = "degraded"
	}

	build := version.Get()
	writeJSON(w, responseStatus, probeResponse{
		Status:      overallStatus,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		RequestID:   requestIDFromRequest(r),
		InstanceID:  support.GetInstanceID(),
		Version:     build.BuildVersion,
		BuiltAt:     build.BuiltAt,
		Degraded:    degraded,
		Components:  components,
		StartupDone: bootstrap.StartupQueueBootstrapCompleted(),
	})
}

func checkDatabaseComponent(ctx context.Context) probeComponent {
	if database.DB == nil {
		return probeComponent{Status: componentStatusDown, Details: "database connection is not initialized"}
	}

	sqlDB, err := database.DB.DB()
	if err != nil {
		return probeComponent{Status: componentStatusDown, Details: "database sql handle unavailable"}
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return probeComponent{Status: componentStatusDown, Details: "database ping failed"}
	}

	return probeComponent{Status: componentStatusUp}
}

func checkRedisComponent(ctx context.Context) probeComponent {
	status := support.GetRedisClientStatus()
	redisClient, err := support.GetRedisClient()
	if err != nil {
		status = support.GetRedisClientStatus()
		details := formatRedisComponentDetails(status, err)
		if support.GetEnvBool(envReadyzAllowRedisDegraded, false) {
			return probeComponent{Status: componentStatusDegraded, Details: details}
		}
		return probeComponent{Status: componentStatusDown, Details: details}
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		status = support.GetRedisClientStatus()
		details := formatRedisComponentDetails(status, fmt.Errorf("redis ping failed: %w", err))
		if support.GetEnvBool(envReadyzAllowRedisDegraded, false) {
			return probeComponent{Status: componentStatusDegraded, Details: details}
		}
		return probeComponent{Status: componentStatusDown, Details: details}
	}

	status = support.GetRedisClientStatus()
	return probeComponent{Status: componentStatusUp, Details: formatRedisComponentDetails(status, nil)}
}

func checkStartupBootstrapComponent() probeComponent {
	if bootstrap.StartupQueueBootstrapCompleted() {
		return probeComponent{Status: componentStatusUp}
	}

	return probeComponent{Status: componentStatusStarting, Details: "startup queue bootstrap still running"}
}

func formatRedisComponentDetails(status support.RedisClientStatus, err error) string {
	details := fmt.Sprintf("mode=%s", status.Mode)
	if errorClass := classifyRedisError(err, status.LastError); errorClass != "" {
		details += fmt.Sprintf("; error_class=%s", errorClass)
	}
	if status.RetryAfter > 0 {
		details += fmt.Sprintf("; retry_after=%s", status.RetryAfter)
	}
	return details
}

func classifyRedisError(err error, lastErr string) string {
	msg := strings.ToLower(strings.TrimSpace(lastErr))
	if err != nil {
		msg = strings.ToLower(strings.TrimSpace(err.Error()))
	}
	if msg == "" {
		return ""
	}

	switch {
	case strings.Contains(msg, "redis reconnect deferred"):
		return "reconnect_deferred"
	case strings.Contains(msg, "failed to parse redisurl"),
		strings.Contains(msg, "missing redis_master_name"),
		strings.Contains(msg, "missing redis_sentinel_addrs"),
		strings.Contains(msg, "invalid redis_mode"):
		return "config_invalid"
	case strings.Contains(msg, "redis ping failed"):
		return "ping_failed"
	case strings.Contains(msg, "failed to connect to redis"):
		return "connect_failed"
	default:
		return "unavailable"
	}
}
