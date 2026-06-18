package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCodexCLIVersionFetcherAcceptsNpmLatestEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/@openai/codex/latest" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.142.0"}`))
	}))
	defer server.Close()

	fetcher := NewCodexCLIVersionFetcher(time.Hour)
	fetcher.registryURL = server.URL + "/@openai/codex/latest"

	got, err := fetcher.FetchLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestVersion returned error: %v", err)
	}
	if got != "0.142.0" {
		t.Fatalf("FetchLatestVersion = %q, want %q", got, "0.142.0")
	}
	if fetcher.GetCurrentVersion() != "0.142.0" {
		t.Fatalf("current version = %q, want 0.142.0", fetcher.GetCurrentVersion())
	}
	if ua := fetcher.GetCurrentUserAgent(); !strings.Contains(ua, "Codex Desktop/0.142.0 ") || !strings.Contains(ua, "(codex_exec; 0.142.0)") {
		t.Fatalf("current user agent = %q, want version in both desktop and exec positions", ua)
	}
	if ua := fetcher.GetDefaultUserAgent(); !strings.Contains(ua, "codex_cli_rs/0.142.0 ") {
		t.Fatalf("default user agent = %q, want codex_cli_rs/0.142.0", ua)
	}
}

func TestCodexCLIVersionFetcherFallsBackToDistTagsLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"dist-tags":{"latest":"0.143.1"}}`))
	}))
	defer server.Close()

	fetcher := NewCodexCLIVersionFetcher(time.Hour)
	fetcher.registryURL = server.URL + "/@openai/codex/latest"

	got, err := fetcher.FetchLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestVersion returned error: %v", err)
	}
	if got != "0.143.1" {
		t.Fatalf("FetchLatestVersion = %q, want %q", got, "0.143.1")
	}
}

func TestCodexCLIVersionFetcherKeepsFallbackWhenRegistryFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewCodexCLIVersionFetcher(time.Hour)
	fetcher.registryURL = server.URL + "/@openai/codex/latest"

	got, err := fetcher.FetchLatestVersion(context.Background())
	if err == nil {
		t.Fatalf("FetchLatestVersion returned nil error")
	}
	if got != CodexCLIDefaultVersion {
		t.Fatalf("FetchLatestVersion fallback = %q, want %q", got, CodexCLIDefaultVersion)
	}
	if fetcher.GetCurrentVersion() != CodexCLIDefaultVersion {
		t.Fatalf("current version = %q, want fallback %q", fetcher.GetCurrentVersion(), CodexCLIDefaultVersion)
	}
}
