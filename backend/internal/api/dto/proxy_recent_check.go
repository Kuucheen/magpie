package dto

import "time"

type ProxyRecentCheck struct {
	ID           uint64
	IP           string
	Port         uint16
	ResponseTime uint16
	Alive        bool
	LatestCheck  time.Time
}
