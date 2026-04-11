package domain

import "time"

const (
	EmailOutboxStatusPending    = "pending"
	EmailOutboxStatusProcessing = "processing"
	EmailOutboxStatusSent       = "sent"
	EmailOutboxStatusAbandoned  = "abandoned"
)

type EmailOutbox struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"`
	Kind          string    `gorm:"not null;size:64;index"`
	ToAddress     string    `gorm:"not null;size:255"`
	Subject       string    `gorm:"not null;size:255"`
	Body          string    `gorm:"not null;type:text"`
	Status        string    `gorm:"not null;size:32;index"`
	Attempts      int       `gorm:"not null;default:0"`
	MaxAttempts   int       `gorm:"not null;default:4"`
	LastError     string    `gorm:"size:1000"`
	NextAttemptAt time.Time `gorm:"not null;index"`
	LastAttemptAt *time.Time
	SentAt        *time.Time
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}
