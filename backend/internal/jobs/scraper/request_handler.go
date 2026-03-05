package scraper

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"magpie/internal/config"
	"magpie/internal/support"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	scraperUserAgent               = "magpie-scraper/1.0"
	envScraperFallbackMaxBodyBytes = "SCRAPER_FALLBACK_MAX_RESPONSE_BODY_BYTES"
	envScraperCapturedMaxBodyBytes = "SCRAPER_CAPTURED_MAX_RESPONSE_BODY_BYTES"
	defaultScraperFallbackMaxBody  = 8 << 20 // 8 MiB
	defaultScraperCapturedMaxBody  = 8 << 20 // 8 MiB
)

/*
ScraperRequest fetches the HTML of url within the given timeout.

It borrows a *rod.Page from the global pagePool, does the navigation
and then defers the page‑recycling to recyclePage(), which decides
whether to return the page to the pool or close it (depending on
signals from managePagePool). This keeps the request code tiny while
all pool housekeeping lives in thread_handler.go.
*/
func ScraperRequest(url string, timeout time.Duration) (string, error) {
	if err := validateScrapeTarget(url, timeout); err != nil {
		return "", err
	}

	StartInfrastructure()

	if config.IsWebsiteBlocked(url) {
		return "", fmt.Errorf("scrape blocked by website blacklist: %s", url)
	}

	// 1) acquire a page with timeout
	var basePage *rod.Page
	select {
	case basePage = <-pagePool:
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for available page")
	}

	page := basePage.Timeout(timeout)

	// 2) ensure we recycle it back (or close+re-add on error)
	defer func() {
		page.CancelTimeout()
		recyclePage(basePage)
	}()

	// Deny disk downloads for this page; deprecated API but still honored.
	_ = proto.PageSetDownloadBehavior{
		Behavior: proto.PageSetDownloadBehaviorBehaviorDeny,
	}.Call(page)

	// Ensure network events are available so we can pull raw responses.
	_ = proto.NetworkEnable{}.Call(page)
	if err := (proto.FetchEnable{
		Patterns: []*proto.FetchRequestPattern{
			{URLPattern: "http://*", RequestStage: proto.FetchRequestStageRequest},
			{URLPattern: "https://*", RequestStage: proto.FetchRequestStageRequest},
		},
	}).Call(page); err != nil {
		return "", fmt.Errorf("enable browser request guard: %w", err)
	}
	defer func() {
		_ = proto.FetchDisable{}.Call(page)
	}()

	var (
		capturedBody         string
		capturedMime         string
		capturedDisposition  string
		captured             bool
		done                 = make(chan struct{})
		doneOnce             sync.Once
		responseCaptureError error
	)

	eventCtx, cancelEvents := context.WithCancel(context.Background())
	defer cancelEvents()

	var mainRequestID proto.NetworkRequestID
	capturedBodyLimit := scraperCapturedMaxResponseBodyBytes()

	waitResponse := page.Context(eventCtx).EachEvent(
		func(e *proto.FetchRequestPaused) {
			if e == nil || e.Request == nil {
				return
			}

			if err := validateScrapeRuntimeURL(e.Request.URL, timeout); err != nil {
				responseCaptureError = fmt.Errorf("unsafe browser request target: %w", err)
				_ = proto.FetchFailRequest{
					RequestID:   e.RequestID,
					ErrorReason: proto.NetworkErrorReasonAccessDenied,
				}.Call(page)
				_ = page.StopLoading()
				doneOnce.Do(func() { close(done) })
				return
			}

			if err := (proto.FetchContinueRequest{RequestID: e.RequestID}).Call(page); err != nil {
				responseCaptureError = fmt.Errorf("continue browser request: %w", err)
				doneOnce.Do(func() { close(done) })
			}
		},
		func(e *proto.NetworkRequestWillBeSent) {
			if e.FrameID != "" && e.FrameID != page.FrameID {
				return
			}

			if err := validateScrapeRuntimeURL(e.Request.URL, timeout); err != nil {
				responseCaptureError = fmt.Errorf("unsafe browser request target: %w", err)
				_ = page.StopLoading()
				doneOnce.Do(func() { close(done) })
				return
			}
			if e.Type == proto.NetworkResourceTypeDocument {
				mainRequestID = e.RequestID
				return
			}
			if mainRequestID == "" && (e.Request.URL == url || e.DocumentURL == url) {
				mainRequestID = e.RequestID
				return
			}
			if mainRequestID == "" && (e.Type == proto.NetworkResourceTypeOther || e.Type == proto.NetworkResourceTypeXHR || e.Type == proto.NetworkResourceTypeFetch) {
				mainRequestID = e.RequestID
			}
		},
		func(e *proto.NetworkResponseReceived) bool {
			if e.FrameID != "" && e.FrameID != page.FrameID {
				return false
			}
			if mainRequestID != "" && e.RequestID != mainRequestID {
				return false
			}
			if err := validateScrapeRemoteIP(e.Response.RemoteIPAddress); err != nil {
				responseCaptureError = fmt.Errorf("unsafe browser remote address: %w", err)
				_ = page.StopLoading()
				doneOnce.Do(func() { close(done) })
				return true
			}

			if contentLengthExceedsLimit(e.Response.Headers, capturedBodyLimit) {
				responseCaptureError = fmt.Errorf("captured response body exceeded %d bytes", capturedBodyLimit)
				doneOnce.Do(func() { close(done) })
				return true
			}

			body, err := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
			if err != nil {
				responseCaptureError = err
				doneOnce.Do(func() { close(done) })
				return true
			}

			if body.Base64Encoded {
				if int64(base64.StdEncoding.DecodedLen(len(body.Body))) > capturedBodyLimit {
					responseCaptureError = fmt.Errorf("captured response body exceeded %d bytes", capturedBodyLimit)
					doneOnce.Do(func() { close(done) })
					return true
				}
				raw, decodeErr := base64.StdEncoding.DecodeString(body.Body)
				if decodeErr != nil {
					responseCaptureError = decodeErr
					doneOnce.Do(func() { close(done) })
					return true
				}
				if int64(len(raw)) > capturedBodyLimit {
					responseCaptureError = fmt.Errorf("captured response body exceeded %d bytes", capturedBodyLimit)
					doneOnce.Do(func() { close(done) })
					return true
				}
				capturedBody = string(raw)
			} else {
				if int64(len(body.Body)) > capturedBodyLimit {
					responseCaptureError = fmt.Errorf("captured response body exceeded %d bytes", capturedBodyLimit)
					doneOnce.Do(func() { close(done) })
					return true
				}
				capturedBody = body.Body
			}

			captured = true
			capturedMime = e.Response.MIMEType
			capturedDisposition = headerValue(e.Response.Headers, "Content-Disposition")
			mainRequestID = e.RequestID

			doneOnce.Do(func() { close(done) })
			return true
		},
		func(e *proto.NetworkLoadingFinished) bool {
			if mainRequestID == "" || e.RequestID != mainRequestID {
				return false
			}
			doneOnce.Do(func() { close(done) })
			return true
		},
	)

	go func() {
		waitResponse()
		doneOnce.Do(func() { close(done) })
	}()

	waitWindow := time.Second
	if timeout > 0 && timeout < waitWindow {
		waitWindow = timeout
	}

	navErr := page.Navigate(url)

	select {
	case <-done:
	case <-time.After(waitWindow):
	}

	if responseCaptureError != nil {
		captured = false
	}

	if navErr != nil {
		if captured {
			return capturedBody, nil
		}
		if isNavigationAbortError(navErr) {
			if fallback, err := fetchDirect(url, timeout); err == nil {
				return fallback, nil
			} else {
				return "", fmt.Errorf("navigation aborted and fallback fetch failed: %w", err)
			}
		}
		return "", navErr
	}

	if err := page.WaitLoad(); err != nil {
		if captured {
			return capturedBody, nil
		}
		if isNavigationAbortError(err) {
			if fallback, fallbackErr := fetchDirect(url, timeout); fallbackErr == nil {
				return fallback, nil
			} else {
				return "", fmt.Errorf("navigation aborted and fallback fetch failed: %w", fallbackErr)
			}
		}
		return "", err
	}

	// 4) grab the HTML
	html, err := page.HTML()
	if err != nil {
		if captured {
			return capturedBody, nil
		}
		return "", err
	}
	if captured && shouldPreferCapturedBody(capturedMime, capturedDisposition, html) {
		return capturedBody, nil
	}
	return html, nil
}

func resetPage(page *rod.Page) error {
	// Clear cookies
	err := proto.NetworkClearBrowserCookies{}.Call(page)
	if err != nil {
		return fmt.Errorf("clear cookies: %w", err)
	}

	// Navigate to about:blank first
	if err := page.Navigate("about:blank"); err != nil {
		return fmt.Errorf("navigate blank: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait blank: %w", err)
	}

	_, _ = page.Eval(`() => {
        try {
            localStorage.clear();
            sessionStorage.clear();
        } catch (e) {
            // Silently ignore security errors
        }
        return true;
    }`)

	return nil
}

func headerValue(headers proto.NetworkHeaders, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func shouldPreferCapturedBody(mime, disposition, html string) bool {
	if disposition != "" && strings.Contains(strings.ToLower(disposition), "attachment") {
		return true
	}

	if mime != "" && !strings.Contains(strings.ToLower(mime), "html") {
		return true
	}

	if strings.TrimSpace(html) == "" {
		return true
	}

	return false
}

func isNavigationAbortError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	if msg == "" {
		return false
	}
	if strings.Contains(strings.ToLower(msg), "context deadline exceeded") {
		return true
	}
	abortSignatures := []string{
		"net::ERR_ABORTED",
		"NS_BINDING_ABORTED",
		"ERR_INTERNET_DISCONNECTED",
	}
	for _, sig := range abortSignatures {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}

func fetchDirect(url string, timeout time.Duration) (string, error) {
	limit := 30 * time.Second
	if timeout > 0 {
		limit = timeout
	}

	if config.IsWebsiteBlocked(url) {
		return "", fmt.Errorf("direct fetch blocked by website blacklist: %s", url)
	}

	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	validatedURL, err := support.ValidateOutboundHTTPURLContext(ctx, url)
	if err != nil {
		return "", fmt.Errorf("unsafe fallback target: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", scraperUserAgent)

	client := support.NewRestrictedOutboundHTTPClient(limit)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("fallback fetch status %d", resp.StatusCode)
	}

	bodyLimit := scraperFallbackMaxResponseBodyBytes()
	body, err := support.ReadAllWithLimit(resp.Body, bodyLimit)
	if err != nil {
		if errors.Is(err, support.ErrResponseBodyTooLarge) {
			return "", fmt.Errorf("fallback response body exceeded %d bytes", bodyLimit)
		}
		return "", err
	}

	return string(body), nil
}

func scraperFallbackMaxResponseBodyBytes() int64 {
	limit := support.GetEnvInt(envScraperFallbackMaxBodyBytes, defaultScraperFallbackMaxBody)
	if limit <= 0 {
		limit = defaultScraperFallbackMaxBody
	}
	return int64(limit)
}

func scraperCapturedMaxResponseBodyBytes() int64 {
	limit := support.GetEnvInt(envScraperCapturedMaxBodyBytes, defaultScraperCapturedMaxBody)
	if limit <= 0 {
		limit = defaultScraperCapturedMaxBody
	}
	return int64(limit)
}

func contentLengthExceedsLimit(headers proto.NetworkHeaders, limit int64) bool {
	if limit <= 0 {
		return true
	}
	raw := strings.TrimSpace(headerValue(headers, "Content-Length"))
	if raw == "" {
		return false
	}
	length, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || length < 0 {
		return false
	}
	return length > limit
}

func validateScrapeTarget(rawURL string, timeout time.Duration) error {
	limit := 5 * time.Second
	if timeout > 0 && timeout < limit {
		limit = timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	if _, err := support.ValidateOutboundHTTPURLContext(ctx, rawURL); err != nil {
		return fmt.Errorf("unsafe scrape target: %w", err)
	}

	return nil
}

func validateScrapeRuntimeURL(rawURL string, timeout time.Duration) error {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return nil
	}

	limit := 2 * time.Second
	if timeout > 0 && timeout < limit {
		limit = timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	if _, err := support.ValidateOutboundHTTPURLContext(ctx, rawURL); err != nil {
		return err
	}

	return nil
}

func validateScrapeRemoteIP(rawIP string) error {
	if strings.TrimSpace(rawIP) == "" {
		return nil
	}
	return support.ValidateOutboundIPLiteral(rawIP)
}
