package support

import (
	"fmt"
	"magpie/internal/config"
	"magpie/internal/domain"
	"net"
	"regexp"
	"strconv"
	"strings"
)

func ParseTextToProxies(text string) []domain.Proxy {
	proxies, _ := parseTextToProxiesWithStats(text, true)
	return proxies
}

type ProxyParseStats struct {
	SubmittedCount     int
	ParsedCount        int
	InvalidFormatCount int
	InvalidIPCount     int
	InvalidIPv4Count   int
	InvalidPortCount   int
}

func ParseTextToProxiesWithStats(text string) ([]domain.Proxy, ProxyParseStats) {
	return parseTextToProxiesWithStats(text, true)
}

// ParseTextToProxiesStrictAuth returns proxies where credentials are only read
// from user:pass@host:port formatted entries. Useful when plain colon-delimited
// proxy lists would otherwise be misinterpreted as auth.
func ParseTextToProxiesStrictAuth(text string) []domain.Proxy {
	proxies, _ := parseTextToProxiesWithStats(text, false)
	return proxies
}

func parseTextToProxiesWithStats(text string, allowColonAuth bool) ([]domain.Proxy, ProxyParseStats) {
	if allowColonAuth {
		text = clearProxyString(text)
	} else {
		text = clearProxyStringPreserveAuth(text)
	}

	lines := strings.Split(text, "\n")
	proxies := make([]domain.Proxy, 0, len(lines))
	stats := ProxyParseStats{}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		stats.SubmittedCount++

		var (
			username string
			password string
			hostPart string = line
		)

		if at := strings.LastIndex(line, "@"); at != -1 {
			credPart := line[:at]
			hostPart = line[at+1:]

			credSplit := strings.SplitN(credPart, ":", 2)
			if len(credSplit) == 2 {
				username = strings.TrimSpace(credSplit[0])
				password = strings.TrimSpace(credSplit[1])
			}
		}

		hostSplit := strings.Split(hostPart, ":")
		if len(hostSplit) < 2 {
			stats.InvalidFormatCount++
			continue
		}

		ip := normalizeIPv4(strings.TrimSpace(hostSplit[0]))
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			stats.InvalidIPCount++
			continue
		}
		if parsedIP.To4() == nil {
			stats.InvalidIPv4Count++
			continue
		}

		portStr := strings.TrimSpace(hostSplit[1])
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			stats.InvalidPortCount++
			continue
		}

		// Handle formats like ip:port:user:pass when no @ credentials were provided.
		if allowColonAuth && username == "" && password == "" && len(hostSplit) >= 4 {
			candidateUser := strings.TrimSpace(hostSplit[2])
			candidatePass := strings.TrimSpace(strings.Join(hostSplit[3:], ":"))

			// Skip obviously wrong mappings where creds repeat host/port.
			if !(candidateUser == ip && candidatePass == portStr) {
				username = candidateUser
				password = candidatePass
			}
		}

		proxy := domain.Proxy{
			Port:     uint16(port),
			Username: username,
			Password: password,
		}

		if err := proxy.SetIP(ip); err != nil {
			stats.InvalidIPv4Count++
			continue
		}

		proxies = append(proxies, proxy)
		stats.ParsedCount++
	}

	return proxies, stats
}

func clearProxyString(proxies string) string {
	return cleanProxyString(proxies, true)
}

func clearProxyStringPreserveAuth(proxies string) string {
	return cleanProxyString(proxies, false)
}

func cleanProxyString(proxies string, replaceAuthDelimiter bool) string {
	if replaceAuthDelimiter {
		proxies = strings.ReplaceAll(proxies, "@", ";")
	}
	proxies = strings.ReplaceAll(proxies, "\r", "")

	proxies = strings.ReplaceAll(proxies, "..", ".0.")
	proxies = strings.ReplaceAll(proxies, ".:", ".0:")

	return proxies
}

func normalizeIPv4(value string) string {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return value
	}

	normalized := make([]string, 0, 4)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return value
		}
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return value
		}
		normalized = append(normalized, strconv.Itoa(num))
	}

	return strings.Join(normalized, ".")
}

// FindIP identifies the first IP address (IPv4 or IPv6) in a given string.
func FindIP(input string) string {
	// Regular expression for matching IPv4 and IPv6 addresses
	ipRegex := `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b|` + // IPv4
		`\b(?:[A-Fa-f0-9]{1,4}:){7}[A-Fa-f0-9]{1,4}\b` // IPv6

	return regexp.MustCompile(ipRegex).FindString(input)
}

func GetProxyLevel(html string) int {
	//When the headers contain UserIp proxy is transparent
	if strings.Contains(html, config.GetCurrentIp()) {
		return 3
	}

	//When containing one of these headers the proxy is anonymous
	cfg := config.GetConfig()
	for _, header := range cfg.Checker.ProxyHeader {
		if strings.Contains(html, header) {
			return 2
		}
	}

	//Proxy is elite
	return 1
}

// FormatProxies formats the list of proxies according to the specified output format
func FormatProxies(proxies []domain.Proxy, outputFormat string) string {
	var result strings.Builder

	for _, proxy := range proxies {
		protocolName := ""
		aliveValue := "false"
		timeValue := "0"

		if latestStat := latestStatistic(proxy.Statistics); latestStat != nil {
			protocolName = getProtocolName(latestStat)
			aliveValue = strconv.FormatBool(latestStat.Alive)
			timeValue = strconv.Itoa(int(latestStat.ResponseTime))
		}

		reputationLabel, reputationScore := resolveReputationForExport(proxy.Reputations, protocolName)

		replacements := []string{
			"protocol", protocolName,
			"ip", proxy.GetIp(),
			"port", fmt.Sprintf("%d", proxy.Port),
			"username", proxy.Username,
			"password", proxy.Password,
			"country", proxy.Country,
			"alive", aliveValue,
			"type", proxy.EstimatedType,
			"time", timeValue,
			"reputation_score", reputationScore,
			"reputation_label", reputationLabel,
			"reputation", reputationLabel,
		}

		line := strings.NewReplacer(replacements...).Replace(outputFormat)

		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}

// Helper function to get protocol name from statistics
func getProtocolName(stat *domain.ProxyStatistic) string {
	if stat == nil || stat.Protocol.Name == "" {
		return ""
	}
	return stat.Protocol.Name
}

func latestStatistic(stats []domain.ProxyStatistic) *domain.ProxyStatistic {
	if len(stats) == 0 {
		return nil
	}

	latest := &stats[0]
	for i := 1; i < len(stats); i++ {
		candidate := &stats[i]
		if candidate.CreatedAt.After(latest.CreatedAt) {
			latest = candidate
			continue
		}
		if candidate.CreatedAt.Equal(latest.CreatedAt) && candidate.ID > latest.ID {
			latest = candidate
		}
	}
	return latest
}

func formatReputationScore(score float32) string {
	formatted := fmt.Sprintf("%.2f", score)
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	if formatted == "-0" {
		return "0"
	}
	return formatted
}

func resolveReputationForExport(reputations []domain.ProxyReputation, protocolName string) (string, string) {
	targetKinds := make([]string, 0, 2)
	if trimmed := strings.ToLower(strings.TrimSpace(protocolName)); trimmed != "" {
		targetKinds = append(targetKinds, trimmed)
	}
	targetKinds = append(targetKinds, domain.ProxyReputationKindOverall)

	for _, kind := range targetKinds {
		if rep, ok := findReputationByKind(reputations, kind); ok {
			label := strings.TrimSpace(rep.Label)
			if label == "" {
				label = "unknown"
			}
			return label, formatReputationScore(rep.Score)
		}
	}

	return "", ""
}

func findReputationByKind(reputations []domain.ProxyReputation, kind string) (domain.ProxyReputation, bool) {
	lowerKind := strings.ToLower(strings.TrimSpace(kind))
	for _, rep := range reputations {
		if strings.ToLower(rep.Kind) == lowerKind {
			return rep, true
		}
	}
	return domain.ProxyReputation{}, false
}
