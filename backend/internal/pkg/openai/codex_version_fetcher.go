package openai

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// CodexCLIDefaultVersion is the fallback version used when local Codex CLI
	// is unavailable in the runtime environment.
	CodexCLIDefaultVersion = "0.138.0"

	defaultCodexCLIVersionSyncInterval = 30 * time.Minute
	defaultCodexCLIVersionCommand      = "codex"
	defaultCodexCLIVersionTimeout      = 5 * time.Second
)

var codexCLIVersionPattern = regexp.MustCompile(`\b\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?\b`)

type codexCLICommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// CodexCLIVersionFetcher reads the installed local Codex CLI version.
type CodexCLIVersionFetcher struct {
	mu             sync.RWMutex
	currentVersion string
	lastFetchTime  time.Time
	fetchInterval  time.Duration
	commandName    string
	commandRunner  codexCLICommandRunner
}

var (
	globalCodexCLIVersionFetcher     *CodexCLIVersionFetcher
	globalCodexCLIVersionFetcherOnce sync.Once
)

// NewCodexCLIVersionFetcher creates a Codex CLI version fetcher.
func NewCodexCLIVersionFetcher(fetchInterval time.Duration) *CodexCLIVersionFetcher {
	if fetchInterval <= 0 {
		fetchInterval = defaultCodexCLIVersionSyncInterval
	}
	return &CodexCLIVersionFetcher{
		currentVersion: CodexCLIDefaultVersion,
		fetchInterval:  fetchInterval,
		commandName:    defaultCodexCLIVersionCommand,
		commandRunner:  runCodexCLIVersionCommand,
	}
}

// GetGlobalCodexCLIVersionFetcher returns the process-wide Codex CLI version fetcher.
func GetGlobalCodexCLIVersionFetcher() *CodexCLIVersionFetcher {
	globalCodexCLIVersionFetcherOnce.Do(func() {
		globalCodexCLIVersionFetcher = NewCodexCLIVersionFetcher(defaultCodexCLIVersionSyncInterval)
	})
	return globalCodexCLIVersionFetcher
}

// GetCurrentVersion returns the cached Codex CLI version.
func (f *CodexCLIVersionFetcher) GetCurrentVersion() string {
	if f == nil {
		return CodexCLIDefaultVersion
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.currentVersion
}

// FetchLatestVersion reads the current local Codex CLI version.
func (f *CodexCLIVersionFetcher) FetchLatestVersion(ctx context.Context) (string, error) {
	if f == nil {
		return CodexCLIDefaultVersion, fmt.Errorf("nil Codex CLI version fetcher")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if time.Since(f.lastFetchTime) < f.fetchInterval {
		return f.currentVersion, nil
	}

	commandCtx, cancel := context.WithTimeout(ctx, defaultCodexCLIVersionTimeout)
	defer cancel()

	output, err := f.commandRunner(commandCtx, f.commandName, "--version")
	version, parseErr := parseCodexCLIVersionOutput(string(output))
	if parseErr != nil {
		if err != nil {
			return f.currentVersion, fmt.Errorf("failed to run codex --version: %w", err)
		}
		return f.currentVersion, parseErr
	}

	f.currentVersion = version
	f.lastFetchTime = time.Now()
	return version, nil
}

// StartBackgroundSync starts immediate and periodic local Codex CLI version sync.
func (f *CodexCLIVersionFetcher) StartBackgroundSync(ctx context.Context) func() {
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(f.fetchInterval)
		defer ticker.Stop()

		_, _ = f.FetchLatestVersion(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				_, _ = f.FetchLatestVersion(ctx)
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(stopCh)
			<-doneCh
		})
	}
}

// GetCurrentUserAgent returns the Codex Desktop style User-Agent for the cached version.
func (f *CodexCLIVersionFetcher) GetCurrentUserAgent() string {
	return CodexCLIUserAgentForVersion(f.GetCurrentVersion())
}

// GetDefaultUserAgent returns the codex_cli_rs style fallback User-Agent for the cached version.
func (f *CodexCLIVersionFetcher) GetDefaultUserAgent() string {
	return CodexCLIDefaultUserAgentForVersion(f.GetCurrentVersion())
}

// GetCurrentCodexCLIVersion returns the global cached Codex CLI version.
func GetCurrentCodexCLIVersion() string {
	return GetGlobalCodexCLIVersionFetcher().GetCurrentVersion()
}

// GetCurrentCodexCLIUserAgent returns the global Codex Desktop style User-Agent.
func GetCurrentCodexCLIUserAgent() string {
	return GetGlobalCodexCLIVersionFetcher().GetCurrentUserAgent()
}

// GetCurrentCodexCLIDefaultUserAgent returns the global codex_cli_rs style User-Agent.
func GetCurrentCodexCLIDefaultUserAgent() string {
	return GetGlobalCodexCLIVersionFetcher().GetDefaultUserAgent()
}

// CodexCLIUserAgentForVersion builds the upstream Codex Desktop request shape.
func CodexCLIUserAgentForVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = CodexCLIDefaultVersion
	}
	return fmt.Sprintf("Codex Desktop/%s (Windows 10.0.26200; x86_64) unknown (codex_exec; %s)", version, version)
}

// CodexCLIDefaultUserAgentForVersion builds the configurable fallback Codex CLI UA.
func CodexCLIDefaultUserAgentForVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = CodexCLIDefaultVersion
	}
	return fmt.Sprintf("codex_cli_rs/%s (Ubuntu 22.4.0; x86_64) xterm-256color", version)
}

func parseCodexCLIVersionOutput(output string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("empty codex --version output")
	}
	version := codexCLIVersionPattern.FindString(output)
	if version == "" {
		return "", fmt.Errorf("no semver version found in codex --version output: %q", output)
	}
	return version, nil
}

func runCodexCLIVersionCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
