package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"

	"magpie/internal/api/dto"
	"magpie/internal/auth"
	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/jobs/runtime"
	"magpie/internal/rotatingproxy"
	"magpie/internal/support"
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

	instanceID := strings.TrimSpace(payload.InstanceID)
	if instanceID == "" {
		writeError(w, "instance_id is required", http.StatusBadRequest)
		return
	}

	availableInstances, listErr := loadAvailableRotatorInstances()
	if listErr != nil {
		writeError(w, "Failed to load rotating proxy instances", http.StatusInternalServerError)
		return
	}
	if len(availableInstances) == 0 {
		writeError(w, "No instances with free rotator ports are currently available", http.StatusServiceUnavailable)
		return
	}

	var selected *dto.RotatingProxyInstance
	for idx := range availableInstances {
		if availableInstances[idx].ID == instanceID {
			selected = &availableInstances[idx]
			break
		}
	}
	if selected == nil {
		writeError(w, "Selected instance is unavailable or has no free ports", http.StatusBadRequest)
		return
	}

	payload.InstanceID = selected.ID
	payload.InstanceName = selected.Name
	payload.InstanceRegion = selected.Region

	proxy, createErr := database.CreateRotatingProxy(userID, payload)
	if createErr != nil {
		writeRotatingProxyError(w, createErr)
		return
	}

	if proxy.InstanceID == support.GetInstanceID() {
		if err := rotatingproxy.GlobalManager.Add(proxy.ID); err != nil {
			log.Error("rotating proxy: failed to start listener", "rotator_id", proxy.ID, "error", err)
			_ = database.DeleteRotatingProxy(userID, proxy.ID)
			writeError(w, "Failed to start rotating proxy listener", http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, http.StatusCreated, proxy)
}

func listRotatingProxyInstances(w http.ResponseWriter, _ *http.Request) {
	instances, err := loadAvailableRotatorInstances()
	if err != nil {
		writeError(w, "Failed to load rotating proxy instances", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"instances": instances,
	})
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

func loadAvailableRotatorInstances() ([]dto.RotatingProxyInstance, error) {
	activeInstances, err := discoverActiveInstances()
	if err != nil {
		return nil, err
	}

	if len(activeInstances) == 0 {
		current := runtime.CurrentInstance()
		activeInstances = append(activeInstances, current)
	}

	instanceIDs := make([]string, 0, len(activeInstances))
	for _, instance := range activeInstances {
		if id := strings.TrimSpace(instance.ID); id != "" {
			instanceIDs = append(instanceIDs, id)
		}
	}

	usageByInstance, err := database.CountRotatingProxiesByInstanceIDs(instanceIDs)
	if err != nil {
		return nil, err
	}

	instances := make([]dto.RotatingProxyInstance, 0, len(activeInstances))
	for _, instance := range activeInstances {
		id := strings.TrimSpace(instance.ID)
		if id == "" {
			continue
		}
		start := instance.PortStart
		end := instance.PortEnd
		if start <= 0 || end <= 0 || end < start {
			start, end = support.GetRotatingProxyPortRange()
		}
		total := end - start + 1
		if total < 0 {
			total = 0
		}
		used := usageByInstance[id]
		free := total - used
		if free < 0 {
			free = 0
		}
		if free == 0 {
			continue
		}

		name := strings.TrimSpace(instance.Name)
		if name == "" {
			name = id
		}
		region := strings.TrimSpace(instance.Region)
		if region == "" {
			region = "Unknown"
		}

		instances = append(instances, dto.RotatingProxyInstance{
			ID:         id,
			Name:       name,
			Region:     region,
			PortStart:  start,
			PortEnd:    end,
			UsedPorts:  used,
			FreePorts:  free,
			TotalPorts: total,
		})
	}

	sort.Slice(instances, func(i, j int) bool {
		left := strings.ToLower(instances[i].Region + ":" + instances[i].Name + ":" + instances[i].ID)
		right := strings.ToLower(instances[j].Region + ":" + instances[j].Name + ":" + instances[j].ID)
		return left < right
	})

	return instances, nil
}

func discoverActiveInstances() ([]runtime.ActiveInstance, error) {
	client, err := support.GetRedisClient()
	if err != nil {
		return []runtime.ActiveInstance{runtime.CurrentInstance()}, nil
	}

	instances, err := runtime.ListActiveInstances(context.Background(), client)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return []runtime.ActiveInstance{runtime.CurrentInstance()}, nil
	}
	return instances, nil
}
