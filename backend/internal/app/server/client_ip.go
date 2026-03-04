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
	trustedCIDRs := trustedProxyCIDRsFromEnv()
	if !ipIsTrustedByCIDRs(remoteIP, trustedCIDRs) {
		return remoteIP
	}

	forwardedChain := parseForwardedForHeader(r.Header.Get("X-Forwarded-For"))
	if len(forwardedChain) > 0 {
		hops := make([]string, 0, len(forwardedChain)+1)
		hops = append(hops, forwardedChain...)
		hops = append(hops, remoteIP)
		for i := len(hops) - 1; i >= 0; i-- {
			if !ipIsTrustedByCIDRs(hops[i], trustedCIDRs) {
				return hops[i]
			}
		}
		return remoteIP
	}

	if realIP := parseIP(r.Header.Get("X-Real-IP")); realIP != "" && !ipIsTrustedByCIDRs(realIP, trustedCIDRs) {
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

func getAuthRateLimitKey(r *http.Request) string {
	ip := getAuthClientIP(r)
	if r == nil {
		return ip
	}

	// Trust verified proxy chains as-is.
	if remoteAddrIsTrustedProxy(r.RemoteAddr) {
		return ip
	}

	// If we're likely behind an untrusted reverse proxy, fold stable request hints
	// into the key to avoid collapsing all users onto one shared remote IP.
	if !requestHasForwardedClientIPHeaders(r) || !ipLikelyProxyAddress(remoteAddrIP(r.RemoteAddr)) {
		return ip
	}

	fingerprint := strings.TrimSpace(strings.ToLower(strings.Join([]string{
		strings.TrimSpace(r.Header.Get("X-Forwarded-For")),
		strings.TrimSpace(r.Header.Get("X-Real-IP")),
		strings.TrimSpace(r.UserAgent()),
	}, "|")))
	if fingerprint == "" {
		return ip
	}

	return ip + ":" + hashIdentifier(fingerprint)
}

func remoteAddrIsTrustedProxy(remoteAddr string) bool {
	remoteIP := parseIP(remoteAddr)
	return ipIsTrustedByCIDRs(remoteIP, trustedProxyCIDRsFromEnv())
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
		ones, bits := cidr.Mask.Size()
		if ones == 0 && (bits == 32 || bits == 128) {
			// Ignore wildcard trust ranges (0.0.0.0/0, ::/0).
			continue
		}
		trusted = append(trusted, cidr)
	}

	return trusted
}

func parseForwardedForHeader(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	ips := make([]string, 0, len(parts))
	for _, part := range parts {
		if ip := parseIP(part); ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips
}

func requestHasForwardedClientIPHeaders(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.TrimSpace(r.Header.Get("X-Forwarded-For")) != "" || strings.TrimSpace(r.Header.Get("X-Real-IP")) != ""
}

func ipLikelyProxyAddress(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func ipIsTrustedByCIDRs(raw string, trusted []*net.IPNet) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	parsed := net.ParseIP(strings.TrimSpace(raw))
	if parsed == nil {
		return false
	}
	for _, cidr := range trusted {
		if cidr != nil && cidr.Contains(parsed) {
			return true
		}
	}
	return false
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
