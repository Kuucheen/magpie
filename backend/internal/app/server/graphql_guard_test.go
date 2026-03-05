package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateGraphQLQueryComplexity_DepthLimit(t *testing.T) {
	t.Setenv(envGraphQLMaxDepth, "2")

	err := validateGraphQLQueryComplexity("query { a { b { c } } }", "")
	if err == nil || !strings.Contains(err.Error(), "depth exceeds") {
		t.Fatalf("expected depth error, got %v", err)
	}
}

func TestValidateGraphQLQueryComplexity_FieldLimit(t *testing.T) {
	t.Setenv(envGraphQLMaxFields, "2")

	err := validateGraphQLQueryComplexity("query { a b c }", "")
	if err == nil || !strings.Contains(err.Error(), "field count exceeds") {
		t.Fatalf("expected field count error, got %v", err)
	}
}

func TestValidateGraphQLQueryComplexity_BlocksIntrospectionByDefault(t *testing.T) {
	err := validateGraphQLQueryComplexity("query { __schema { queryType { name } } }", "")
	if err == nil || !strings.Contains(err.Error(), "introspection is disabled") {
		t.Fatalf("expected introspection error, got %v", err)
	}
}

func TestValidateGraphQLQueryComplexity_AllowsIntrospectionWhenEnabled(t *testing.T) {
	t.Setenv(envGraphQLAllowIntrospection, "true")

	if err := validateGraphQLQueryComplexity("query { __typename }", ""); err != nil {
		t.Fatalf("expected introspection query to pass when enabled, got %v", err)
	}
}

func TestParseGraphQLRequestOptions_RestoresPOSTBody(t *testing.T) {
	body := []byte(`{"query":"query { viewer { id } }"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	opts, err := parseGraphQLRequestOptions(req)
	if err != nil {
		t.Fatalf("parseGraphQLRequestOptions error: %v", err)
	}
	if !strings.Contains(opts.Query, "viewer") {
		t.Fatalf("query = %q, expected viewer selection", opts.Query)
	}

	restored, readErr := io.ReadAll(req.Body)
	if readErr != nil {
		t.Fatalf("read restored body: %v", readErr)
	}
	if string(restored) != string(body) {
		t.Fatalf("request body was not restored, got %q want %q", string(restored), string(body))
	}
}

func TestWithGraphQLGuard_RejectsOversizedQuery(t *testing.T) {
	t.Setenv(envGraphQLMaxQueryBytes, "8")

	handler := withGraphQLGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/graphql?query=query%20%7B%20viewer%20%7D", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}
