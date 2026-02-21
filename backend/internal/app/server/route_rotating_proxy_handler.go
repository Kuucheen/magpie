package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"

	"magpie/internal/api/dto"
	"magpie/internal/auth"
	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/rotatingproxy"
)

func listRotatingProxies(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromRequest(r)
	if err != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	proxies, dbErr := database.ListRotatingProxies(userID)
	if dbErr != nil {
		writeError(w, "Failed to load rotating proxies", http.StatusInternalServerError)
		return
	}

	rotatorHost := strings.TrimSpace(config.GetCurrentIp())
	if rotatorHost == "" {
		rotatorHost = strings.TrimSpace(r.Host)
	}
	if rotatorHost != "" {
		if parsedHost, _, err := net.SplitHostPort(rotatorHost); err == nil {
			rotatorHost = parsedHost
		}
	}

	for idx := range proxies {
		proxies[idx].ListenHost = rotatorHost
		if rotatorHost != "" {
			proxies[idx].ListenAddress = fmt.Sprintf("%s:%d", rotatorHost, proxies[idx].ListenPort)
		} else {
			proxies[idx].ListenAddress = fmt.Sprintf("%d", proxies[idx].ListenPort)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"rotating_proxies": proxies})
}

func createRotatingProxy(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromRequest(r)
	if err != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload dto.RotatingProxyCreateRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
		writeError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	proxy, createErr := database.CreateRotatingProxy(userID, payload)
	if createErr != nil {
		writeRotatingProxyError(w, createErr)
		return
	}

	if err := rotatingproxy.GlobalManager.Add(proxy.ID); err != nil {
		log.Error("rotating proxy: failed to start listener", "rotator_id", proxy.ID, "error", err)
		_ = database.DeleteRotatingProxy(userID, proxy.ID)
		writeError(w, "Failed to start rotating proxy listener", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, proxy)
}

func deleteRotatingProxy(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromRequest(r)
	if err != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		writeError(w, "Missing rotating proxy id", http.StatusBadRequest)
		return
	}

	id, convErr := strconv.ParseUint(rawID, 10, 64)
	if convErr != nil {
		writeError(w, "Invalid rotating proxy id", http.StatusBadRequest)
		return
	}

	if err := database.DeleteRotatingProxy(userID, id); err != nil {
		writeRotatingProxyError(w, err)
		return
	}

	rotatingproxy.GlobalManager.Remove(id)

	w.WriteHeader(http.StatusNoContent)
}

func getNextRotatingProxy(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromRequest(r)
	if err != nil {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		writeError(w, "Missing rotating proxy id", http.StatusBadRequest)
		return
	}

	id, convErr := strconv.ParseUint(rawID, 10, 64)
	if convErr != nil {
		writeError(w, "Invalid rotating proxy id", http.StatusBadRequest)
		return
	}

	nextProxy, dbErr := database.GetNextRotatingProxy(userID, id)
	if dbErr != nil {
		writeRotatingProxyError(w, dbErr)
		return
	}

	writeJSON(w, http.StatusOK, nextProxy)
}

func writeRotatingProxyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrRotatingProxyNameRequired),
		errors.Is(err, database.ErrRotatingProxyNameTooLong),
		errors.Is(err, database.ErrRotatingProxyProtocolMissing),
		errors.Is(err, database.ErrRotatingProxyProtocolDenied),
		errors.Is(err, database.ErrRotatingProxyAuthUsernameNeeded),
		errors.Is(err, database.ErrRotatingProxyAuthPasswordNeeded),
		errors.Is(err, database.ErrRotatingProxyUptimeTypeInvalid),
		errors.Is(err, database.ErrRotatingProxyUptimeTypeMissing),
		errors.Is(err, database.ErrRotatingProxyUptimeValueMissing),
		errors.Is(err, database.ErrRotatingProxyUptimeOutOfRange):
		writeError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, database.ErrRotatingProxyNameConflict):
		writeError(w, err.Error(), http.StatusConflict)
	case errors.Is(err, database.ErrRotatingProxyPortExhausted):
		writeError(w, err.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, database.ErrRotatingProxyNotFound):
		writeError(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, database.ErrRotatingProxyNoAliveProxies):
		writeError(w, err.Error(), http.StatusConflict)
	default:
		writeError(w, "Internal server error", http.StatusInternalServerError)
	}
}
