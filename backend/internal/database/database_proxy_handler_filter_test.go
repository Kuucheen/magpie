package database

import (
	"testing"

	"magpie/internal/api/dto"
	"magpie/internal/domain"
)

func TestFilterProxiesForExport_ReputationMatch(t *testing.T) {
	proxies := []domain.Proxy{
		{
			ID: 1,
			Statistics: []domain.ProxyStatistic{
				{Protocol: domain.Protocol{Name: "HTTP"}},
			},
			Reputations: []domain.ProxyReputation{
				{Kind: "http", Label: "good", Score: 90.5},
			},
		},
		{
			ID: 2,
			Statistics: []domain.ProxyStatistic{
				{Protocol: domain.Protocol{Name: "HTTP"}},
			},
			Reputations: []domain.ProxyReputation{
				{Kind: "http", Label: "neutral", Score: 60},
			},
		},
	}

	settings := dto.ExportSettings{
		ReputationLabels: []string{"good"},
	}

	filtered := filterProxiesForExport(proxies, settings)

	if len(filtered) != 1 || filtered[0].ID != 1 {
		t.Fatalf("expected only proxy ID 1, got %#v", filtered)
	}
}

func TestFilterProxiesForExport_ProtocolSpecificSelection(t *testing.T) {
	proxies := []domain.Proxy{
		{
			ID: 1,
			Statistics: []domain.ProxyStatistic{
				{Protocol: domain.Protocol{Name: "HTTPS"}},
			},
			Reputations: []domain.ProxyReputation{
				{Kind: "https", Label: "good", Score: 95},
				{Kind: "http", Label: "neutral", Score: 55},
			},
		},
		{
			ID: 2,
			Statistics: []domain.ProxyStatistic{
				{Protocol: domain.Protocol{Name: "HTTPS"}},
			},
			Reputations: []domain.ProxyReputation{
				{Kind: "https", Label: "poor", Score: 30},
			},
		},
	}

	settings := dto.ExportSettings{
		ReputationLabels: []string{"good"},
		Filter:           true,
		Https:            true,
	}

	filtered := filterProxiesForExport(proxies, settings)

	if len(filtered) != 1 || filtered[0].ID != 1 {
		t.Fatalf("expected only proxy ID 1, got %#v", filtered)
	}
}

func TestHasProxyListFilters_HealthThresholdsCountAsFilters(t *testing.T) {
	filters := dto.ProxyListFilters{
		MinHealthOverall: 70,
	}

	if !hasProxyListFilters(filters) {
		t.Fatal("expected health threshold to count as a proxy list filter")
	}
}
