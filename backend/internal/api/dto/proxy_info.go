package dto

import "time"

type ProxyInfo struct {
	Id             int                     `json:"id"`
	IP             string                  `json:"ip"`
	Port           uint16                  `json:"port"`
	EstimatedType  string                  `json:"estimated_type"`
	ResponseTime   uint16                  `json:"response_time"`
	Country        string                  `json:"country"`
	AnonymityLevel string                  `json:"anonymity_level"`
	Alive          bool                    `json:"alive"`
	Health         *ProxyHealthSummary     `json:"health,omitempty"`
	LatestCheck    time.Time               `json:"latest_check"`
	Reputation     *ProxyReputationSummary `json:"reputation,omitempty"`
}

type ProxyHealthSummary struct {
	Overall *float32 `json:"overall,omitempty"`
	HTTP    *float32 `json:"http,omitempty"`
	HTTPS   *float32 `json:"https,omitempty"`
	SOCKS4  *float32 `json:"socks4,omitempty"`
	SOCKS5  *float32 `json:"socks5,omitempty"`
}

type ProxyPage struct {
	Proxies []ProxyInfo `json:"proxies"`
	Total   int64       `json:"total"`
}
