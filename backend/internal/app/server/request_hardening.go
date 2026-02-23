package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"magpie/internal/support"

	"github.com/charmbracelet/log"
)

const (
	envServerReadTimeoutSeconds       = "SERVER_READ_TIMEOUT_SECONDS"
	envServerReadHeaderTimeoutSeconds = "SERVER_READ_HEADER_TIMEOUT_SECONDS"
	envServerWriteTimeoutSeconds      = "SERVER_WRITE_TIMEOUT_SECONDS"
	envServerIdleTimeoutSeconds       = "SERVER_IDLE_TIMEOUT_SECONDS"
	envServerShutdownTimeoutSeconds   = "SERVER_SHUTDOWN_TIMEOUT_SECONDS"
	envCORSAllowedOrigins             = "CORS_ALLOWED_ORIGINS"
	envAPIUploadMaxBodyBytes          = "API_UPLOAD_MAX_BODY_BYTES"
	envAPIJSONMaxBodyBytes            = "API_JSON_MAX_BODY_BYTES"
	envAPIMultipartMemoryBytes        = "API_MULTIPART_MEMORY_BYTES"

	defaultServerReadTimeout       = 30 * time.Second
	defaultServerReadHeaderTimeout = 10 * time.Second
	defaultServerWriteTimeout      = 30 * time.Second
	defaultServerIdleTimeout       = 120 * time.Second
	defaultServerShutdownTimeout   = 20 * time.Second

	defaultCORSAllowedOrigins = "http://localhost:5050,http://127.0.0.1:5050,http://localhost:4200,http://127.0.0.1:4200"

	defaultAPIUploadMaxBodyBytes   int64 = 10 << 20 // 10 MiB
	defaultAPIJSONMaxBodyBytes     int64 = 1 << 20  // 1 MiB
	defaultAPIMultipartMemoryBytes int64 = 1 << 20  // 1 MiB
)

type serverTimeouts struct {
	readTimeout       time.Duration
	readHeaderTimeout time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
}

type corsConfig struct {
	allowAll bool
	allowed  map[string]struct{}
}

func resolveServerTimeouts() serverTimeouts {
	return serverTimeouts{
		readTimeout:       time.Duration(resolvePositiveEnvInt(envServerReadTimeoutSeconds, int(defaultServerReadTimeout/time.Second))) * time.Second,
		readHeaderTimeout: time.Duration(resolvePositiveEnvInt(envServerReadHeaderTimeoutSeconds, int(defaultServerReadHeaderTimeout/time.Second))) * time.Second,
		writeTimeout:      time.Duration(resolvePositiveEnvInt(envServerWriteTimeoutSeconds, int(defaultServerWriteTimeout/time.Second))) * time.Second,
		idleTimeout:       time.Duration(resolvePositiveEnvInt(envServerIdleTimeoutSeconds, int(defaultServerIdleTimeout/time.Second))) * time.Second,
	}
}

func resolveServerShutdownTimeout() time.Duration {
	seconds := resolvePositiveEnvInt(
		envServerShutdownTimeoutSeconds,
		int(defaultServerShutdownTimeout/time.Second),
	)
	return time.Duration(seconds) * time.Second
}

func resolveCORSConfig() corsConfig {
	raw := strings.TrimSpace(support.GetEnv(envCORSAllowedOrigins, defaultCORSAllowedOrigins))
	config := corsConfig{
		allowed: make(map[string]struct{}),
	}

	if raw == "" {
		return config
	}

	for _, value := range strings.Split(raw, ",") {
		origin := normalizeOrigin(value)
		if origin == "" {
			continue
		}
		if origin == "*" {
			config.allowAll = true
			continue
		}
		config.allowed[origin] = struct{}{}
	}

	if config.allowAll {
		log.Warn("CORS is configured to allow all origins via CORS_ALLOWED_ORIGINS='*'")
	}

	return config
}

func (cfg corsConfig) isAllowed(origin string) bool {
	if cfg.allowAll {
		return true
	}
	if len(cfg.allowed) == 0 {
		return false
	}
	_, ok := cfg.allowed[normalizeOrigin(origin)]
	return ok
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(strings.ToLower(origin))
	return strings.TrimRight(origin, "/")
}

func isSameHostOrigin(origin string, requestHost string) bool {
	if strings.TrimSpace(origin) == "" || strings.TrimSpace(requestHost) == "" {
		return false
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false
	}

	originHostname := strings.TrimSpace(strings.ToLower(parsedOrigin.Hostname()))
	if originHostname == "" {
		return false
	}

	requestHostname := strings.TrimSpace(strings.ToLower(requestHost))
	if host, _, splitErr := net.SplitHostPort(requestHost); splitErr == nil {
		requestHostname = strings.TrimSpace(strings.ToLower(host))
	}

	return originHostname == requestHostname
}

func resolveUploadMaxBodyBytes() int64 {
	return resolvePositiveEnvInt64(envAPIUploadMaxBodyBytes, defaultAPIUploadMaxBodyBytes)
}

func resolveJSONMaxBodyBytes() int64 {
	return resolvePositiveEnvInt64(envAPIJSONMaxBodyBytes, defaultAPIJSONMaxBodyBytes)
}

func resolveMultipartMemoryBytes() int64 {
	return resolvePositiveEnvInt64(envAPIMultipartMemoryBytes, defaultAPIMultipartMemoryBytes)
}

func prepareMultipartForm(w http.ResponseWriter, r *http.Request, maxBodyBytes int64, maxMemoryBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	if err := r.ParseMultipartForm(maxMemoryBytes); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(w, requestBodyTooLargeMessage(maxBodyBytes), http.StatusRequestEntityTooLarge)
			return false
		}
		writeError(w, "Invalid multipart form data", http.StatusBadRequest)
		return false
	}

	return true
}

func decodeJSONBodyLimited(w http.ResponseWriter, r *http.Request, target any, maxBodyBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(w, requestBodyTooLargeMessage(maxBodyBytes), http.StatusRequestEntityTooLarge)
			return false
		}
		writeError(w, "Invalid request", http.StatusBadRequest)
		return false
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if isRequestBodyTooLarge(err) {
			writeError(w, requestBodyTooLargeMessage(maxBodyBytes), http.StatusRequestEntityTooLarge)
			return false
		}
		writeError(w, "Invalid request", http.StatusBadRequest)
		return false
	}

	return true
}

func applyRequestBodyLimit(next http.Handler, maxBodyBytes int64) http.Handler {
	if next == nil || maxBodyBytes <= 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}

		if r.ContentLength > maxBodyBytes {
			writeError(w, requestBodyTooLargeMessage(maxBodyBytes), http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func isRequestBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return true
	}

	return strings.Contains(strings.ToLower(err.Error()), "request body too large")
}

func requestBodyTooLargeMessage(limitBytes int64) string {
	if limitBytes <= 0 {
		return "Request body too large."
	}

	const oneMiB int64 = 1024 * 1024
	if limitBytes%oneMiB == 0 {
		return fmt.Sprintf("Request body too large. Maximum allowed size is %d MB.", limitBytes/oneMiB)
	}

	return fmt.Sprintf("Request body too large. Maximum allowed size is %d bytes.", limitBytes)
}

func resolvePositiveEnvInt(key string, fallback int) int {
	value := support.GetEnvInt(key, fallback)
	if value <= 0 {
		return fallback
	}
	return value
}

func resolvePositiveEnvInt64(key string, fallback int64) int64 {
	raw := strings.TrimSpace(support.GetEnv(key, ""))
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
