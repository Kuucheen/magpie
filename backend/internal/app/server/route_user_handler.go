package server

import (
	"context"
	"encoding/json"
	"errors"
	"magpie/internal/api/dto"
	"magpie/internal/app/bootstrap"
	"magpie/internal/auth"
	"magpie/internal/blacklist"
	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/jobs/checker/judges"
	sitequeue "magpie/internal/jobs/queue/sites"
	jobruntime "magpie/internal/jobs/runtime"
	"magpie/internal/support"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

func checkLogin(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func registerUser(w http.ResponseWriter, r *http.Request) {
	var credentials dto.Credentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user := domain.User{
		Email:    credentials.Email,
		Password: credentials.Password,
	}

	// Validate email format
	if !auth.IsValidEmail(user.Email) {
		writeError(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	// Check if password is provided
	if len(user.Password) < 8 {
		writeError(w, "Password must be at least 8 characters long", http.StatusBadRequest)
		return
	}

	// Hash the password
	hashedPassword, err := support.HashPassword(user.Password)
	if err != nil {
		writeError(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	user.Password = hashedPassword

	// Check if email already exists
	var existingUser domain.User
	if err = database.DB.Where("email = ?", user.Email).First(&existingUser).Error; err == nil {
		writeError(w, "Email already in use", http.StatusConflict)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, "Failed to query database", http.StatusInternalServerError)
		return
	}

	// Check if there are no users in the database and assign admin role
	if err = database.DB.Select("id").Take(&existingUser).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		user.Role = "admin"
	} else {
		user.Role = "user" // just to make sure
	}

	//Set default values
	cfg := config.GetConfig()
	user.HTTPProtocol = cfg.Protocols.HTTP
	user.HTTPSProtocol = cfg.Protocols.HTTPS
	user.SOCKS4Protocol = cfg.Protocols.Socks4
	user.SOCKS5Protocol = cfg.Protocols.Socks5
	user.UseHttpsForSocks = cfg.Checker.UseHttpsForSocks
	user.TransportProtocol = support.TransportTCP

	// Save user to the database
	if err = database.DB.Create(&user).Error; err != nil {
		writeError(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	go bootstrap.AddDefaultJudgesToUsers()
	sites, err := database.SaveScrapingSourcesOfUsers(user.ID, cfg.Scraper.ScrapeSites) // default scrape sites
	if err != nil {
		log.Warn("Could not add default Scraping Sources to user", "err", err)
	} else {
		sitequeue.PublicScrapeSiteQueue.AddToQueue(sites)
	}

	token, err := auth.GenerateJWT(user.ID, user.Role)
	if err != nil {
		writeError(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}

func loginUser(w http.ResponseWriter, r *http.Request) {
	var credentials dto.Credentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var user domain.User
	if err := database.DB.Where("email = ?", credentials.Email).First(&user).Error; err != nil {
		writeError(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Compare passwords
	if !support.CheckPasswordHash(credentials.Password, user.Password) {
		writeError(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Generate token
	token, err := auth.GenerateJWT(user.ID, user.Role)
	if err != nil {
		writeError(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token, "role": user.Role})
}

func saveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	previousCfg := config.GetConfig()

	var newConfig config.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		log.Error("Error decoding request body:", err)
		writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	newConfig.WebsiteBlacklist = config.NormalizeWebsiteBlacklist(newConfig.WebsiteBlacklist)
	blockedSet := config.NewWebsiteBlocklistSet(newConfig.WebsiteBlacklist)

	blocked := make(map[string][]string)
	judgeURLs := make([]string, 0, len(newConfig.Checker.Judges))
	for _, j := range newConfig.Checker.Judges {
		if strings.TrimSpace(j.URL) != "" {
			judgeURLs = append(judgeURLs, j.URL)
		}
	}
	if blockedJudges := config.FindBlockedURLs(judgeURLs, blockedSet); len(blockedJudges) > 0 {
		blocked["judges"] = dedupe(blockedJudges)
	}
	if blockedScrape := config.FindBlockedURLs(newConfig.Scraper.ScrapeSites, blockedSet); len(blockedScrape) > 0 {
		blocked["scrape_sites"] = dedupe(blockedScrape)
	}
	if blockedSources := config.FindBlockedURLs(newConfig.BlacklistSources, blockedSet); len(blockedSources) > 0 {
		blocked["blacklist_sources"] = dedupe(blockedSources)
	}
	if blockedLookup := config.FindBlockedURLs([]string{newConfig.Checker.IpLookup}, blockedSet); len(blockedLookup) > 0 {
		blocked["ip_lookup"] = dedupe(blockedLookup)
	}

	if len(blocked) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":             "One or more URLs are blocked by website blacklist",
			"blocked_websites":  blocked,
			"website_blacklist": newConfig.WebsiteBlacklist,
		})
		return
	}

	config.SetConfig(newConfig)

	if len(newConfig.WebsiteBlacklist) > 0 {
		cleanupResult, err := database.RemoveBlockedWebsitesFromUsers(context.Background(), newConfig.WebsiteBlacklist)
		if err != nil {
			log.Error("Failed to purge blocked websites from users", "error", err)
			writeError(w, "Failed to remove blocked websites from users", http.StatusInternalServerError)
			return
		}

		refreshUserJudgeCache(cleanupResult.UpdatedUserJudges)

		if len(cleanupResult.BlockedScrapeSites) > 0 {
			if err := sitequeue.PublicScrapeSiteQueue.RemoveFromQueue(cleanupResult.BlockedScrapeSites); err != nil {
				log.Warn("Failed to remove blocked scrape sites from queue", "error", err, "count", len(cleanupResult.BlockedScrapeSites))
			}
		}

		if cleanupResult.ScrapeRelationsRemoved > 0 {
			if removed, err := database.DeleteOrphanScrapeSites(context.Background()); err != nil {
				log.Warn("Failed to delete orphan scrape sites after blacklist update", "error", err)
			} else if removed > 0 {
				log.Info("Deleted orphan scrape sites after blacklist update", "count", removed)
			}
		}

		if cleanupResult.JudgeRelationsRemoved > 0 || cleanupResult.ScrapeRelationsRemoved > 0 {
			log.Info("Purged blocked websites from users",
				"judge_relations_removed", cleanupResult.JudgeRelationsRemoved,
				"scrape_relations_removed", cleanupResult.ScrapeRelationsRemoved,
				"blocked_sites", len(cleanupResult.BlockedScrapeSites),
			)
		}
	}

	if strings.TrimSpace(newConfig.GeoLite.APIKey) != "" {
		go jobruntime.RunGeoLiteUpdate(context.Background(), "config-save", true)
	}

	if hasNewBlacklistSources(previousCfg.BlacklistSources, newConfig.BlacklistSources) {
		go blacklist.RunRefresh(context.Background(), "config-save", true)
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Configuration updated successfully"})
}

func getUserSettings(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user := database.GetUserFromId(userID)
	judges := database.GetUserJudges(userID)
	scrapingSources := database.GetScrapingSourcesOfUsers(userID)

	json.NewEncoder(w).Encode(user.ToUserSettings(judges, scrapingSources))
}

func saveUserSettings(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var userSettings dto.UserSettings
	if err := json.NewDecoder(r.Body).Decode(&userSettings); err != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var blocked []string
	for _, judge := range userSettings.SimpleUserJudges {
		if config.IsWebsiteBlocked(judge.Url) {
			blocked = append(blocked, judge.Url)
		}
	}

	if len(blocked) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":            "One or more judges point to blocked websites",
			"blocked_websites": dedupe(blocked),
		})
		return
	}

	if err := database.UpdateUserSettings(userID, userSettings); err != nil {
		writeError(w, "Internal Server Error", http.StatusInternalServerError)
	}

	var jwrList []domain.JudgeWithRegex
	for _, uj := range userSettings.SimpleUserJudges {
		judgeModel := database.GetJudgeFromString(uj.Url)
		if judgeModel == nil {
			log.Warnf("cannot load judge %s for user %d", uj.Url, userID)
			continue
		}
		judgeModel.SetUp()
		judgeModel.UpdateIp()
		jwrList = append(jwrList, domain.JudgeWithRegex{
			Judge: judgeModel,
			Regex: uj.Regex,
		})
	}

	// atomically replace this user's judges in the global map
	judges.SetUserJudges(userID, jwrList)

	json.NewEncoder(w).Encode(map[string]string{"message": "Settings saved successfully"})
}

func getUserRole(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user := database.GetUserFromId(userID)

	json.NewEncoder(w).Encode(user.Role)
}

func changePassword(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user := database.GetUserFromId(userID)

	var changeUserPassword dto.ChangePassword
	if err := json.NewDecoder(r.Body).Decode(&changeUserPassword); err != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !support.CheckPasswordHash(changeUserPassword.OldPassword, user.Password) {
		writeError(w, "Invalid old password", http.StatusUnauthorized)
		return
	}

	hashed, err := support.HashPassword(changeUserPassword.NewPassword)
	if err != nil {
		writeError(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	err = database.ChangePassword(userID, hashed)
	if err != nil {
		writeError(w, "Failed to change password", http.StatusInternalServerError)
		log.Error(err)
		return
	}

	json.NewEncoder(w).Encode("Password changed successfully")
}

func refreshUserJudgeCache(updates map[uint][]database.UserJudgeAssignment) {
	if len(updates) == 0 {
		return
	}

	for userID, assignments := range updates {
		judgesWithRegex := make([]domain.JudgeWithRegex, 0, len(assignments))

		for _, assignment := range assignments {
			judge := &domain.Judge{
				ID:         assignment.JudgeID,
				FullString: assignment.FullString,
				CreatedAt:  assignment.CreatedAt,
			}

			if err := judge.SetUp(); err != nil {
				log.Warn("Failed to set up judge after website blacklist cleanup", "user_id", userID, "url", assignment.FullString, "error", err)
				continue
			}

			judge.UpdateIp()

			judgesWithRegex = append(judgesWithRegex, domain.JudgeWithRegex{
				Judge: judge,
				Regex: assignment.Regex,
			})
		}

		judges.SetUserJudges(userID, judgesWithRegex)
	}
}

func hasNewBlacklistSources(oldSources, newSources []string) bool {
	existing := make(map[string]struct{}, len(oldSources))
	for _, src := range oldSources {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		existing[src] = struct{}{}
	}

	for _, src := range newSources {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		if _, ok := existing[src]; !ok {
			return true
		}
	}

	return false
}

func dedupe(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := unique[v]; ok {
			continue
		}
		unique[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
