package database

import (
	"context"
	"strings"
	"time"

	"magpie/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func EnqueueEmailOutbox(kind, toAddress, subject, body string, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = 4
	}

	return DB.Create(&domain.EmailOutbox{
		Kind:          strings.TrimSpace(kind),
		ToAddress:     strings.TrimSpace(toAddress),
		Subject:       subject,
		Body:          body,
		Status:        domain.EmailOutboxStatusPending,
		MaxAttempts:   maxAttempts,
		NextAttemptAt: time.Now().UTC(),
	}).Error
}

func ClaimPendingEmailOutbox(ctx context.Context, limit int, now time.Time) ([]domain.EmailOutbox, error) {
	if limit <= 0 {
		return nil, nil
	}

	var claimed []domain.EmailOutbox
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.
			Where("status = ? AND next_attempt_at <= ?", domain.EmailOutboxStatusPending, now.UTC()).
			Order("next_attempt_at ASC, id ASC").
			Limit(limit)

		if isPostgresDialect(tx) {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}

		if err := query.Find(&claimed).Error; err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(claimed))
		for _, msg := range claimed {
			ids = append(ids, msg.ID)
		}

		attemptTime := now.UTC()
		if err := tx.Model(&domain.EmailOutbox{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":          domain.EmailOutboxStatusProcessing,
				"attempts":        gorm.Expr("attempts + 1"),
				"last_attempt_at": attemptTime,
				"updated_at":      attemptTime,
			}).Error; err != nil {
			return err
		}

		for i := range claimed {
			claimed[i].Status = domain.EmailOutboxStatusProcessing
			claimed[i].Attempts++
			claimed[i].LastAttemptAt = &attemptTime
			claimed[i].UpdatedAt = attemptTime
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func MarkEmailOutboxSent(ctx context.Context, id uint, now time.Time) error {
	return DB.WithContext(ctx).Model(&domain.EmailOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     domain.EmailOutboxStatusSent,
			"sent_at":    now.UTC(),
			"last_error": "",
			"updated_at": now.UTC(),
		}).Error
}

func MarkEmailOutboxForRetry(ctx context.Context, id uint, nextAttemptAt time.Time, lastError string) error {
	return DB.WithContext(ctx).Model(&domain.EmailOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":          domain.EmailOutboxStatusPending,
			"next_attempt_at": nextAttemptAt.UTC(),
			"last_error":      truncateEmailOutboxError(lastError),
			"updated_at":      time.Now().UTC(),
		}).Error
}

func MarkEmailOutboxAbandoned(ctx context.Context, id uint, lastError string) error {
	return DB.WithContext(ctx).Model(&domain.EmailOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     domain.EmailOutboxStatusAbandoned,
			"last_error": truncateEmailOutboxError(lastError),
			"updated_at": time.Now().UTC(),
		}).Error
}

func RequeueStaleEmailOutboxMessages(ctx context.Context, staleBefore time.Time) (int64, error) {
	result := DB.WithContext(ctx).Model(&domain.EmailOutbox{}).
		Where("status = ? AND last_attempt_at IS NOT NULL AND last_attempt_at <= ?", domain.EmailOutboxStatusProcessing, staleBefore.UTC()).
		Updates(map[string]any{
			"status":          domain.EmailOutboxStatusPending,
			"next_attempt_at": time.Now().UTC(),
			"last_error":      "processing timeout",
			"updated_at":      time.Now().UTC(),
		})
	return result.RowsAffected, result.Error
}

func CountActiveEmailOutboxMessages(ctx context.Context) (int64, error) {
	var count int64
	err := DB.WithContext(ctx).
		Model(&domain.EmailOutbox{}).
		Where("status IN ?", []string{domain.EmailOutboxStatusPending, domain.EmailOutboxStatusProcessing}).
		Count(&count).Error
	return count, err
}

func truncateEmailOutboxError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 1000 {
		return value
	}
	return value[:1000]
}

func DeleteSentEmailOutbox(ctx context.Context, olderThan time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	if isPostgresDialect(DB) {
		query := `
			DELETE FROM email_outboxes
			WHERE id IN (
				SELECT id
				FROM email_outboxes
				WHERE status = ? AND sent_at IS NOT NULL AND sent_at <= ?
				ORDER BY sent_at ASC
				LIMIT ?
			)
		`
		result := DB.WithContext(ctx).Exec(query, domain.EmailOutboxStatusSent, olderThan.UTC(), limit)
		return result.RowsAffected, result.Error
	}

	var rows []domain.EmailOutbox
	if err := DB.WithContext(ctx).
		Select("id").
		Where("status = ? AND sent_at IS NOT NULL AND sent_at <= ?", domain.EmailOutboxStatusSent, olderThan.UTC()).
		Order("sent_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	result := DB.WithContext(ctx).Where("id IN ?", ids).Delete(&domain.EmailOutbox{})
	return result.RowsAffected, result.Error
}
