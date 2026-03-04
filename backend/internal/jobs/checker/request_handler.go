package checker

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"magpie/internal/config"
	"magpie/internal/domain"
	"magpie/internal/support"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type cachedRegex struct {
	re        *regexp.Regexp
	ok        bool
	expiresAt time.Time
}

const (
	regexCacheTTL                       = 5 * time.Minute
	regexCacheCleanupInterval           = 1 * time.Minute
	envCheckerMaxResponseBody           = "CHECKER_MAX_RESPONSE_BODY_BYTES"
	envCheckerDefaultRequestTimeoutMS   = "CHECKER_DEFAULT_REQUEST_TIMEOUT_MS"
	defaultCheckerMaxBodySize           = 1 << 20 // 1 MiB
	defaultCheckerDefaultRequestTimeout = 10 * time.Second
)

var (
	regexCacheMu          sync.Mutex
	regexCacheByKey       = make(map[string]cachedRegex)
	nextRegexCacheCleanup = time.Now().Add(regexCacheCleanupInterval)
)

// ProxyCheckRequest makes a request to the provided siteUrl with the provided proxy
func ProxyCheckRequest(proxyToCheck domain.Proxy, judge *domain.Judge, protocol string, transportProtocol string, timeout uint16) (string, error) {
	if judge == nil {
		return "Invalid judge", fmt.Errorf("judge is required")
	}
	if config.IsWebsiteBlocked(judge.FullString) {
		return "Blocked judge website", fmt.Errorf("judge website is blocked: %s", judge.FullString)
	}

	client, err := getCheckerHTTPClient(proxyToCheck, judge, protocol, transportProtocol)
	if err != nil {
		return "Failed to create transport", err
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", judge.FullString, nil)
	if err != nil {
		return "Error creating request", err
	}
	if support.IsHTTP3Transport(transportProtocol) && proxyToCheck.HasAuth() && (protocol == "http" || protocol == "https") {
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", proxyToCheck.Username, proxyToCheck.Password)))
		req.Header.Set("Proxy-Authorization", "Basic "+auth)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "Request failed", err
	}
	defer resp.Body.Close()

	bodyLimit := checkerMaxResponseBodyBytes()
	body, err := support.ReadAllWithLimit(resp.Body, bodyLimit)
	if err != nil {
		if errors.Is(err, support.ErrResponseBodyTooLarge) {
			return "Error reading body", fmt.Errorf("judge response body exceeded %d bytes", bodyLimit)
		}
		return "Error reading body", err
	}

	html := string(body)

	return html, nil
}

func CheckForValidResponse(html string, regex string) bool {
	if strings.EqualFold(regex, "default") {
		html = strings.ReplaceAll(html, "_", "-")
		html = strings.ToUpper(html)

		for _, header := range config.GetConfig().Checker.StandardHeader {
			if !strings.Contains(html, header) {

				return false
			}
		}

		return true
	}

	re, ok := getCachedRegex(regex)
	if !ok {
		return false
	}

	return re.MatchString(html)
}

func getCachedRegex(pattern string) (*regexp.Regexp, bool) {
	now := time.Now()

	regexCacheMu.Lock()
	if now.After(nextRegexCacheCleanup) {
		evictExpiredRegexEntriesLocked(now)
		nextRegexCacheCleanup = now.Add(regexCacheCleanupInterval)
	}

	if entry, ok := regexCacheByKey[pattern]; ok {
		if !entry.expiresAt.After(now) {
			delete(regexCacheByKey, pattern)
			regexCacheMu.Unlock()
			return compileAndStoreRegex(pattern)
		}

		valid := entry.ok && entry.re != nil
		re := entry.re
		regexCacheMu.Unlock()
		if !valid {
			return nil, false
		}
		return re, true
	}
	regexCacheMu.Unlock()

	return compileAndStoreRegex(pattern)
}

func compileAndStoreRegex(pattern string) (*regexp.Regexp, bool) {
	re, err := regexp.Compile(pattern)
	compiledOK := err == nil && re != nil

	now := time.Now()
	regexCacheMu.Lock()
	regexCacheByKey[pattern] = cachedRegex{
		re:        re,
		ok:        compiledOK,
		expiresAt: now.Add(regexCacheTTL),
	}
	regexCacheMu.Unlock()

	if !compiledOK {
		return nil, false
	}
	return re, true
}

func evictExpiredRegexEntriesLocked(now time.Time) {
	for pattern, entry := range regexCacheByKey {
		if !entry.expiresAt.After(now) {
			delete(regexCacheByKey, pattern)
		}
	}
}

func DefaultRequest(siteName string) (string, error) {
	return DefaultRequestWithContext(context.Background(), siteName)
}

func DefaultRequestWithContext(ctx context.Context, siteName string) (string, error) {
	if config.IsWebsiteBlocked(siteName) {
		return "", fmt.Errorf("target website is blocked: %s", siteName)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, siteName, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: checkerDefaultRequestTimeout()}
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	bodyLimit := checkerMaxResponseBodyBytes()
	body, err := support.ReadAllWithLimit(response.Body, bodyLimit)
	if err != nil {
		if errors.Is(err, support.ErrResponseBodyTooLarge) {
			return "", fmt.Errorf("response body exceeded %d bytes", bodyLimit)
		}
		return "", err
	}

	return string(body), nil
}

func checkerMaxResponseBodyBytes() int64 {
	limit := support.GetEnvInt(envCheckerMaxResponseBody, defaultCheckerMaxBodySize)
	if limit <= 0 {
		limit = defaultCheckerMaxBodySize
	}
	return int64(limit)
}

func checkerDefaultRequestTimeout() time.Duration {
	ms := support.GetEnvInt(envCheckerDefaultRequestTimeoutMS, int(defaultCheckerDefaultRequestTimeout/time.Millisecond))
	if ms <= 0 {
		return defaultCheckerDefaultRequestTimeout
	}
	return time.Duration(ms) * time.Millisecond
}
