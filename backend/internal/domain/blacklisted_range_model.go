package domain

// BlacklistedRange stores IPv4 ranges (CIDRs expanded to start/end) for blacklist enforcement.
type BlacklistedRange struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`

	// CIDR holds the normalized network string (e.g. 192.0.2.0/24).
	CIDR   string `gorm:"column:cidr;type:cidr;uniqueIndex"`
	Source string `gorm:"size:512;not null;default:''"`

	// Computed bounds used in-memory; not persisted.
	StartIP uint32 `gorm:"-"`
	EndIP   uint32 `gorm:"-"`
}
