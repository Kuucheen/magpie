package dto

type ScrapeSourceExportSettings struct {
	ScrapeSources      []uint64 `json:"scrapeSources"`
	Filter             bool     `json:"filter"`
	Http               bool     `json:"http"`
	Https              bool     `json:"https"`
	ProxyCountMode     string   `json:"proxyCountMode"`
	ProxyCountOperator string   `json:"proxyCountOperator"`
	ProxyCount         uint     `json:"proxyCount"`
	AliveCountOperator string   `json:"aliveCountOperator"`
	AliveCount         uint     `json:"aliveCount"`
	MinProxyCount      uint     `json:"minProxyCount"`
	MinAliveCount      uint     `json:"minAliveCount"`
	OutputFormat       string   `json:"outputFormat"`
}
