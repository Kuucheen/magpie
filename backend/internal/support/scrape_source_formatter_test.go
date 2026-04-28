package support

import (
	"testing"

	"magpie/internal/api/dto"
)

func TestFormatScrapeSourceIncludesProtocolUrlAndCounts(t *testing.T) {
	source := dto.ScrapeSiteInfo{
		Url:        "https://example.com/proxies.txt?fresh=true",
		ProxyCount: 42,
		AliveCount: 17,
	}

	got := FormatScrapeSource(source, "protocol;url;proxy_count;alive_proxy_count")
	want := "https;example.com/proxies.txt?fresh=true;42;17"

	if got != want {
		t.Fatalf("FormatScrapeSource() = %q, want %q", got, want)
	}
}
