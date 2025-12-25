package dto

type AddProxiesDetails struct {
	SubmittedCount     int   `json:"submittedCount"`
	ParsedCount        int   `json:"parsedCount"`
	InvalidFormatCount int   `json:"invalidFormatCount"`
	InvalidIPCount     int   `json:"invalidIpCount"`
	InvalidIPv4Count   int   `json:"invalidIpv4Count"`
	InvalidPortCount   int   `json:"invalidPortCount"`
	BlacklistedCount   int   `json:"blacklistedCount"`
	ProcessingMs       int64 `json:"processingMs"`
}

type AddProxiesResponse struct {
	ProxyCount int               `json:"proxyCount"`
	Details    AddProxiesDetails `json:"details"`
}
