package domain

import "time"

// ProxyDailyCheck stores pre-aggregated check counts per proxy per UTC day.
type ProxyDailyCheck struct {
	ProxyID     uint64    `gorm:"primaryKey;autoIncrement:false"`
	Day         time.Time `gorm:"type:date;primaryKey"`
	ChecksCount int64     `gorm:"not null;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (ProxyDailyCheck) TableName() string {
	return "proxy_daily_checks"
}
