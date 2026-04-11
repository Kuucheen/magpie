package maintenance

import (
	"context"
	"errors"
	"time"

	"magpie/internal/database"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
)

const (
	envPasswordResetCleanupInterval        = "PASSWORD_RESET_CLEANUP_INTERVAL"
	envPasswordResetCleanupIntervalMinutes = "PASSWORD_RESET_CLEANUP_INTERVAL_MINUTES"
	defaultPasswordResetCleanupMinutes     = 60
	passwordResetCleanupLockKey            = "magpie:leader:password_reset_cleanup"
)

func StartPasswordResetCleanupRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	err := support.RunWithLeader(ctx, passwordResetCleanupLockKey, support.DefaultLeadershipTTL, func(leaderCtx context.Context) {
		runPasswordResetCleanupLoop(leaderCtx)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Password reset cleanup routine stopped", "error", err)
	}
}

func runPasswordResetCleanupLoop(ctx context.Context) {
	interval := resolvePasswordResetCleanupInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runPasswordResetCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runPasswordResetCleanup(ctx)
		}
	}
}

func resolvePasswordResetCleanupInterval() time.Duration {
	if raw := support.GetEnv(envPasswordResetCleanupInterval, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Warn("Invalid PASSWORD_RESET_CLEANUP_INTERVAL value, falling back to minutes env", "value", raw)
	}

	minutes := support.GetEnvInt(envPasswordResetCleanupIntervalMinutes, defaultPasswordResetCleanupMinutes)
	if minutes <= 0 {
		minutes = defaultPasswordResetCleanupMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func runPasswordResetCleanup(ctx context.Context) {
	removed, err := database.DeleteExpiredPasswordResetTokens(ctx, time.Now().UTC())
	if err != nil {
		log.Error("Failed to cleanup expired password reset tokens", "error", err)
		return
	}
	if removed > 0 {
		log.Info("Expired password reset tokens removed", "count", removed)
	}
}
