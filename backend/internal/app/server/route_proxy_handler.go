package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"magpie/internal/api/dto"
	"magpie/internal/auth"
	"magpie/internal/blacklist"
	"magpie/internal/database"
	proxyqueue "magpie/internal/jobs/queue/proxy"
	"magpie/internal/support"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"gorm.io/gorm"
)

func addProxies(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	startedAt := time.Now()

	textareaContent := r.FormValue("proxyTextarea") // "proxyTextarea" matches the key sent by the frontend
	clipboardContent := r.FormValue("clipboardProxies")
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
		writeError(w, "Failed to retrieve file", http.StatusBadRequest)
		return
	}

	// Merge the file content and the textarea content
	mergedContent := string(fileContent) + "\n" + textareaContent + "\n" + clipboardContent

	log.Infof("File content received: %d bytes", len(mergedContent))

	proxyList, parseStats := support.ParseTextToProxiesWithStats(mergedContent)
	proxyList, blocked := blacklist.FilterProxies(proxyList)
	if len(blocked) > 0 {
		log.Info("Dropped blacklisted proxies from upload", "count", len(blocked))
	}

	proxyList, err = database.InsertAndGetProxiesWithUser(proxyList, userID)
	if err != nil {
		log.Error("Could not add proxies to database", "error", err.Error())
		writeError(w, "Could not add proxies to database", http.StatusInternalServerError)
		return
	}

	database.AsyncEnrichProxyMetadata(proxyList)
	proxyqueue.PublicProxyQueue.AddToQueue(proxyList)

	w.WriteHeader(http.StatusOK)
	processingMs := time.Since(startedAt).Milliseconds()
	response := dto.AddProxiesResponse{
		ProxyCount: len(proxyList),
		Details: dto.AddProxiesDetails{
			SubmittedCount:     parseStats.SubmittedCount,
			ParsedCount:        parseStats.ParsedCount,
			InvalidFormatCount: parseStats.InvalidFormatCount,
			InvalidIPCount:     parseStats.InvalidIPCount,
			InvalidIPv4Count:   parseStats.InvalidIPv4Count,
			InvalidPortCount:   parseStats.InvalidPortCount,
			BlacklistedCount:   len(blocked),
			ProcessingMs:       processingMs,
		},
	}
	json.NewEncoder(w).Encode(response)
}

func getProxyPage(w http.ResponseWriter, r *http.Request) {
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

	pageSize := 0
	if rawPageSize := r.URL.Query().Get("pageSize"); rawPageSize != "" {
		if parsedPageSize, parseErr := strconv.Atoi(rawPageSize); parseErr == nil && parsedPageSize > 0 {
			pageSize = parsedPageSize
		}
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))

	proxies, total := database.GetProxyInfoPageWithFilters(userID, page, pageSize, search)

	response := dto.ProxyPage{
		Proxies: proxies,
		Total:   total,
	}

	json.NewEncoder(w).Encode(response)
}

func getProxyCount(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(database.GetAllProxyCountOfUser(userID))
}

func getProxyDetail(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	proxyID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Error("error converting proxy id", "error", err.Error())
		writeError(w, "Invalid proxy id", http.StatusBadRequest)
		return
	}

	detail, dbErr := database.GetProxyDetail(userID, proxyID)
	if dbErr != nil {
		log.Error("error retrieving proxy detail", "error", dbErr.Error(), "proxy_id", proxyID)
		writeError(w, "Failed to retrieve proxy", http.StatusInternalServerError)
		return
	}

	if detail == nil {
		writeError(w, "Proxy not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(detail)
}

func getProxyStatistics(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	proxyID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Error("error converting proxy id", "error", err.Error())
		writeError(w, "Invalid proxy id", http.StatusBadRequest)
		return
	}

	limit := 100
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}

	statistics, dbErr := database.GetProxyStatistics(userID, proxyID, limit)
	if dbErr != nil {
		log.Error("error retrieving proxy statistics", "error", dbErr.Error(), "proxy_id", proxyID)
		writeError(w, "Failed to retrieve proxy statistics", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"statistics": statistics})
}

func getProxyStatisticResponseBody(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	proxyID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Error("error converting proxy id", "error", err.Error())
		writeError(w, "Invalid proxy id", http.StatusBadRequest)
		return
	}

	statisticID, err := strconv.ParseUint(r.PathValue("statisticId"), 10, 64)
	if err != nil {
		log.Error("error converting statistic id", "error", err.Error())
		writeError(w, "Invalid statistic id", http.StatusBadRequest)
		return
	}

	responseDetail, dbErr := database.GetProxyStatisticResponseBody(userID, proxyID, statisticID)
	if dbErr != nil {
		if errors.Is(dbErr, gorm.ErrRecordNotFound) {
			writeError(w, "Proxy statistic not found", http.StatusNotFound)
			return
		}

		log.Error("error retrieving proxy statistic body", "error", dbErr.Error(), "proxy_id", proxyID, "statistic_id", statisticID)
		writeError(w, "Failed to retrieve proxy statistic body", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(responseDetail)
}

func deleteProxies(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rawBody, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	body := bytes.TrimSpace(rawBody)

	if len(body) == 0 {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if body[0] == '[' {
		var proxies []int
		if err := json.Unmarshal(body, &proxies); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if len(proxies) == 0 {
			writeError(w, "No proxies selected for deletion", http.StatusBadRequest)
			return
		}

		deleted, orphaned, deleteErr := database.DeleteProxyRelation(userID, proxies)
		if deleteErr != nil {
			log.Error("could not delete proxies", "error", deleteErr.Error())
			writeError(w, "Could not delete proxies", http.StatusInternalServerError)
			return
		}

		if len(orphaned) > 0 {
			if err := proxyqueue.PublicProxyQueue.RemoveFromQueue(orphaned); err != nil {
				log.Error("failed to remove orphaned proxies from queue", "error", err)
			}
		}

		if deleted == 0 {
			json.NewEncoder(w).Encode("No proxies matched the delete criteria.")
			return
		}

		json.NewEncoder(w).Encode(fmt.Sprintf("Deleted %d proxies.", deleted))
		return
	}

	var settings dto.DeleteSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if settings.Scope != "all" && settings.Scope != "selected" {
		settings.Scope = "all"
	}

	deleted, orphaned, deleteErr := database.DeleteProxiesWithSettings(userID, settings)
	if deleteErr != nil {
		if errors.Is(deleteErr, database.ErrNoProxiesSelected) {
			writeError(w, "No proxies selected for deletion", http.StatusBadRequest)
			return
		}

		log.Error("could not delete proxies with filters", "error", deleteErr.Error())
		writeError(w, "Could not delete proxies", http.StatusInternalServerError)
		return
	}

	if len(orphaned) > 0 {
		if err := proxyqueue.PublicProxyQueue.RemoveFromQueue(orphaned); err != nil {
			log.Error("failed to remove orphaned proxies from queue", "error", err)
		}
	}

	if deleted == 0 {
		json.NewEncoder(w).Encode("No proxies matched the delete criteria.")
		return
	}

	json.NewEncoder(w).Encode(fmt.Sprintf("Deleted %d proxies.", deleted))
}

func exportProxies(w http.ResponseWriter, r *http.Request) {
	userID, userErr := auth.GetUserIDFromRequest(r)
	if userErr != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var settings dto.ExportSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	proxies, err := database.GetProxiesForExport(userID, settings)

	if err != nil {
		writeError(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	formattedProxies := support.FormatProxies(proxies, settings.OutputFormat)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=proxies.txt")
	json.NewEncoder(w).Encode(formattedProxies)
}
