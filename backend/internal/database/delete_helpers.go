package database

import (
	"magpie/internal/api/dto"
	"magpie/internal/domain"

	"gorm.io/gorm"
)

func collectProxyIDsForDeletion(userID uint, settings dto.DeleteSettings) ([]uint, error) {
	query := DB.Model(&domain.Proxy{}).
		Select("DISTINCT proxies.id").
		Joins("JOIN user_proxies ON user_proxies.proxy_id = proxies.id").
		Where("user_proxies.user_id = ?", userID)

	if settings.Scope == "selected" && len(settings.Proxies) > 0 {
		query = query.Where("proxies.id IN ?", settings.Proxies)
	}

	if settings.ProxyStatus == "alive" || settings.ProxyStatus == "dead" {
		isAlive := settings.ProxyStatus == "alive"
		query = query.Joins("JOIN proxy_overall_statuses pos ON pos.proxy_id = proxies.id").
			Where("pos.overall_alive = ?", isAlive)
	}

	if len(settings.ReputationLabels) > 0 {
		query = applyReputationFilters(query, settings.ReputationLabels)
	}

	if settings.Filter {
		query = applyDeleteFilterConditions(query, settings)
	}

	var ids []uint
	if err := query.Pluck("proxies.id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

func applyDeleteFilterConditions(query *gorm.DB, settings dto.DeleteSettings) *gorm.DB {
	needsProxyStatistics := settings.Http || settings.Https || settings.Socks4 || settings.Socks5 || settings.MaxTimeout > 0 || settings.MaxRetries > 0
	if needsProxyStatistics {
		query = query.Joins("JOIN proxy_statistics ON proxy_statistics.proxy_id = proxies.id")
	}

	if settings.Http || settings.Https || settings.Socks4 || settings.Socks5 {
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

		query = query.Joins("JOIN protocols ON proxy_statistics.protocol_id = protocols.id").
			Where("protocols.name IN ?", protocols)
	}

	if settings.MaxTimeout > 0 {
		query = query.Where("proxy_statistics.response_time <= ?", settings.MaxTimeout)
	}

	if settings.MaxRetries > 0 {
		query = query.Where("proxy_statistics.attempt <= ?", settings.MaxRetries)
	}

	return query
}
