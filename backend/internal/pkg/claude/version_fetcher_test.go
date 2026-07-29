package claude

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCurrentClaudeCLIVersionHeadersStayAligned(t *testing.T) {
	if CLICurrentVersion != "2.1.220" {
		t.Fatalf("CLICurrentVersion = %q, want 2.1.220", CLICurrentVersion)
	}
	if got := DefaultHeaders["User-Agent"]; got != "claude-cli/2.1.220 (external, cli)" {
		t.Fatalf("User-Agent = %q, want current Claude CLI version", got)
	}
}

func TestVersionFetcherAcceptsNpmLatestEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/@anthropic-ai/claude-code/latest" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"2.2.123"}`))
	}))
	defer server.Close()

	fetcher := NewVersionFetcher(time.Hour)
	fetcher.registryURL = server.URL + "/@anthropic-ai/claude-code/latest"

	got, err := fetcher.FetchLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestVersion returned error: %v", err)
	}
	if got != "2.2.123" {
		t.Fatalf("FetchLatestVersion = %q, want %q", got, "2.2.123")
	}
	if fetcher.GetCurrentVersion() != "2.2.123" {
		t.Fatalf("current version = %q, want 2.2.123", fetcher.GetCurrentVersion())
	}
}

func TestHeadersForCLIVersionUsesDynamicUserAgent(t *testing.T) {
	headers := HeadersForCLIVersion("2.2.123")
	if got := headers["User-Agent"]; got != "claude-cli/2.2.123 (external, cli)" {
		t.Fatalf("User-Agent = %q, want dynamic version", got)
	}
	if headers["X-App"] != "cli" {
		t.Fatalf("X-App = %q, want cli", headers["X-App"])
	}

	headers["X-App"] = "changed"
	if DefaultHeaders["X-App"] != "cli" {
		t.Fatalf("HeadersForCLIVersion must return a copy, DefaultHeaders X-App = %q", DefaultHeaders["X-App"])
	}
}

func TestDefaultHeadersForCurrentVersionUsesFetcherCache(t *testing.T) {
	fetcher := NewVersionFetcher(time.Hour)
	fetcher.currentVersion = "2.2.124"

	headers := fetcher.DefaultHeaders()
	if got := headers["User-Agent"]; !strings.Contains(got, "2.2.124") {
		t.Fatalf("User-Agent = %q, want cached version", got)
	}
}
