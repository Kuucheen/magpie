package dto

type ScrapeSourceListFilters struct {
	Protocols          []string
	ProxyCountOperator string
	ProxyCount         uint
	AliveCountOperator string
	AliveCount         uint
}
