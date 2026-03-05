package server

import (
	"net/http"

	"magpie/internal/support"
)

const (
	envSecurityHeadersEnabled = "SECURITY_HEADERS_ENABLED"
)

func withSecurityHeaders(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if securityHeadersEnabled() {
			applyBaseSecurityHeaders(w)
		}

		next.ServeHTTP(w, r)
	})
}

func securityHeadersEnabled() bool {
	return support.GetEnvBool(envSecurityHeadersEnabled, true)
}

func applyBaseSecurityHeaders(w http.ResponseWriter) {
	if w == nil {
		return
	}
	header := w.Header()
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
}
