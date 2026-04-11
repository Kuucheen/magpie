package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
)

const (
	emailDeliveryLockKey              = "magpie:leader:email_delivery"
	envEmailOutboxPollInterval        = "EMAIL_OUTBOX_POLL_INTERVAL"
	envEmailOutboxPollIntervalSeconds = "EMAIL_OUTBOX_POLL_INTERVAL_SECONDS"
	envEmailOutboxBatchSize           = "EMAIL_OUTBOX_BATCH_SIZE"
	envEmailRetryBaseSeconds          = "EMAIL_RETRY_BASE_SECONDS"
	envEmailProcessingTimeout         = "EMAIL_PROCESSING_TIMEOUT"
	envEmailProcessingTimeoutSeconds  = "EMAIL_PROCESSING_TIMEOUT_SECONDS"
	envEmailOutboxRetentionHours      = "EMAIL_OUTBOX_RETENTION_HOURS"
	defaultEmailOutboxPollInterval    = 5 * time.Second
	defaultEmailOutboxBatchSize       = 10
	defaultEmailRetryBaseDelay        = 5 * time.Second
	defaultEmailProcessingTimeout     = 15 * time.Minute
	defaultEmailOutboxRetention       = 24 * time.Hour
	emailOutboxCleanupBatchSize       = 500
	maxEmailRetryDelay                = 30 * time.Minute
)

func StartEmailDeliveryRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	err := support.RunWithLeader(ctx, emailDeliveryLockKey, support.DefaultLeadershipTTL, func(leaderCtx context.Context) {
		runEmailDeliveryLoop(leaderCtx)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Email delivery routine stopped", "error", err)
	}
}

func runEmailDeliveryLoop(ctx context.Context) {
	ticker := time.NewTicker(resolveEmailOutboxPollInterval())
	defer ticker.Stop()

	runEmailDeliveryPass(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runEmailDeliveryPass(ctx)
		}
	}
}

func runEmailDeliveryPass(ctx context.Context) {
	processingTimeout := resolveEmailProcessingTimeout()
	if processingTimeout > 0 {
		if recovered, err := database.RequeueStaleEmailOutboxMessages(ctx, time.Now().UTC().Add(-processingTimeout)); err != nil {
			log.Error("Failed to requeue stale email outbox messages", "error", err)
		} else if recovered > 0 {
			log.Warn("Recovered stale email outbox messages", "count", recovered)
		}
	}

	batchSize := resolveEmailOutboxBatchSize()
	messages, err := database.ClaimPendingEmailOutbox(ctx, batchSize, time.Now().UTC())
	if err != nil {
		log.Error("Failed to claim email outbox messages", "error", err)
		return
	}
	if len(messages) == 0 {
		updateEmailQueueDepthMetric(ctx)
		cleanupSentEmailOutbox(ctx)
		return
	}

	var wg sync.WaitGroup
	for _, msg := range messages {
		msg := msg
		wg.Add(1)
		go func() {
			defer wg.Done()
			deliverEmailOutboxMessage(ctx, msg)
		}()
	}
	wg.Wait()

	updateEmailQueueDepthMetric(ctx)
	cleanupSentEmailOutbox(ctx)
}

func deliverEmailOutboxMessage(ctx context.Context, msg domain.EmailOutbox) {
	cfg, err := support.ReadEmailConfig()
	if err != nil {
		scheduleEmailRetry(ctx, msg, "config_error", err)
		return
	}

	if err := support.SendEmail(cfg, msg.ToAddress, msg.Subject, msg.Body); err != nil {
		scheduleEmailRetry(ctx, msg, "send_failed", err)
		return
	}

	support.RecordEmailDeliveryMetric(msg.Kind, "sent")
	if err := database.MarkEmailOutboxSent(ctx, msg.ID, time.Now().UTC()); err != nil {
		log.Error("Failed to mark email outbox message as sent", "id", msg.ID, "kind", msg.Kind, "error", err)
	}
}

func scheduleEmailRetry(ctx context.Context, msg domain.EmailOutbox, metricResult string, cause error) {
	support.RecordEmailDeliveryMetric(msg.Kind, metricResult)
	lastError := cause.Error()

	if msg.Attempts >= msg.MaxAttempts {
		support.RecordEmailDeliveryMetric(msg.Kind, "abandoned")
		if err := database.MarkEmailOutboxAbandoned(ctx, msg.ID, lastError); err != nil {
			log.Error("Failed to abandon email outbox message", "id", msg.ID, "kind", msg.Kind, "error", err)
		}
		return
	}

	nextAttempt := time.Now().UTC().Add(resolveEmailRetryDelay(msg.Attempts + 1))
	support.RecordEmailDeliveryMetric(msg.Kind, "retry_scheduled")
	if err := database.MarkEmailOutboxForRetry(ctx, msg.ID, nextAttempt, lastError); err != nil {
		log.Error("Failed to mark email outbox message for retry", "id", msg.ID, "kind", msg.Kind, "error", err)
	}
}

func cleanupSentEmailOutbox(ctx context.Context) {
	retention := resolveEmailOutboxRetention()
	if retention <= 0 {
		return
	}
	olderThan := time.Now().UTC().Add(-retention)
	_, err := database.DeleteSentEmailOutbox(ctx, olderThan, emailOutboxCleanupBatchSize)
	if err != nil {
		log.Error("Failed to cleanup sent email outbox rows", "error", err)
	}
}

func updateEmailQueueDepthMetric(ctx context.Context) {
	count, err := database.CountActiveEmailOutboxMessages(ctx)
	if err != nil {
		log.Error("Failed to count active email outbox rows", "error", err)
		return
	}
	support.SetEmailDeliveryQueueDepth(int(count))
}

func resolveEmailOutboxPollInterval() time.Duration {
	if raw := support.GetEnv(envEmailOutboxPollInterval, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Warn("Invalid EMAIL_OUTBOX_POLL_INTERVAL value, falling back to seconds env", "value", raw)
	}

	seconds := support.GetEnvInt(envEmailOutboxPollIntervalSeconds, int(defaultEmailOutboxPollInterval/time.Second))
	if seconds <= 0 {
		seconds = int(defaultEmailOutboxPollInterval / time.Second)
	}
	return time.Duration(seconds) * time.Second
}

func resolveEmailOutboxBatchSize() int {
	size := support.GetEnvInt(envEmailOutboxBatchSize, defaultEmailOutboxBatchSize)
	if size <= 0 {
		return defaultEmailOutboxBatchSize
	}
	return size
}

func resolveEmailProcessingTimeout() time.Duration {
	if raw := support.GetEnv(envEmailProcessingTimeout, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Warn("Invalid EMAIL_PROCESSING_TIMEOUT value, falling back to seconds env", "value", raw)
	}

	seconds := support.GetEnvInt(envEmailProcessingTimeoutSeconds, int(defaultEmailProcessingTimeout/time.Second))
	if seconds <= 0 {
		seconds = int(defaultEmailProcessingTimeout / time.Second)
	}
	return time.Duration(seconds) * time.Second
}

func resolveEmailOutboxRetention() time.Duration {
	hours := support.GetEnvInt(envEmailOutboxRetentionHours, int(defaultEmailOutboxRetention/time.Hour))
	if hours <= 0 {
		return defaultEmailOutboxRetention
	}
	return time.Duration(hours) * time.Hour
}

func resolveEmailRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	baseSeconds := support.GetEnvInt(envEmailRetryBaseSeconds, int(defaultEmailRetryBaseDelay/time.Second))
	if baseSeconds <= 0 {
		baseSeconds = int(defaultEmailRetryBaseDelay / time.Second)
	}

	delay := time.Duration(baseSeconds) * time.Second
	for i := 1; i < attempt; i++ {
		if delay >= maxEmailRetryDelay/2 {
			return maxEmailRetryDelay
		}
		delay *= 2
	}
	if delay > maxEmailRetryDelay {
		return maxEmailRetryDelay
	}
	return delay
}
