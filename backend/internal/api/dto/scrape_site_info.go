package dto

type ScrapeSiteInfo struct {
	Id           uint64 `json:"id"`
	Url          string `json:"url"`
	ProxyCount   uint   `json:"proxy_count"`
	AliveCount   uint   `json:"alive_count"`
	DeadCount    uint   `json:"dead_count"`
	UnknownCount uint   `json:"unknown_count"`
}
