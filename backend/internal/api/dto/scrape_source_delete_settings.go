package dto

type ScrapeSourceDeleteSettings struct {
	ScrapeSources      []uint64 `json:"scrapeSources"`
	Filter             bool     `json:"filter"`
	Http               bool     `json:"http"`
	Https              bool     `json:"https"`
	ProxyCountOperator string   `json:"proxyCountOperator"`
	ProxyCount         uint     `json:"proxyCount"`
	AliveCountOperator string   `json:"aliveCountOperator"`
	AliveCount         uint     `json:"aliveCount"`
	Scope              string   `json:"scope"`
}
