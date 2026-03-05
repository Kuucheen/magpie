package support

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func IsValidURL(raw string) bool {
	_, err := parseOutboundHTTPURL(raw)
	return err == nil
}

func GetProxiesOfHTML(rawHTML string) []string {
	decoded := html.UnescapeString(rawHTML)

	set := make(map[string]struct{})

	// 1. Literal "ip:port" or "ip &colon; port" occurrences.
	ipPortRe := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\s*(?::|&colon;)\s*\d{1,5}\b`)
	for _, m := range ipPortRe.FindAllString(decoded, -1) {
		candidate := strings.ReplaceAll(strings.ReplaceAll(m, "&colon;", ":"), " ", "")
		set[candidate] = struct{}{}
	}

	// 2. Table‑aware extraction where IP and port are split across cells.
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(decoded))
	if err == nil {
		ipOnlyRe := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
		portOnlyRe := regexp.MustCompile(`\b\d{1,5}\b`)

		doc.Find("tr").Each(func(_ int, row *goquery.Selection) {
			var ip string
			row.Find("td,th").Each(func(_ int, cell *goquery.Selection) {
				txt := strings.TrimSpace(cell.Text())
				if ip == "" {
					if ipOnlyRe.MatchString(txt) {
						ip = ipOnlyRe.FindString(txt)
					}
					return
				}
				if portOnlyRe.MatchString(txt) {
					port := portOnlyRe.FindString(txt)
					proxy := fmt.Sprintf("%s:%s", ip, port)
					set[proxy] = struct{}{}
				}
			})
		})

		// 3. Consecutive‑token scan of the plain text.
		plain := strings.TrimSpace(doc.Text())
		words := strings.Fields(plain)
		for i := 0; i+1 < len(words); i++ {
			w1, w2 := words[i], words[i+1]
			if ipOnlyRe.MatchString(w1) && portOnlyRe.MatchString(w2) {
				proxy := fmt.Sprintf("%s:%s", w1, w2)
				set[proxy] = struct{}{}
			}
		}
	}

	// 4. Convert set → sorted slice.
	proxies := make([]string, 0, len(set))
	for p := range set {
		proxies = append(proxies, p)
	}
	sort.Strings(proxies)
	return proxies
}

func ParseTextToSources(text string) []string {
	lines := strings.Split(text, "\n")
	var sources []string

	for _, line := range lines {
		// Trim whitespace and skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Only add valid URLs
		if IsValidURL(line) {
			sources = append(sources, line)
		}
	}

	return sources
}
