package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// CodexCLIDefaultVersion is the fallback version used when the npm registry
	// is unreachable at startup. Keep it aligned with the latest published
	// @openai/codex release so upstream version gates are satisfied even offline.
	CodexCLIDefaultVersion = "0.141.0"

	defaultCodexCLIVersionSyncInterval = 30 * time.Minute
	// defaultCodexCLIRegistryURL points at the npm "latest" manifest endpoint,
	// mirroring the Claude Code fetcher. Server deployments have no local Codex
	// binary, so the version must come from a remote source, not `codex --version`.
	defaultCodexCLIRegistryURL = "https://registry.npmjs.org/@openai/codex/latest"
)

// CodexCLIVersionFetcher resolves the latest Codex CLI version from npm.
type CodexCLIVersionFetcher struct {
	mu             sync.RWMutex
	currentVersion string
	lastFetchTime  time.Time
	fetchInterval  time.Duration
	httpClient     *http.Client
	registryURL    string
}

// npmCodexRegistryResponse npm registry API 响应结构
type npmCodexRegistryResponse struct {
	Version  string `json:"version"`
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
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
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		registryURL: defaultCodexCLIRegistryURL,
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

// FetchLatestVersion pulls the latest @openai/codex version from the npm registry.
// 失败时保持当前缓存值，不阻断服务启动。
func (f *CodexCLIVersionFetcher) FetchLatestVersion(ctx context.Context) (string, error) {
	if f == nil {
		return CodexCLIDefaultVersion, fmt.Errorf("nil Codex CLI version fetcher")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// 检查是否需要更新（避免频繁请求）
	if time.Since(f.lastFetchTime) < f.fetchInterval {
		return f.currentVersion, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", f.registryURL, nil)
	if err != nil {
		return f.currentVersion, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return f.currentVersion, fmt.Errorf("failed to fetch from npm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return f.currentVersion, fmt.Errorf("npm registry returned status %d", resp.StatusCode)
	}

	var npmResp npmCodexRegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&npmResp); err != nil {
		return f.currentVersion, fmt.Errorf("failed to parse npm response: %w", err)
	}

	latestVersion := strings.TrimSpace(npmResp.Version)
	if latestVersion == "" {
		latestVersion = strings.TrimSpace(npmResp.DistTags.Latest)
	}
	if latestVersion == "" {
		return f.currentVersion, fmt.Errorf("empty version from npm registry")
	}

	f.currentVersion = latestVersion
	f.lastFetchTime = time.Now()
	return latestVersion, nil
}

// StartBackgroundSync starts immediate and periodic Codex CLI version sync.
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
