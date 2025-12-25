package database

import (
	"time"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"magpie/internal/api/dto"
	"magpie/internal/config"
	"magpie/internal/domain"
	"magpie/internal/security"
	"magpie/internal/support"
)

func GetUserFromId(id uint) domain.User {
	var users domain.User
	DB.Where("id = ?", id).First(&users)
	return users
}

func GetUsersByIDs(ids []uint) (map[uint]domain.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	unique := make(map[uint]struct{}, len(ids))
	filtered := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		filtered = append(filtered, id)
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	var users []domain.User
	if err := DB.Where("id IN ?", filtered).Find(&users).Error; err != nil {
		return nil, err
	}

	result := make(map[uint]domain.User, len(users))
	for _, user := range users {
		result[user.ID] = user
	}

	return result, nil
}

func GetUsersThatDontHaveJudges() []domain.User {
	var users []domain.User
	DB.Where("id NOT IN (SELECT DISTINCT user_id FROM user_judges)").Find(&users)
	return users
}

// AddUserJudgesRelation cannot normally fail because of to many parameters because
// users start with the default judges anyway
func AddUserJudgesRelation(users []domain.User, judges []*domain.JudgeWithRegex) error {
	var userJudges []domain.UserJudge

	for _, user := range users {
		for _, judge := range judges {
			userJudges = append(userJudges, domain.UserJudge{
				UserID:  user.ID,
				JudgeID: judge.Judge.ID,
				Regex:   judge.Regex,
			})
		}
	}

	if len(userJudges) > 0 {
		if err := DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "judge_id"}},
			DoNothing: true,
		}).Create(&userJudges).Error; err != nil {
			return err
		}
	}

	return nil
}

func GetAllUserJudgeRelations() ([]domain.UserJudge, []domain.JudgeWithRegex) {
	var userJudges []domain.UserJudge
	if err := DB.Find(&userJudges).Error; err != nil {
		return nil, nil
	}

	var results []struct {
		ID         uint   `gorm:"column:id"`
		FullString string `gorm:"column:full_string"`
		CreatedAt  time.Time
		Regex      string `gorm:"column:regex"`
	}

	if err := DB.Table("user_judges").
		Select("judges.id, judges.full_string, judges.created_at, user_judges.regex").
		Joins("JOIN judges ON user_judges.judge_id = judges.id").
		Scan(&results).Error; err != nil {
		return nil, nil
	}

	var judgesWithRegex []domain.JudgeWithRegex
	for _, result := range results {
		judge := &domain.Judge{
			ID:         result.ID,
			FullString: result.FullString,
			CreatedAt:  result.CreatedAt,
		}
		judge.SetUp()
		judgesWithRegex = append(judgesWithRegex, domain.JudgeWithRegex{
			Judge: judge,
			Regex: result.Regex,
		})
	}

	return userJudges, judgesWithRegex
}

func UpdateUserSettings(userID uint, settings dto.UserSettings) error {
	// Wrap everything in a single transaction so either all changes
	// happen or none do.
	return DB.Transaction(func(tx *gorm.DB) error {
		transportProtocol := support.NormalizeTransportProtocol(settings.TransportProtocol)

		/* ─── 1.  Update primitive columns on the User row ─────────────────────── */
		updates := map[string]interface{}{
			"HTTPProtocol":               settings.HTTPProtocol,
			"HTTPSProtocol":              settings.HTTPSProtocol,
			"SOCKS4Protocol":             settings.SOCKS4Protocol,
			"SOCKS5Protocol":             settings.SOCKS5Protocol,
			"Timeout":                    settings.Timeout,
			"Retries":                    settings.Retries,
			"UseHttpsForSocks":           settings.UseHttpsForSocks,
			"TransportProtocol":          transportProtocol,
			"AutoRemoveFailingProxies":   settings.AutoRemoveFailingProxies,
			"AutoRemoveFailureThreshold": settings.AutoRemoveFailureThreshold,
		}
		if err := tx.Model(&domain.User{}).
			Where("id = ?", userID).
			Updates(updates).Error; err != nil {
			return err
		}

		keepIDs := make([]uint, 0, len(settings.SimpleUserJudges))

		for _, s := range settings.SimpleUserJudges {
			if config.IsWebsiteBlocked(s.Url) {
				log.Info("Skipped blocked judge for user", "user_id", userID, "url", s.Url)
				continue
			}

			judge := domain.Judge{FullString: s.Url}
			if err := tx.
				Clauses(clause.OnConflict{DoNothing: true}).
				FirstOrCreate(&judge, judge).Error; err != nil {
				return err
			}

			keepIDs = append(keepIDs, judge.ID)

			uj := domain.UserJudge{
				UserID:  userID,
				JudgeID: judge.ID,
				Regex:   s.Regex,
			}
			if err := tx.
				Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "user_id"}, {Name: "judge_id"}},
					DoUpdates: clause.AssignmentColumns([]string{"regex"}),
				}).
				Create(&uj).Error; err != nil {
				return err
			}
		}

		if err := tx.
			Where("user_id = ? AND judge_id NOT IN ?", userID, keepIDs).
			Delete(&domain.UserJudge{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func GetUserJudges(userid uint) []dto.SimpleUserJudge {
	var results []dto.SimpleUserJudge

	if err := DB.Table("user_judges").
		Select("judges.full_string AS Url, user_judges.regex AS Regex").
		Joins("JOIN judges ON user_judges.judge_id = judges.id").
		Where("user_judges.user_id = ?", userid).
		Scan(&results).Error; err != nil {
		return nil
	}

	return results
}

func GetDashboardInfo(userid uint) dto.DashboardInfo {
	var info dto.DashboardInfo
	// cut‑off for “this week”
	weekAgo := time.Now().AddDate(0, 0, -7)

	// 1) TotalChecks
	DB.Model(&domain.ProxyStatistic{}).
		Joins("JOIN user_proxies up ON up.proxy_id = proxy_statistics.proxy_id").
		Where("up.user_id = ?", userid).
		Count(&info.TotalChecks)

	// 2) TotalChecksWeek
	DB.Model(&domain.ProxyStatistic{}).
		Joins("JOIN user_proxies up ON up.proxy_id = proxy_statistics.proxy_id").
		Where("up.user_id = ? AND proxy_statistics.created_at >= ?", userid, weekAgo).
		Count(&info.TotalChecksWeek)

	// 3) TotalScraped
	DB.Table("proxy_scrape_site AS ps").
		Joins("JOIN user_proxies up ON up.proxy_id = ps.proxy_id").
		Where("up.user_id = ?", userid).
		Count(&info.TotalScraped)

	// 4) TotalScrapedWeek
	DB.Table("proxy_scrape_site AS ps").
		Joins("JOIN user_proxies up ON up.proxy_id = ps.proxy_id").
		Where("up.user_id = ? AND ps.created_at >= ?", userid, weekAgo).
		Count(&info.TotalScrapedWeek)

	// 5) Country breakdown – latest known country per proxy assigned to the user
	type countryCount struct {
		Country string `gorm:"column:country"`
		Count   uint   `gorm:"column:count"`
	}

	var countries []countryCount

	const countryExpr = "COALESCE(NULLIF(proxies.country, ''), 'Unknown')"

	DB.Model(&domain.Proxy{}).
		Select(countryExpr+" AS country, COUNT(*) AS count").
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userid).
		Group(countryExpr).
		Order("count DESC, country ASC").
		Scan(&countries)

	for _, row := range countries {
		country := row.Country
		if country == "" || country == "N/A" {
			country = "Unknown"
		}
		info.CountryBreakdown = append(info.CountryBreakdown, struct {
			Country string `json:"country"`
			Count   uint   `json:"count"`
		}{
			Country: country,
			Count:   row.Count,
		})
	}

	// 6) JudgeValidProxies – one row per judge, with counts by anonymity level
	type jvp struct {
		JudgeUrl           string `json:"judge_url"`
		EliteProxies       uint   `json:"elite_proxies"`
		AnonymousProxies   uint   `json:"anonymous_proxies"`
		TransparentProxies uint   `json:"transparent_proxies"`
	}
	var tmp []jvp

	DB.Model(&domain.ProxyStatistic{}).
		Select(
			"j.full_string AS judge_url, "+
				"SUM(CASE WHEN al.name = 'elite' THEN 1 ELSE 0 END)       AS elite_proxies, "+
				"SUM(CASE WHEN al.name = 'anonymous' THEN 1 ELSE 0 END)   AS anonymous_proxies, "+
				"SUM(CASE WHEN al.name = 'transparent' THEN 1 ELSE 0 END) AS transparent_proxies",
		).
		Joins("JOIN user_proxies up ON up.proxy_id = proxy_statistics.proxy_id AND up.user_id = ?", userid).
		Joins("JOIN user_judges uj ON uj.judge_id = proxy_statistics.judge_id AND uj.user_id = ?", userid).
		Joins("JOIN judges j ON j.id = proxy_statistics.judge_id").
		Joins("JOIN anonymity_levels al ON al.id = proxy_statistics.level_id").
		Where("proxy_statistics.alive = TRUE").
		Group("j.id, j.full_string").
		Scan(&tmp)

	// assign into the dto struct
	for _, row := range tmp {
		info.JudgeValidProxies = append(info.JudgeValidProxies, struct {
			JudgeUrl           string `json:"judge_url"`
			EliteProxies       uint   `json:"elite_proxies"`
			AnonymousProxies   uint   `json:"anonymous_proxies"`
			TransparentProxies uint   `json:"transparent_proxies"`
		}{
			JudgeUrl:           row.JudgeUrl,
			EliteProxies:       row.EliteProxies,
			AnonymousProxies:   row.AnonymousProxies,
			TransparentProxies: row.TransparentProxies,
		})
	}

	// 7) Reputation breakdown (good / neutral / poor / unknown)
	type reputationCount struct {
		Label string `gorm:"column:label"`
		Count uint   `gorm:"column:count"`
	}

	var repCounts []reputationCount

	DB.Table("proxy_reputations AS pr").
		Select("LOWER(COALESCE(NULLIF(pr.label, ''), 'unknown')) AS label, COUNT(*) AS count").
		Joins("JOIN user_proxies up ON up.proxy_id = pr.proxy_id AND up.user_id = ?", userid).
		Where("pr.kind = ?", domain.ProxyReputationKindOverall).
		Group("label").
		Scan(&repCounts)

	for _, row := range repCounts {
		switch row.Label {
		case "good":
			info.ReputationBreakdown.Good = row.Count
		case "neutral":
			info.ReputationBreakdown.Neutral = row.Count
		case "poor":
			info.ReputationBreakdown.Poor = row.Count
		default:
			info.ReputationBreakdown.Unknown += row.Count
		}
	}

	// 8) Best overall reputation proxy
	type topProxyRow struct {
		ProxyID uint64  `gorm:"column:proxy_id"`
		IP      string  `gorm:"column:ip"`
		Port    uint    `gorm:"column:port"`
		Score   float32 `gorm:"column:score"`
		Label   string  `gorm:"column:label"`
	}

	var topRow topProxyRow

	topResult := DB.Table("proxy_reputations AS pr").
		Select("pr.proxy_id, pr.score, pr.label, p.ip, p.port").
		Joins("JOIN user_proxies up ON up.proxy_id = pr.proxy_id AND up.user_id = ?", userid).
		Joins("JOIN proxies p ON p.id = pr.proxy_id").
		Where("pr.kind = ?", domain.ProxyReputationKindOverall).
		Order("pr.score DESC, pr.proxy_id ASC").
		Limit(1).
		Scan(&topRow)

	if topResult.Error == nil && topResult.RowsAffected > 0 && topRow.ProxyID != 0 {
		ip := topRow.IP
		if ip != "" {
			plain, _, err := security.DecryptProxySecret(ip)
			if err != nil {
				log.Errorf("decrypt top reputation proxy ip: %v", err)
			} else {
				ip = plain
			}
		}

		info.TopReputationProxy = &struct {
			ProxyID uint64  `json:"proxy_id"`
			IP      string  `json:"ip"`
			Port    uint16  `json:"port"`
			Score   float32 `json:"score"`
			Label   string  `json:"label"`
		}{
			ProxyID: topRow.ProxyID,
			IP:      ip,
			Port:    uint16(topRow.Port),
			Score:   topRow.Score,
			Label:   topRow.Label,
		}
	}

	return info
}

func ChangePassword(userID uint, password string) error {
	err := DB.Model(&domain.User{}).Where("ID = ?", userID).Update("password", password).Error
	return err
}
