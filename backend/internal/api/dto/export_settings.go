package dto

type ExportSettings struct {
	Proxies          []uint   `json:"proxies"`
	Filter           bool     `json:"filter"`
	Http             bool     `json:"http"`
	Https            bool     `json:"https"`
	Socks4           bool     `json:"socks4"`
	Socks5           bool     `json:"socks5"`
	MinHealthOverall uint     `json:"minHealthOverall"`
	MinHealthHTTP    uint     `json:"minHealthHttp"`
	MinHealthHTTPS   uint     `json:"minHealthHttps"`
	MinHealthSOCKS4  uint     `json:"minHealthSocks4"`
	MinHealthSOCKS5  uint     `json:"minHealthSocks5"`
	MaxRetries       uint     `json:"maxRetries"`
	MaxTimeout       uint     `json:"maxTimeout"`
	Countries        []string `json:"countries"`
	Types            []string `json:"types"`
	AnonymityLevels  []string `json:"anonymityLevels"`
	ProxyStatus      string   `json:"proxyStatus"`
	ReputationLabels []string `json:"reputationLabels"`
	OutputFormat     string   `json:"outputFormat"`
}
