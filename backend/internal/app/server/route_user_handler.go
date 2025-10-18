package server

import (
	"context"
	"encoding/json"
	"errors"
	"magpie/internal/api/dto"
	"magpie/internal/app/bootstrap"
	"magpie/internal/auth"
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

	var newConfig config.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		log.Error("Error decoding request body:", err)
		writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	config.SetConfig(newConfig)

	if strings.TrimSpace(newConfig.GeoLite.APIKey) != "" {
		go jobruntime.RunGeoLiteUpdate(context.Background(), "config-save", true)
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
