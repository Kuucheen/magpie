package server

import (
	"net"
	"net/http"
	"os"
	"strings"
)

const envTrustedProxyCIDRs = "TRUSTED_PROXY_CIDRS"

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}

	remoteIP := remoteAddrIP(r.RemoteAddr)
	if !remoteAddrIsTrustedProxy(r.RemoteAddr) {
		return remoteIP
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		for _, part := range strings.Split(forwarded, ",") {
			if ip := parseIP(part); ip != "" {
				return ip
			}
		}
	}

	if realIP := parseIP(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	return remoteIP
}

func getAuthClientIP(r *http.Request) string {
	ip := clientIPFromRequest(r)
	if strings.TrimSpace(ip) == "" {
		return "unknown"
	}
	return ip
}

func remoteAddrIsTrustedProxy(remoteAddr string) bool {
	remoteIP := parseIP(remoteAddr)
	if remoteIP == "" {
		return false
	}

	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}

	for _, cidr := range trustedProxyCIDRsFromEnv() {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

func trustedProxyCIDRsFromEnv() []*net.IPNet {
	raw := strings.TrimSpace(os.Getenv(envTrustedProxyCIDRs))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	trusted := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(part)
		if err != nil {
			continue
		}
		trusted = append(trusted, cidr)
	}

	return trusted
}

func remoteAddrIP(remoteAddr string) string {
	if ip := parseIP(remoteAddr); ip != "" {
		return ip
	}
	return strings.TrimSpace(remoteAddr)
}

func parseIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}

	parsed := net.ParseIP(raw)
	if parsed == nil {
		return ""
	}

	return parsed.String()
}
