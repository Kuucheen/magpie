package domain

import "time"

// ProxyDailyCheckProxyBackfill tracks whether historical daily checks were backfilled for a proxy.
type ProxyDailyCheckProxyBackfill struct {
	ProxyID      uint64    `gorm:"primaryKey;autoIncrement:false"`
	BackfilledAt time.Time `gorm:"not null;autoCreateTime"`
}

func (ProxyDailyCheckProxyBackfill) TableName() string {
	return "proxy_daily_check_proxy_backfills"
}
