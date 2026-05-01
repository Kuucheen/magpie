package database

import (
	"magpie/internal/api/dto"
	"magpie/internal/domain"
)

func collectProxyIDsForDeletion(userID uint, settings dto.DeleteSettings) ([]uint, error) {
	query := DB.Model(&domain.Proxy{}).
		Select("DISTINCT proxies.id").
		Joins("JOIN user_proxies ON user_proxies.proxy_id = proxies.id").
		Where("user_proxies.user_id = ?", userID)

	if settings.Scope == "selected" && len(settings.Proxies) > 0 {
		query = query.Where("proxies.id IN ?", settings.Proxies)
	}

	if settings.Filter {
		filterQuery := buildProxyListFilterQuery(userID, proxyListFiltersForDelete(settings))
		if filterQuery != nil {
			query = query.Where("proxies.id IN (?)", filterQuery)
		}
	}

	if !settings.Filter && (settings.ProxyStatus == "alive" || settings.ProxyStatus == "dead") {
		isAlive := settings.ProxyStatus == "alive"
		query = query.Joins("JOIN proxy_overall_statuses pos ON pos.proxy_id = proxies.id").
			Where("pos.overall_alive = ?", isAlive)
	}

	if !settings.Filter && len(settings.ReputationLabels) > 0 {
		query = applyReputationFilters(query, settings.ReputationLabels)
	}

	var ids []uint
	if err := query.Pluck("proxies.id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

func proxyListFiltersForDelete(settings dto.DeleteSettings) dto.ProxyListFilters {
	protocols := make([]string, 0, 4)
	if settings.Http {
		protocols = append(protocols, "http")
	}
	if settings.Https {
		protocols = append(protocols, "https")
	}
	if settings.Socks4 {
		protocols = append(protocols, "socks4")
	}
	if settings.Socks5 {
		protocols = append(protocols, "socks5")
	}

	return dto.ProxyListFilters{
		Status:           settings.ProxyStatus,
		Protocols:        protocols,
		MinHealthOverall: int(settings.MinHealthOverall),
		MinHealthHTTP:    int(settings.MinHealthHTTP),
		MinHealthHTTPS:   int(settings.MinHealthHTTPS),
		MinHealthSOCKS4:  int(settings.MinHealthSOCKS4),
		MinHealthSOCKS5:  int(settings.MinHealthSOCKS5),
		Countries:        normalizeFilterValues(settings.Countries),
		Types:            normalizeFilterValues(settings.Types),
		AnonymityLevels:  normalizeFilterValues(settings.AnonymityLevels),
		MaxTimeout:       int(settings.MaxTimeout),
		MaxRetries:       int(settings.MaxRetries),
		ReputationLabels: settings.ReputationLabels,
	}
}
