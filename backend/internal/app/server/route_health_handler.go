package server

import (
	"context"
	"net/http"
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
	redisClient, err := support.GetRedisClient()
	if err != nil {
		if support.GetEnvBool(envReadyzAllowRedisDegraded, false) {
			return probeComponent{Status: componentStatusDegraded, Details: "redis unavailable"}
		}
		return probeComponent{Status: componentStatusDown, Details: "redis unavailable"}
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		if support.GetEnvBool(envReadyzAllowRedisDegraded, false) {
			return probeComponent{Status: componentStatusDegraded, Details: "redis ping failed"}
		}
		return probeComponent{Status: componentStatusDown, Details: "redis ping failed"}
	}

	return probeComponent{Status: componentStatusUp}
}

func checkStartupBootstrapComponent() probeComponent {
	if bootstrap.StartupQueueBootstrapCompleted() {
		return probeComponent{Status: componentStatusUp}
	}

	return probeComponent{Status: componentStatusStarting, Details: "startup queue bootstrap still running"}
}
