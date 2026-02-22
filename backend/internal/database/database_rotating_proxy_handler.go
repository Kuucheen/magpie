package database

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"magpie/internal/api/dto"
	"magpie/internal/domain"
	"magpie/internal/support"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrRotatingProxyNotFound           = errors.New("rotating proxy not found")
	ErrRotatingProxyNameRequired       = errors.New("rotating proxy name is required")
	ErrRotatingProxyNameTooLong        = errors.New("rotating proxy name is too long")
	ErrRotatingProxyNameConflict       = errors.New("rotating proxy name already exists")
	ErrRotatingProxyProtocolMissing    = errors.New("rotating proxy protocol is required")
	ErrRotatingProxyProtocolDenied     = errors.New("protocol is not enabled for this user")
	ErrRotatingProxyNoAliveProxies     = errors.New("no alive proxies are available for the selected protocol")
	ErrRotatingProxyAuthUsernameNeeded = errors.New("authentication username is required when authentication is enabled")
	ErrRotatingProxyAuthPasswordNeeded = errors.New("authentication password is required when authentication is enabled")
	ErrRotatingProxyPortExhausted      = errors.New("no available ports for rotating proxies")
	ErrRotatingProxyUptimeTypeInvalid  = errors.New("uptime filter type must be either min or max")
	ErrRotatingProxyUptimeTypeMissing  = errors.New("uptime filter type is required when uptime percentage is set")
	ErrRotatingProxyUptimeValueMissing = errors.New("uptime percentage is required when uptime filter type is set")
	ErrRotatingProxyUptimeOutOfRange   = errors.New("uptime percentage must be between 0 and 100")
)

var (
	reputationLabelOrder = []string{"good", "neutral", "poor"}
	reputationLabelSet   = map[string]struct{}{
		"good":    {},
		"neutral": {},
		"poor":    {},
	}
)

const (
	rotatingProxyNameMaxLength = 120
	uptimeFilterMin            = "min"
	uptimeFilterMax            = "max"
	defaultInstanceRegion      = "Unknown"
)

func CreateRotatingProxy(userID uint, payload dto.RotatingProxyCreateRequest) (*dto.RotatingProxy, error) {
	if DB == nil {
		return nil, fmt.Errorf("rotating proxy: database connection was not initialised")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return nil, ErrRotatingProxyNameRequired
	}
	if len(name) > rotatingProxyNameMaxLength {
		return nil, ErrRotatingProxyNameTooLong
	}

	protocolName := strings.ToLower(strings.TrimSpace(payload.Protocol))
	if protocolName == "" {
		return nil, ErrRotatingProxyProtocolMissing
	}
	listenProtocolName := strings.ToLower(strings.TrimSpace(payload.ListenProtocol))
	if listenProtocolName == "" {
		listenProtocolName = protocolName
	}
	transportProtocol := support.NormalizeTransportProtocol(payload.TransportProtocol)
	listenTransportProtocol := support.NormalizeTransportProtocol(payload.ListenTransportProtocol)
	if strings.TrimSpace(payload.ListenTransportProtocol) == "" {
		listenTransportProtocol = transportProtocol
	}
	instanceID := strings.TrimSpace(payload.InstanceID)
	if instanceID == "" {
		instanceID = support.GetInstanceID()
	}
	instanceName := strings.TrimSpace(payload.InstanceName)
	if instanceName == "" {
		instanceName = instanceID
	}
	instanceRegion := strings.TrimSpace(payload.InstanceRegion)
	if instanceRegion == "" {
		instanceRegion = defaultInstanceRegion
	}

	if payload.AuthRequired {
		if strings.TrimSpace(payload.AuthUsername) == "" {
			return nil, ErrRotatingProxyAuthUsernameNeeded
		}
		if strings.TrimSpace(payload.AuthPassword) == "" {
			return nil, ErrRotatingProxyAuthPasswordNeeded
		}
	}

	uptimeFilterType, uptimePercentage, err := validateRotatorUptimeFilter(payload.UptimeFilterType, payload.UptimePercentage)
	if err != nil {
		return nil, err
	}

	var result *dto.RotatingProxy

	err = DB.Transaction(func(tx *gorm.DB) error {
		var user domain.User
		if err := tx.First(&user, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("rotating proxy: user %d not found", userID)
			}
			return err
		}

		proxyProtocol, err := fetchProtocolByName(tx, protocolName)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRotatingProxyProtocolDenied
			}
			return err
		}
		if !isProtocolEnabledForUser(user, protocolName) {
			return ErrRotatingProxyProtocolDenied
		}

		listenProtocol, err := fetchProtocolByName(tx, listenProtocolName)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRotatingProxyProtocolDenied
			}
			return err
		}
		filters := sanitizeRotatorReputationLabels(payload.ReputationLabels)

		entity := domain.RotatingProxy{
			UserID:                  userID,
			Name:                    name,
			InstanceID:              instanceID,
			InstanceName:            instanceName,
			InstanceRegion:          instanceRegion,
			ProtocolID:              proxyProtocol.ID,
			ListenProtocolID:        listenProtocol.ID,
			TransportProtocol:       transportProtocol,
			ListenTransportProtocol: listenTransportProtocol,
			UptimeFilterType:        uptimeFilterType,
			UptimePercentage:        cloneFloat64Ptr(uptimePercentage),
			AuthRequired:            payload.AuthRequired,
			AuthUsername:            strings.TrimSpace(payload.AuthUsername),
			AuthPassword:            payload.AuthPassword,
			ReputationLabels:        domain.StringList(filters),
		}

		listenPort, err := allocateListenPort(tx, instanceID)
		if err != nil {
			return err
		}
		entity.ListenPort = listenPort

		if err := tx.Create(&entity).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrRotatingProxyNameConflict
			}
			return err
		}

		aliveProxies, err := aliveProxiesForProtocol(tx, userID, proxyProtocol.ID, filters, uptimeFilterType, uptimePercentage)
		if err != nil {
			return err
		}

		result = &dto.RotatingProxy{
			ID:                      entity.ID,
			Name:                    entity.Name,
			InstanceID:              entity.InstanceID,
			InstanceName:            entity.InstanceName,
			InstanceRegion:          entity.InstanceRegion,
			Protocol:                proxyProtocol.Name,
			ListenProtocol:          listenProtocol.Name,
			TransportProtocol:       transportProtocol,
			ListenTransportProtocol: listenTransportProtocol,
			UptimeFilterType:        uptimeFilterType,
			UptimePercentage:        cloneFloat64Ptr(uptimePercentage),
			AliveProxyCount:         len(aliveProxies),
			ListenPort:              entity.ListenPort,
			AuthRequired:            entity.AuthRequired,
			AuthUsername:            entity.AuthUsername,
			AuthPassword:            strings.TrimSpace(payload.AuthPassword),
			ReputationLabels:        filters,
			CreatedAt:               entity.CreatedAt,
		}

		entity.AuthPassword = ""

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func ListRotatingProxies(userID uint) ([]dto.RotatingProxy, error) {
	if DB == nil {
		return nil, fmt.Errorf("rotating proxy: database connection was not initialised")
	}

	var rows []domain.RotatingProxy
	if err := DB.
		Preload("Protocol").
		Preload("ListenProtocol").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return []dto.RotatingProxy{}, nil
	}

	protocolCache := make(map[string][]domain.Proxy)
	lastProxyCache := make(map[uint64]string)
	result := make([]dto.RotatingProxy, 0, len(rows))

	for _, row := range rows {
		normalizeRotatingProxyProtocols(&row)
		protocolName := row.Protocol.Name
		listenProtocol := row.ListenProtocol.Name
		transportProtocol := support.NormalizeTransportProtocol(row.TransportProtocol)
		listenTransportProtocol := support.NormalizeTransportProtocol(row.ListenTransportProtocol)
		if strings.TrimSpace(row.ListenTransportProtocol) == "" {
			listenTransportProtocol = transportProtocol
		}
		labels := sanitizeRotatorReputationLabels(row.ReputationLabels.Clone())
		uptimeFilterType, uptimePercentage := normalizeRotatorUptimeFilter(row.UptimeFilterType, row.UptimePercentage)
		instanceID := strings.TrimSpace(row.InstanceID)
		if instanceID == "" {
			instanceID = support.GetInstanceID()
		}
		instanceName := strings.TrimSpace(row.InstanceName)
		if instanceName == "" {
			instanceName = instanceID
		}
		instanceRegion := strings.TrimSpace(row.InstanceRegion)
		if instanceRegion == "" {
			instanceRegion = defaultInstanceRegion
		}
		proxies, err := getAliveProxiesCached(userID, row.ProtocolID, labels, uptimeFilterType, uptimePercentage, protocolCache)
		if err != nil {
			return nil, err
		}

		lastProxy := ""
		if row.LastProxyID != nil {
			lastProxy, err = getProxyAddressCached(userID, *row.LastProxyID, lastProxyCache)
			if err != nil {
				return nil, err
			}
		}

		result = append(result, dto.RotatingProxy{
			ID:                      row.ID,
			Name:                    row.Name,
			InstanceID:              instanceID,
			InstanceName:            instanceName,
			InstanceRegion:          instanceRegion,
			Protocol:                protocolName,
			ListenProtocol:          listenProtocol,
			TransportProtocol:       transportProtocol,
			ListenTransportProtocol: listenTransportProtocol,
			UptimeFilterType:        uptimeFilterType,
			UptimePercentage:        cloneFloat64Ptr(uptimePercentage),
			AliveProxyCount:         len(proxies),
			ListenPort:              row.ListenPort,
			AuthRequired:            row.AuthRequired,
			AuthUsername:            row.AuthUsername,
			AuthPassword:            row.AuthPassword,
			LastRotationAt:          row.LastRotationAt,
			LastServedProxy:         lastProxy,
			ReputationLabels:        labels,
			CreatedAt:               row.CreatedAt,
		})
	}

	return result, nil
}

func DeleteRotatingProxy(userID uint, rotatingProxyID uint64) error {
	if DB == nil {
		return fmt.Errorf("rotating proxy: database connection was not initialised")
	}

	res := DB.Where("user_id = ? AND id = ?", userID, rotatingProxyID).Delete(&domain.RotatingProxy{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRotatingProxyNotFound
	}
	return nil
}

func GetNextRotatingProxy(userID uint, rotatingProxyID uint64) (*dto.RotatingProxyNext, error) {
	if DB == nil {
		return nil, fmt.Errorf("rotating proxy: database connection was not initialised")
	}

	var result *dto.RotatingProxyNext

	err := DB.Transaction(func(tx *gorm.DB) error {
		var entity domain.RotatingProxy
		if err := tx.
			Preload("Protocol").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND id = ?", userID, rotatingProxyID).
			First(&entity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRotatingProxyNotFound
			}
			return err
		}

		labels := sanitizeRotatorReputationLabels(entity.ReputationLabels.Clone())
		uptimeFilterType, uptimePercentage := normalizeRotatorUptimeFilter(entity.UptimeFilterType, entity.UptimePercentage)
		proxies, err := aliveProxiesForProtocol(tx, userID, entity.ProtocolID, labels, uptimeFilterType, uptimePercentage)
		if err != nil {
			return err
		}

		if len(proxies) == 0 {
			return ErrRotatingProxyNoAliveProxies
		}

		selected := selectNextProxy(proxies, entity.LastProxyID)

		now := time.Now()

		updatePayload := map[string]interface{}{
			"last_proxy_id":    selected.ID,
			"last_rotation_at": now,
		}

		if err := tx.Model(&domain.RotatingProxy{}).
			Where("id = ?", entity.ID).
			Updates(updatePayload).Error; err != nil {
			return err
		}

		result = &dto.RotatingProxyNext{
			ProxyID:  selected.ID,
			IP:       selected.GetIp(),
			Port:     selected.Port,
			Username: selected.Username,
			Password: selected.Password,
			HasAuth:  selected.HasAuth(),
			Protocol: entity.Protocol.Name,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func getAliveProxiesCached(userID uint, protocolID int, labels []string, uptimeFilterType string, uptimePercentage *float64, cache map[string][]domain.Proxy) ([]domain.Proxy, error) {
	normLabels := sanitizeRotatorReputationLabels(labels)
	cacheKey := buildAliveProxyCacheKey(protocolID, normLabels, uptimeFilterType, uptimePercentage)

	if proxies, ok := cache[cacheKey]; ok {
		return proxies, nil
	}

	proxies, err := aliveProxiesForProtocol(DB, userID, protocolID, normLabels, uptimeFilterType, uptimePercentage)
	if err != nil {
		return nil, err
	}

	cache[cacheKey] = proxies
	return proxies, nil
}

func getProxyAddressCached(userID uint, proxyID uint64, cache map[uint64]string) (string, error) {
	if cached, ok := cache[proxyID]; ok {
		return cached, nil
	}

	proxy, err := fetchUserProxyByID(DB, userID, proxyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cache[proxyID] = ""
			return "", nil
		}
		return "", err
	}

	address := proxy.GetFullProxy()
	cache[proxyID] = address
	return address, nil
}

func aliveProxiesForProtocol(tx *gorm.DB, userID uint, protocolID int, labels []string, uptimeFilterType string, uptimePercentage *float64) ([]domain.Proxy, error) {
	filterLabels := sanitizeRotatorReputationLabels(labels)

	var proxies []domain.Proxy
	query := tx.
		Model(&domain.Proxy{}).
		Select("proxies.*").
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userID).
		Joins("JOIN proxy_latest_statistics pls ON pls.proxy_id = proxies.id AND pls.protocol_id = ? AND pls.alive = ?", protocolID, true)

	query = applyReputationFilter(query, filterLabels)
	query = applyUptimeFilter(query, tx, protocolID, uptimeFilterType, uptimePercentage)

	err := query.
		Order("proxies.id").
		Find(&proxies).Error
	if err != nil {
		return nil, err
	}

	return proxies, nil
}

func applyReputationFilter(query *gorm.DB, labels []string) *gorm.DB {
	if !shouldApplyReputationFilter(labels) {
		return query
	}

	return query.
		Joins("JOIN proxy_reputations pr ON pr.proxy_id = proxies.id AND pr.kind = ?", domain.ProxyReputationKindOverall).
		Where("pr.label IN ?", labels)
}

func applyUptimeFilter(query *gorm.DB, tx *gorm.DB, protocolID int, uptimeFilterType string, uptimePercentage *float64) *gorm.DB {
	normalizedType, normalizedPercentage := normalizeRotatorUptimeFilter(uptimeFilterType, uptimePercentage)
	if normalizedType == "" || normalizedPercentage == nil {
		return query
	}

	uptimeQuery := tx.
		Table("proxy_statistics psr").
		Select(
			"psr.proxy_id AS proxy_id, "+
				"ROUND(100.0 * SUM(CASE WHEN psr.alive THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0), 1) AS uptime_percentage",
		).
		Where("psr.protocol_id = ?", protocolID).
		Group("psr.proxy_id")

	query = query.Joins("JOIN (?) AS puf ON puf.proxy_id = proxies.id", uptimeQuery)
	if normalizedType == uptimeFilterMax {
		return query.Where("puf.uptime_percentage <= ?", *normalizedPercentage)
	}
	return query.Where("puf.uptime_percentage >= ?", *normalizedPercentage)
}

func shouldApplyReputationFilter(labels []string) bool {
	return len(labels) > 0 && len(labels) < len(reputationLabelOrder)
}

func fetchUserProxyByID(tx *gorm.DB, userID uint, proxyID uint64) (*domain.Proxy, error) {
	var proxy domain.Proxy
	err := tx.
		Model(&domain.Proxy{}).
		Where("proxies.id = ?", proxyID).
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userID).
		First(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func selectNextProxy(proxies []domain.Proxy, lastProxyID *uint64) domain.Proxy {
	if lastProxyID == nil {
		return proxies[0]
	}

	for idx := range proxies {
		if proxies[idx].ID == *lastProxyID {
			next := idx + 1
			if next >= len(proxies) {
				next = 0
			}
			return proxies[next]
		}
	}

	return proxies[0]
}

func fetchProtocolByName(tx *gorm.DB, name string) (domain.Protocol, error) {
	var protocol domain.Protocol
	err := tx.
		Model(&domain.Protocol{}).
		Where("LOWER(name) = ?", strings.ToLower(name)).
		First(&protocol).Error
	return protocol, err
}

func isProtocolEnabledForUser(user domain.User, protocolName string) bool {
	switch protocolName {
	case "http":
		return user.HTTPProtocol
	case "https":
		return user.HTTPSProtocol
	case "socks4":
		return user.SOCKS4Protocol
	case "socks5":
		return user.SOCKS5Protocol
	default:
		return false
	}
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key value violates unique constraint")
}

func allocateListenPort(tx *gorm.DB, instanceID string) (uint16, error) {
	start, end := support.GetRotatingProxyPortRange()
	if start <= 0 || end <= 0 {
		return 0, ErrRotatingProxyPortExhausted
	}

	count := end - start + 1
	if count <= 0 {
		return 0, ErrRotatingProxyPortExhausted
	}

	ports := make([]int, 0, count)
	for port := start; port <= end; port++ {
		ports = append(ports, port)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(ports), func(i, j int) {
		ports[i], ports[j] = ports[j], ports[i]
	})

	for _, port := range ports {
		var existing int64
		if err := tx.Model(&domain.RotatingProxy{}).
			Where("instance_id = ? AND listen_port = ?", instanceID, port).
			Count(&existing).Error; err != nil {
			return 0, err
		}
		if existing == 0 {
			return uint16(port), nil
		}
	}

	return 0, ErrRotatingProxyPortExhausted
}

func sanitizeRotatorReputationLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(labels))
	for _, raw := range labels {
		label := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := reputationLabelSet[label]; ok {
			seen[label] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	result := make([]string, 0, len(seen))
	for _, label := range reputationLabelOrder {
		if _, ok := seen[label]; ok {
			result = append(result, label)
		}
	}
	return result
}

func validateRotatorUptimeFilter(rawType string, rawPercentage *float64) (string, *float64, error) {
	filterType := strings.ToLower(strings.TrimSpace(rawType))
	hasType := filterType != ""
	hasPercentage := rawPercentage != nil

	if !hasType && !hasPercentage {
		return "", nil, nil
	}
	if !hasType && hasPercentage {
		return "", nil, ErrRotatingProxyUptimeTypeMissing
	}
	if hasType && !hasPercentage {
		return "", nil, ErrRotatingProxyUptimeValueMissing
	}
	if filterType != uptimeFilterMin && filterType != uptimeFilterMax {
		return "", nil, ErrRotatingProxyUptimeTypeInvalid
	}

	value := *rawPercentage
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", nil, ErrRotatingProxyUptimeOutOfRange
	}
	if value < 0 || value > 100 {
		return "", nil, ErrRotatingProxyUptimeOutOfRange
	}

	rounded := math.Round(value*10) / 10
	return filterType, &rounded, nil
}

func normalizeRotatorUptimeFilter(rawType string, rawPercentage *float64) (string, *float64) {
	filterType := strings.ToLower(strings.TrimSpace(rawType))
	if rawPercentage == nil {
		return "", nil
	}
	if filterType != uptimeFilterMin && filterType != uptimeFilterMax {
		return "", nil
	}

	value := *rawPercentage
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
		return "", nil
	}

	rounded := math.Round(value*10) / 10
	return filterType, &rounded
}

func buildAliveProxyCacheKey(protocolID int, labels []string, uptimeFilterType string, uptimePercentage *float64) string {
	labelKey := "*"
	if len(labels) > 0 {
		labelKey = strings.Join(labels, ",")
	}

	uptimeType, uptimeValue := normalizeRotatorUptimeFilter(uptimeFilterType, uptimePercentage)
	uptimeKey := "*"
	if uptimeType != "" && uptimeValue != nil {
		uptimeKey = fmt.Sprintf("%s:%0.1f", uptimeType, *uptimeValue)
	}

	return fmt.Sprintf("%d:%s:%s", protocolID, labelKey, uptimeKey)
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func GetAllRotatingProxies() ([]domain.RotatingProxy, error) {
	if DB == nil {
		return nil, fmt.Errorf("rotating proxy: database connection was not initialised")
	}

	instanceID := support.GetInstanceID()

	var proxies []domain.RotatingProxy
	if err := DB.
		Preload("Protocol").
		Preload("ListenProtocol").
		Where("instance_id = ?", instanceID).
		Order("created_at ASC").
		Find(&proxies).Error; err != nil {
		return nil, err
	}

	for idx := range proxies {
		normalizeRotatingProxyProtocols(&proxies[idx])
	}

	return proxies, nil
}

func CountRotatingProxiesByInstanceIDs(instanceIDs []string) (map[string]int, error) {
	if DB == nil {
		return nil, fmt.Errorf("rotating proxy: database connection was not initialised")
	}

	cleanIDs := make([]string, 0, len(instanceIDs))
	seen := make(map[string]struct{}, len(instanceIDs))
	for _, raw := range instanceIDs {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		cleanIDs = append(cleanIDs, value)
	}
	if len(cleanIDs) == 0 {
		return map[string]int{}, nil
	}

	type instanceCount struct {
		InstanceID string `gorm:"column:instance_id"`
		Count      int64  `gorm:"column:count"`
	}
	var rows []instanceCount
	if err := DB.
		Model(&domain.RotatingProxy{}).
		Select("instance_id, COUNT(*) AS count").
		Where("instance_id IN ?", cleanIDs).
		Group("instance_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]int, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.InstanceID)
		if id == "" {
			continue
		}
		result[id] = int(row.Count)
	}
	return result, nil
}

func GetRotatingProxyByID(rotatorID uint64) (*domain.RotatingProxy, error) {
	if DB == nil {
		return nil, fmt.Errorf("rotating proxy: database connection was not initialised")
	}

	instanceID := support.GetInstanceID()

	var proxy domain.RotatingProxy
	if err := DB.
		Preload("Protocol").
		Preload("ListenProtocol").
		Where("id = ? AND instance_id = ?", rotatorID, instanceID).
		First(&proxy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRotatingProxyNotFound
		}
		return nil, err
	}

	normalizeRotatingProxyProtocols(&proxy)

	return &proxy, nil
}

func normalizeRotatingProxyProtocols(rotator *domain.RotatingProxy) {
	if rotator == nil {
		return
	}

	if rotator.ListenProtocolID == 0 {
		rotator.ListenProtocolID = rotator.ProtocolID
	}

	if strings.TrimSpace(rotator.ListenProtocol.Name) == "" {
		rotator.ListenProtocol = rotator.Protocol
	}

	transportProtocol := support.NormalizeTransportProtocol(rotator.TransportProtocol)
	listenTransportProtocol := support.NormalizeTransportProtocol(rotator.ListenTransportProtocol)
	if strings.TrimSpace(rotator.ListenTransportProtocol) == "" {
		listenTransportProtocol = transportProtocol
	}
	uptimeFilterType, uptimePercentage := normalizeRotatorUptimeFilter(rotator.UptimeFilterType, rotator.UptimePercentage)

	rotator.TransportProtocol = transportProtocol
	rotator.ListenTransportProtocol = listenTransportProtocol
	rotator.UptimeFilterType = uptimeFilterType
	rotator.UptimePercentage = cloneFloat64Ptr(uptimePercentage)
	if strings.TrimSpace(rotator.InstanceID) == "" {
		rotator.InstanceID = support.GetInstanceID()
	}
	if strings.TrimSpace(rotator.InstanceName) == "" {
		rotator.InstanceName = rotator.InstanceID
	}
	if strings.TrimSpace(rotator.InstanceRegion) == "" {
		rotator.InstanceRegion = defaultInstanceRegion
	}
}
