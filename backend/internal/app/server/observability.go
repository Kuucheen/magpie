package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"magpie/internal/auth"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

func withRequestID(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := sanitizeRequestID(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = generateRequestID()
		}

		w.Header().Set(requestIDHeader, requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))
		next.ServeHTTP(w, r)
	})
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return value
	}
	return ""
}

func requestIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}

	if requestID := requestIDFromContext(r.Context()); requestID != "" {
		return requestID
	}

	return sanitizeRequestID(r.Header.Get(requestIDHeader))
}

func withPanicRecovery(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("Recovered panic in HTTP handler",
					"request_id", requestIDFromRequest(r),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", recovered,
					"stack", string(debug.Stack()),
				)

				if recorder, ok := w.(*statusRecorder); ok && recorder.HeaderWritten() {
					return
				}

				writeError(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func withAccessLog(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := newStatusRecorder(w)
		start := time.Now()

		next.ServeHTTP(recorder, r)
		duration := time.Since(start)
		observeRequestMetrics(r, recorder.StatusCode(), duration)

		pathTemplate := strings.TrimSpace(r.Pattern)
		if pathTemplate == "" {
			pathTemplate = r.URL.Path
		}

		fields := []any{
			"request_id", requestIDFromRequest(r),
			"method", r.Method,
			"path", r.URL.Path,
			"route", pathTemplate,
			"status", recorder.StatusCode(),
			"latency_ms", duration.Milliseconds(),
			"bytes", recorder.bytes,
			"remote_ip", clientIPFromRequest(r),
		}

		if userID, err := auth.GetUserIDFromRequest(r); err == nil && userID > 0 {
			fields = append(fields, "user_id", userID)
		}

		log.Info("HTTP request", fields...)
	})
}

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			if candidate := strings.TrimSpace(parts[0]); candidate != "" {
				return candidate
			}
		}
	}

	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		return value
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func sanitizeRequestID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	if len(trimmed) > 128 {
		trimmed = trimmed[:128]
	}

	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == ':' || r == '.' {
			continue
		}
		return ""
	}

	return trimmed
}

func generateRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}

	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) ReadFrom(src io.Reader) (int64, error) {
	readerFrom, ok := r.ResponseWriter.(io.ReaderFrom)
	if !ok {
		return io.Copy(r, src)
	}
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := readerFrom.ReadFrom(src)
	r.bytes += int(n)
	return n, err
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijacker not supported")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (r *statusRecorder) StatusCode() int {
	return r.status
}

func (r *statusRecorder) HeaderWritten() bool {
	return r.wroteHeader
}
