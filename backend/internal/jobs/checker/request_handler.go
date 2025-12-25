package checker

import (
	"encoding/base64"
	"fmt"
	"io"
	"magpie/internal/config"
	"magpie/internal/domain"
	"magpie/internal/support"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ProxyCheckRequest makes a request to the provided siteUrl with the provided proxy
func ProxyCheckRequest(proxyToCheck domain.Proxy, judge *domain.Judge, protocol string, transportProtocol string, timeout uint16) (string, error) {
	if judge != nil && config.IsWebsiteBlocked(judge.FullString) {
		return "Blocked judge website", fmt.Errorf("judge website is blocked: %s", judge.FullString)
	}

	transport, closeFunc, err := support.CreateTransport(proxyToCheck, judge, protocol, transportProtocol)
	if err != nil {
		return "Failed to create transport", err
	}
	if closeFunc != nil {
		defer closeFunc() // Release resources immediately
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Millisecond,
	}

	req, err := http.NewRequest("GET", judge.FullString, nil)
	if err != nil {
		return "Error creating request", err
	}
	req.Header.Set("Connection", "close")
	if support.IsHTTP3Transport(transportProtocol) && proxyToCheck.HasAuth() && (protocol == "http" || protocol == "https") {
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", proxyToCheck.Username, proxyToCheck.Password)))
		req.Header.Set("Proxy-Authorization", "Basic "+auth)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "Request failed", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
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

	re, err := regexp.Compile(regex)
	if err != nil {
		return false
	}

	return re.MatchString(html)
}

func DefaultRequest(siteName string) (string, error) {
	if config.IsWebsiteBlocked(siteName) {
		return "", fmt.Errorf("target website is blocked: %s", siteName)
	}

	response, err := http.Get(siteName)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
