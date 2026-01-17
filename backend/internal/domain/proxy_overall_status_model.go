package domain

import "time"

type ProxyOverallStatus struct {
	ProxyID       uint64    `gorm:"primaryKey"`
	OverallAlive  bool      `gorm:"not null;index"`
	LastCheckedAt time.Time `gorm:"index"`

	// Relationships
	Proxy Proxy `gorm:"foreignKey:ProxyID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
