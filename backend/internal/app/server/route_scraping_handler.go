package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"magpie/internal/api/dto"
	"magpie/internal/auth"
	"magpie/internal/config"
	"magpie/internal/database"
	sitequeue "magpie/internal/jobs/queue/sites"
	"magpie/internal/jobs/scraper"
	"magpie/internal/support"
)

func getScrapeSourcesCount(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(database.GetAllScrapeSiteCountOfUser(userID))
}

func getScrapeSourcePage(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	page, err := strconv.Atoi(r.PathValue("page"))
	if err != nil {
		log.Error("error converting page to int", "error", err.Error())
		writeError(w, "Invalid page", http.StatusBadRequest)
		return
	}

	scrapeSiteInfoPages := database.GetScrapeSiteInfoPage(userID, page)

	json.NewEncoder(w).Encode(scrapeSiteInfoPages)
}

func getScrapeSourceProxies(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sourceID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Error("error converting scrape source id", "error", err.Error())
		writeError(w, "Invalid scrape source id", http.StatusBadRequest)
		return
	}

	page := 1
	if rawPage := strings.TrimSpace(r.URL.Query().Get("page")); rawPage != "" {
		if parsed, parseErr := strconv.Atoi(rawPage); parseErr == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 0
	if rawPageSize := strings.TrimSpace(r.URL.Query().Get("pageSize")); rawPageSize != "" {
		if parsed, parseErr := strconv.Atoi(rawPageSize); parseErr == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))

	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "alive" && status != "dead" {
		status = ""
	}

	filters := dto.ProxyListFilters{
		Status:           status,
		Protocols:        normalizeQueryList(r.URL.Query()["protocol"]),
		Countries:        normalizeQueryList(r.URL.Query()["country"]),
		Types:            normalizeQueryList(r.URL.Query()["type"]),
		AnonymityLevels:  normalizeQueryList(r.URL.Query()["anonymity"]),
		MaxTimeout:       parsePositiveIntParam(r.URL.Query().Get("maxTimeout")),
		MaxRetries:       parsePositiveIntParam(r.URL.Query().Get("maxRetries")),
		ReputationLabels: normalizeQueryList(r.URL.Query()["reputation"]),
	}

	proxies, total, dbErr := database.GetScrapeSiteProxyPage(userID, sourceID, page, pageSize, search, filters)
	if dbErr != nil {
		log.Error("error retrieving scrape source proxies", "error", dbErr.Error(), "scrape_source_id", sourceID)
		writeError(w, "Failed to retrieve scrape source proxies", http.StatusInternalServerError)
		return
	}

	response := dto.ProxyPage{
		Proxies: proxies,
		Total:   total,
	}

	json.NewEncoder(w).Encode(response)
}

func getScrapeSourceDetail(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sourceID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Error("error converting scrape source id", "error", err.Error())
		writeError(w, "Invalid scrape source id", http.StatusBadRequest)
		return
	}

	detail, dbErr := database.GetScrapeSiteDetail(userID, sourceID)
	if dbErr != nil {
		log.Error("error retrieving scrape source detail", "error", dbErr.Error(), "scrape_source_id", sourceID)
		writeError(w, "Failed to retrieve scrape source", http.StatusInternalServerError)
		return
	}

	if detail == nil {
		writeError(w, "Scrape source not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(detail)
}

func deleteScrapingSources(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var scrapingSource []int

	if err := json.NewDecoder(r.Body).Decode(&scrapingSource); err != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	deleted, orphaned, deleteErr := database.DeleteScrapeSiteRelation(userID, scrapingSource)
	if deleteErr != nil {
		log.Error("could not delete scrape sites", "error", deleteErr.Error())
		writeError(w, "Could not delete scraping sources", http.StatusInternalServerError)
		return
	}

	if len(orphaned) > 0 {
		if err := sitequeue.PublicScrapeSiteQueue.RemoveFromQueue(orphaned); err != nil {
			log.Error("failed to remove scrape sites from queue", "error", err, "count", len(orphaned))
		}
	}

	if deleted == 0 {
		json.NewEncoder(w).Encode("No scraping sources matched the delete criteria.")
		return
	}

	json.NewEncoder(w).Encode(fmt.Sprintf("Deleted %d scraping sources.", deleted))
}

func saveScrapingSources(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromRequest(r)
	if err != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	textareaContent := r.FormValue("scrapeSourceTextarea") // Match the key sent by frontend
	clipboardContent := r.FormValue("clipboardScrapeSources")
	file, fileHeader, err := r.FormFile("file") // "file" is the key of the form field

	var fileContent []byte

	if err == nil {
		defer file.Close()

		log.Debugf("Uploaded file: %s (%d bytes)", fileHeader.Filename, fileHeader.Size)

		fileContent, err = io.ReadAll(file)
		if err != nil {
			writeError(w, "Failed to read file", http.StatusInternalServerError)
			return
		}

	} else if len(textareaContent) == 0 && len(clipboardContent) == 0 {
		writeError(w, "Failed to retrieve sources from any input method", http.StatusBadRequest)
		return
	}

	// Merge the file content and the textarea content
	mergedContent := string(fileContent) + "\n" + textareaContent + "\n" + clipboardContent

	log.Infof("Sources content received: %d bytes", len(mergedContent))

	// Parse the merged content into a slice of sources
	sources := support.ParseTextToSources(mergedContent)

	var allowedSources, blockedSources []string
	for _, src := range sources {
		if config.IsWebsiteBlocked(src) {
			blockedSources = append(blockedSources, src)
			continue
		}
		allowedSources = append(allowedSources, src)
	}

	if len(blockedSources) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":            "One or more scrape sources are blocked",
			"blocked_sources":  dedupeStrings(blockedSources),
			"websiteBlacklist": config.GetConfig().WebsiteBlacklist,
		})
		return
	}

	sites, err := database.SaveScrapingSourcesOfUsers(userID, allowedSources)
	if err != nil {
		log.Error("Could not save sources to database", "error", err.Error())
		writeError(w, "Could not save sources to database", http.StatusInternalServerError)
		return
	}

	sitequeue.PublicScrapeSiteQueue.AddToQueue(sites)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"sourceCount": len(sites)})
}

func checkScrapeSourceRobots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, err := auth.GetUserIDFromRequest(r); err != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		writeError(w, "Missing url query parameter", http.StatusBadRequest)
		return
	}
	if config.IsWebsiteBlocked(rawURL) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":            "This website is blocked",
			"blocked_website":  rawURL,
			"websiteBlacklist": config.GetConfig().WebsiteBlacklist,
		})
		return
	}

	result, err := scraper.CheckRobotsAllowance(rawURL, 10*time.Second)
	if err != nil {
		log.Warn("robots.txt check", "url", rawURL, "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"allowed":      result.Allowed,
		"robots_found": result.RobotsFound,
		"error":        errString(err),
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

func getRobotsRespectSetting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, err := auth.GetUserIDFromRequest(r); err != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cfg := config.GetConfig()

	writeJSON(w, http.StatusOK, map[string]bool{
		"respect_robots_txt": cfg.Scraper.RespectRobots,
	})
}
