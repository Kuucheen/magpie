package dto

import "time"

type ScrapeSiteReputationBreakdown struct {
	Good    uint `json:"good"`
	Neutral uint `json:"neutral"`
	Poor    uint `json:"poor"`
	Unknown uint `json:"unknown"`
}

type ScrapeSiteDetail struct {
	Id                  uint64                        `json:"id"`
	Url                 string                        `json:"url"`
	AddedAt             time.Time                     `json:"added_at"`
	ProxyCount          uint                          `json:"proxy_count"`
	AliveCount          uint                          `json:"alive_count"`
	DeadCount           uint                          `json:"dead_count"`
	UnknownCount        uint                          `json:"unknown_count"`
	AvgReputation       *float32                      `json:"avg_reputation,omitempty"`
	LastProxyAddedAt    *time.Time                    `json:"last_proxy_added_at,omitempty"`
	LastCheckedAt       *time.Time                    `json:"last_checked_at,omitempty"`
	ReputationBreakdown ScrapeSiteReputationBreakdown `json:"reputation_breakdown"`
}
