package openai

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseCodexCLIVersionOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{name: "current local cli shape", output: "codex-cli 0.140.0\n", want: "0.140.0"},
		{name: "plain codex shape", output: "codex 0.141.2\n", want: "0.141.2"},
		{name: "prefixed sentence", output: "Codex CLI version 0.142.3", want: "0.142.3"},
		{name: "no semver", output: "codex-cli dev\n", wantErr: true},
		{name: "empty", output: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCodexCLIVersionOutput(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseCodexCLIVersionOutput(%q) returned nil error", tt.output)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCodexCLIVersionOutput(%q) returned error: %v", tt.output, err)
			}
			if got != tt.want {
				t.Fatalf("parseCodexCLIVersionOutput(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}

func TestCodexCLIVersionFetcherUsesLocalCommandOutput(t *testing.T) {
	fetcher := NewCodexCLIVersionFetcher(time.Hour)
	var gotName string
	var gotArgs []string
	fetcher.commandRunner = func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("codex-cli 0.141.0\n"), nil
	}

	got, err := fetcher.FetchLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestVersion returned error: %v", err)
	}
	if got != "0.141.0" {
		t.Fatalf("FetchLatestVersion = %q, want %q", got, "0.141.0")
	}
	if gotName != "codex" {
		t.Fatalf("command name = %q, want codex", gotName)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--version"}) {
		t.Fatalf("command args = %#v, want [--version]", gotArgs)
	}
	if fetcher.GetCurrentVersion() != "0.141.0" {
		t.Fatalf("current version = %q, want 0.141.0", fetcher.GetCurrentVersion())
	}
	if ua := fetcher.GetCurrentUserAgent(); !strings.Contains(ua, "Codex Desktop/0.141.0 ") || !strings.Contains(ua, "(codex_exec; 0.141.0)") {
		t.Fatalf("current user agent = %q, want version in both desktop and exec positions", ua)
	}
	if ua := fetcher.GetDefaultUserAgent(); !strings.Contains(ua, "codex_cli_rs/0.141.0 ") {
		t.Fatalf("default user agent = %q, want codex_cli_rs/0.141.0", ua)
	}
}

func TestCodexCLIVersionFetcherKeepsFallbackWhenLocalCommandFails(t *testing.T) {
	fetcher := NewCodexCLIVersionFetcher(time.Hour)
	fetcher.commandRunner = func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("not installed")
	}

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
