package support

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	envAllowPrivateNetworkEgress = "ALLOW_PRIVATE_NETWORK_EGRESS"
)

var ErrUnsafeOutboundTarget = errors.New("unsafe outbound target")

type OutboundTargetError struct {
	Host   string
	IP     string
	Reason string
}

func (e *OutboundTargetError) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "target is not allowed"
	}

	host := strings.TrimSpace(e.Host)
	ip := strings.TrimSpace(e.IP)

	switch {
	case host != "" && ip != "":
		return fmt.Sprintf("%s (%s): %s", host, ip, reason)
	case host != "":
		return fmt.Sprintf("%s: %s", host, reason)
	case ip != "":
		return fmt.Sprintf("%s: %s", ip, reason)
	default:
		return reason
	}
}

func (e *OutboundTargetError) Unwrap() error {
	return ErrUnsafeOutboundTarget
}

type disallowedPrefix struct {
	prefix netip.Prefix
	reason string
}

var disallowedOutboundPrefixes = []disallowedPrefix{
	{prefix: netip.MustParsePrefix("0.0.0.0/8"), reason: "unspecified_ipv4_range"},
	{prefix: netip.MustParsePrefix("10.0.0.0/8"), reason: "private_network"},
	{prefix: netip.MustParsePrefix("100.64.0.0/10"), reason: "carrier_grade_nat"},
	{prefix: netip.MustParsePrefix("127.0.0.0/8"), reason: "loopback"},
	{prefix: netip.MustParsePrefix("169.254.0.0/16"), reason: "link_local"},
	{prefix: netip.MustParsePrefix("172.16.0.0/12"), reason: "private_network"},
	{prefix: netip.MustParsePrefix("192.0.0.0/24"), reason: "ietf_protocol_assignments"},
	{prefix: netip.MustParsePrefix("192.0.2.0/24"), reason: "documentation_range"},
	{prefix: netip.MustParsePrefix("192.168.0.0/16"), reason: "private_network"},
	{prefix: netip.MustParsePrefix("198.18.0.0/15"), reason: "benchmarking_range"},
	{prefix: netip.MustParsePrefix("198.51.100.0/24"), reason: "documentation_range"},
	{prefix: netip.MustParsePrefix("203.0.113.0/24"), reason: "documentation_range"},
	{prefix: netip.MustParsePrefix("224.0.0.0/4"), reason: "multicast_or_reserved"},
	{prefix: netip.MustParsePrefix("240.0.0.0/4"), reason: "reserved_range"},
	{prefix: netip.MustParsePrefix("255.255.255.255/32"), reason: "limited_broadcast"},
	{prefix: netip.MustParsePrefix("::/128"), reason: "unspecified_ipv6"},
	{prefix: netip.MustParsePrefix("::1/128"), reason: "loopback"},
	{prefix: netip.MustParsePrefix("fc00::/7"), reason: "unique_local_address"},
	{prefix: netip.MustParsePrefix("fe80::/10"), reason: "link_local"},
	{prefix: netip.MustParsePrefix("fec0::/10"), reason: "site_local"},
	{prefix: netip.MustParsePrefix("ff00::/8"), reason: "multicast"},
	{prefix: netip.MustParsePrefix("2001:db8::/32"), reason: "documentation_range"},
}

type netIPResolver interface {
	LookupNetIP(ctx context.Context, network string, host string) ([]netip.Addr, error)
}

var outboundResolver netIPResolver = net.DefaultResolver

func PrivateNetworkEgressAllowed() bool {
	return GetEnvBool(envAllowPrivateNetworkEgress, false)
}

func ValidateOutboundHTTPURL(raw string) (*url.URL, error) {
	return ValidateOutboundHTTPURLContext(context.Background(), raw)
}

func ValidateOutboundHTTPURLContext(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := parseOutboundHTTPURL(raw)
	if err != nil {
		return nil, err
	}
	if PrivateNetworkEgressAllowed() {
		return parsed, nil
	}
	if err := validateOutboundHostWithResolver(ctx, parsed.Hostname(), outboundResolver); err != nil {
		return nil, err
	}
	return parsed, nil
}

func ValidateOutboundHTTPLiteral(raw string) (*url.URL, error) {
	parsed, err := parseOutboundHTTPURL(raw)
	if err != nil {
		return nil, err
	}
	if PrivateNetworkEgressAllowed() {
		return parsed, nil
	}
	if err := validateOutboundHostLiterals(parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

func ValidateOutboundIPLiteral(rawIP string) error {
	if PrivateNetworkEgressAllowed() {
		return nil
	}

	trimmed := strings.TrimSpace(rawIP)
	if trimmed == "" {
		return &OutboundTargetError{Reason: "remote ip is empty"}
	}

	parsed := net.ParseIP(trimmed)
	if parsed == nil {
		return &OutboundTargetError{
			IP:     trimmed,
			Reason: "invalid_ip",
		}
	}

	addr, ok := netip.AddrFromSlice(parsed)
	if !ok {
		return &OutboundTargetError{
			IP:     trimmed,
			Reason: "invalid_ip",
		}
	}
	if addr.Is4In6() {
		addr = addr.Unmap()
	}

	if reason, blocked := blockedOutboundAddrReason(addr); blocked {
		return &OutboundTargetError{
			IP:     addr.String(),
			Reason: reason,
		}
	}

	return nil
}

func NewRestrictedOutboundHTTPClient(timeout time.Duration) *http.Client {
	base, _ := http.DefaultTransport.(*http.Transport)
	transport := &http.Transport{}
	if base != nil {
		transport = base.Clone()
	}
	transport.Proxy = nil

	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if PrivateNetworkEgressAllowed() {
			return dialer.DialContext(ctx, network, address)
		}

		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		allowedIPs, err := resolveAllowedOutboundIPs(ctx, host, outboundResolver)
		if err != nil {
			return nil, err
		}

		var dialErr error
		for _, ip := range allowedIPs {
			conn, attemptErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if attemptErr == nil {
				return conn, nil
			}
			dialErr = attemptErr
		}

		if dialErr != nil {
			return nil, dialErr
		}

		return nil, &OutboundTargetError{
			Host:   host,
			Reason: "no allowed upstream addresses",
		}
	}

	client := &http.Client{Transport: transport}
	if timeout > 0 {
		client.Timeout = timeout
	}
	return client
}

func parseOutboundHTTPURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("url is empty")
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q", parsed.Scheme)
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, fmt.Errorf("url host is empty")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("url user info is not allowed")
	}

	return parsed, nil
}

func validateOutboundHostWithResolver(ctx context.Context, host string, resolver netIPResolver) error {
	_, err := resolveAllowedOutboundIPs(ctx, host, resolver)
	return err
}

func resolveAllowedOutboundIPs(ctx context.Context, host string, resolver netIPResolver) ([]netip.Addr, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, &OutboundTargetError{Reason: "url host is empty"}
	}

	if err := validateOutboundHostLiterals(host); err != nil {
		return nil, err
	}

	parsedIP := net.ParseIP(host)
	if parsedIP != nil {
		addr, ok := netip.AddrFromSlice(parsedIP)
		if !ok {
			return nil, &OutboundTargetError{
				Host:   host,
				Reason: "invalid_ip",
			}
		}
		if addr.Is4In6() {
			addr = addr.Unmap()
		}
		return []netip.Addr{addr}, nil
	}

	if resolver == nil {
		resolver = net.DefaultResolver
	}

	lookupCtx := ctx
	if lookupCtx == nil {
		lookupCtx = context.Background()
	}
	if _, hasDeadline := lookupCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		lookupCtx, cancel = context.WithTimeout(lookupCtx, 5*time.Second)
		defer cancel()
	}

	ips, err := resolver.LookupNetIP(lookupCtx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve outbound host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, &OutboundTargetError{
			Host:   host,
			Reason: "host resolved without addresses",
		}
	}

	allowed := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if ip.Is4In6() {
			ip = ip.Unmap()
		}
		if reason, blocked := blockedOutboundAddrReason(ip); blocked {
			return nil, &OutboundTargetError{
				Host:   host,
				IP:     ip.String(),
				Reason: reason,
			}
		}
		allowed = append(allowed, ip)
	}

	return allowed, nil
}

func validateOutboundHostLiterals(host string) error {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return &OutboundTargetError{Reason: "url host is empty"}
	}

	if isLocalhostHostname(trimmed) {
		return &OutboundTargetError{
			Host:   trimmed,
			Reason: "localhost targets are not allowed",
		}
	}

	if parsed := net.ParseIP(trimmed); parsed != nil {
		addr, ok := netip.AddrFromSlice(parsed)
		if !ok {
			return &OutboundTargetError{
				Host:   trimmed,
				Reason: "invalid_ip",
			}
		}
		if addr.Is4In6() {
			addr = addr.Unmap()
		}
		if reason, blocked := blockedOutboundAddrReason(addr); blocked {
			return &OutboundTargetError{
				Host:   trimmed,
				IP:     addr.String(),
				Reason: reason,
			}
		}
	}

	return nil
}

func isLocalhostHostname(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimSuffix(host, ".")
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

func blockedOutboundAddrReason(addr netip.Addr) (string, bool) {
	for _, entry := range disallowedOutboundPrefixes {
		if entry.prefix.Contains(addr) {
			return entry.reason, true
		}
	}

	return "", false
}
