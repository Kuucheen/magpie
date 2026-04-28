package support

import (
	"fmt"
	"net/url"
	"strings"

	"magpie/internal/api/dto"
)

func FormatScrapeSource(source dto.ScrapeSiteInfo, outputFormat string) string {
	if strings.TrimSpace(outputFormat) == "" {
		outputFormat = "url"
	}

	protocol, sourceURL := splitScrapeSourceURL(source.Url)
	replacements := []string{
		"alive_proxy_count", fmt.Sprintf("%d", source.AliveCount),
		"proxy_count", fmt.Sprintf("%d", source.ProxyCount),
		"protocol", protocol,
		"url", sourceURL,
	}

	return strings.NewReplacer(replacements...).Replace(outputFormat)
}

func splitScrapeSourceURL(rawURL string) (string, string) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", ""
	}

	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" {
		sourceURL := parsed.Host
		if parsed.Opaque != "" && sourceURL == "" {
			sourceURL = parsed.Opaque
		}
		sourceURL += parsed.EscapedPath()
		if parsed.RawQuery != "" {
			sourceURL += "?" + parsed.RawQuery
		}
		if parsed.Fragment != "" {
			sourceURL += "#" + parsed.Fragment
		}
		return strings.ToLower(parsed.Scheme), sourceURL
	}

	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSuffix(prefix, "://"), trimmed[len(prefix):]
		}
	}

	return "", trimmed
}
