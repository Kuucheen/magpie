package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"magpie/internal/config"
	"magpie/internal/support"
)

const (
	envAllowPublicObservabilityEndpoints = "ALLOW_PUBLIC_OBSERVABILITY_ENDPOINTS"
	envObservabilityToken                = "OBSERVABILITY_TOKEN"
	headObservabilityToken               = "X-Observability-Token"
)

func withObservabilityProtection(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if observabilityRequestAllowed(r) {
			next.ServeHTTP(w, r)
			return
		}

		writeError(w, "Forbidden", http.StatusForbidden)
	})
}

func observabilityRequestAllowed(r *http.Request) bool {
	if observabilityPublicEndpointsEnabled() {
		return true
	}

	if observabilityRequestIsLoopback(r) {
		return true
	}

	configuredToken := strings.TrimSpace(support.GetEnv(envObservabilityToken, ""))
	if configuredToken == "" {
		return false
	}

	requestToken := strings.TrimSpace(r.Header.Get(headObservabilityToken))
	if requestToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(requestToken), []byte(configuredToken)) == 1
}

func observabilityPublicEndpointsEnabled() bool {
	defaultAllowPublic := !config.InProductionMode
	return support.GetEnvBool(envAllowPublicObservabilityEndpoints, defaultAllowPublic)
}

func observabilityRequestIsLoopback(r *http.Request) bool {
	if r == nil {
		return false
	}

	clientIP := clientIPFromRequest(r)
	parsed := net.ParseIP(strings.TrimSpace(clientIP))
	return parsed != nil && parsed.IsLoopback()
}
