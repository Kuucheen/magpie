package dto

type ProxyListFilters struct {
	Status           string   `json:"status,omitempty"`
	Protocols        []string `json:"protocols,omitempty"`
	MinHealthOverall int      `json:"minHealthOverall,omitempty"`
	MinHealthHTTP    int      `json:"minHealthHttp,omitempty"`
	MinHealthHTTPS   int      `json:"minHealthHttps,omitempty"`
	MinHealthSOCKS4  int      `json:"minHealthSocks4,omitempty"`
	MinHealthSOCKS5  int      `json:"minHealthSocks5,omitempty"`
	Countries        []string `json:"countries,omitempty"`
	Types            []string `json:"types,omitempty"`
	AnonymityLevels  []string `json:"anonymityLevels,omitempty"`
	MaxTimeout       int      `json:"maxTimeout,omitempty"`
	MaxRetries       int      `json:"maxRetries,omitempty"`
	ReputationLabels []string `json:"reputationLabels,omitempty"`
}
