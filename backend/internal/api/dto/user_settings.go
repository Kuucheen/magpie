package dto

type UserSettings struct {
	HTTPProtocol               bool   `json:"http_protocol"`
	HTTPSProtocol              bool   `json:"https_protocol"`
	SOCKS4Protocol             bool   `json:"socks4_protocol"`
	SOCKS5Protocol             bool   `json:"socks5_protocol"`
	Timeout                    uint16 `json:"timeout"`
	Retries                    uint8  `json:"retries"`
	UseHttpsForSocks           bool   `gorm:"not null;default:true"`
	TransportProtocol          string `json:"transport_protocol"`
	AutoRemoveFailingProxies   bool   `json:"auto_remove_failing_proxies"`
	AutoRemoveFailureThreshold uint8  `json:"auto_remove_failure_threshold"`

	SimpleUserJudges []SimpleUserJudge `json:"judges"`

	ScrapingSources  []string `json:"scraping_sources"`
	ProxyListColumns []string `json:"proxy_list_columns"`
}
