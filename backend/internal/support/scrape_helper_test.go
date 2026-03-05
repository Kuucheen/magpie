package support

import (
	"reflect"
	"testing"
)

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		valid bool
	}{
		{"http", "http://example.com", true},
		{"https", "https://example.com/path", true},
		{"missing scheme", "example.com", false},
		{"unsupported scheme", "ftp://example.com", false},
		{"userinfo not allowed", "https://user:pass@example.com", false},
		{"invalid", "://missing-scheme", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidURL(tt.url); got != tt.valid {
				t.Fatalf("IsValidURL(%q) = %t, want %t", tt.url, got, tt.valid)
			}
		})
	}
}

func TestParseTextToSources(t *testing.T) {
	input := "https://example.com\nnot-a-url\n   \nhttp://valid.org/path\n"
	expected := []string{"https://example.com", "http://valid.org/path"}

	if got := ParseTextToSources(input); !reflect.DeepEqual(got, expected) {
		t.Fatalf("ParseTextToSources returned %v, want %v", got, expected)
	}
}

func TestGetProxiesOfHTML(t *testing.T) {
	html := `
		<table>
			<tr><td>192.0.2.10</td><td>8080</td></tr>
		</table>
		<p>Another proxy: 203.0.113.5 &colon; 3128</p>
		<p>Inline 198.51.100.1:8000 entry</p>
	`

	expected := []string{"192.0.2.10:8080", "198.51.100.1:8000", "203.0.113.5:3128"}
	if got := GetProxiesOfHTML(html); !reflect.DeepEqual(got, expected) {
		t.Fatalf("GetProxiesOfHTML returned %v, want %v", got, expected)
	}
}
