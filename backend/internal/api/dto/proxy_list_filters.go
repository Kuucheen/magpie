package dto

type ProxyListFilters struct {
	Status           string   `json:"status,omitempty"`
	Protocols        []string `json:"protocols,omitempty"`
	Countries        []string `json:"countries,omitempty"`
	Types            []string `json:"types,omitempty"`
	AnonymityLevels  []string `json:"anonymityLevels,omitempty"`
	MaxTimeout       int      `json:"maxTimeout,omitempty"`
	MaxRetries       int      `json:"maxRetries,omitempty"`
	ReputationLabels []string `json:"reputationLabels,omitempty"`
}
