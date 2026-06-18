package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultClaudeCodeRegistryURL = "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"

// VersionFetcher 负责从 npm registry 获取最新 Claude Code CLI 版本
type VersionFetcher struct {
	mu             sync.RWMutex
	currentVersion string
	lastFetchTime  time.Time
	fetchInterval  time.Duration
	httpClient     *http.Client
	registryURL    string
}

// npmRegistryResponse npm registry API 响应结构
type npmRegistryResponse struct {
	Version  string `json:"version"`
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
}

var (
	globalFetcher     *VersionFetcher
	globalFetcherOnce sync.Once
)

// GetGlobalFetcher 获取全局版本拉取器单例
func GetGlobalFetcher() *VersionFetcher {
	globalFetcherOnce.Do(func() {
		globalFetcher = NewVersionFetcher(30 * time.Minute)
	})
	return globalFetcher
}

// NewVersionFetcher 创建新的版本拉取器
// fetchInterval: 拉取间隔，建议 30 分钟以上避免频繁请求 npm
func NewVersionFetcher(fetchInterval time.Duration) *VersionFetcher {
	if fetchInterval <= 0 {
		fetchInterval = 30 * time.Minute
	}
	return &VersionFetcher{
		currentVersion: CLICurrentVersion, // 初始值使用常量作为 fallback
		fetchInterval:  fetchInterval,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		registryURL: defaultClaudeCodeRegistryURL,
	}
}

// GetCurrentVersion 获取当前缓存的版本号（线程安全）
func (f *VersionFetcher) GetCurrentVersion() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.currentVersion
}

// FetchLatestVersion 从 npm registry 拉取最新版本号
// 返回版本号和错误，失败时返回当前缓存值
func (f *VersionFetcher) FetchLatestVersion(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 检查是否需要更新（避免频繁请求）
	if time.Since(f.lastFetchTime) < f.fetchInterval {
		return f.currentVersion, nil
	}

	// 构造请求
	req, err := http.NewRequestWithContext(ctx, "GET", f.registryURL, nil)
	if err != nil {
		return f.currentVersion, fmt.Errorf("failed to create request: %w", err)
	}

	// 发送请求
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return f.currentVersion, fmt.Errorf("failed to fetch from npm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return f.currentVersion, fmt.Errorf("npm registry returned status %d", resp.StatusCode)
	}

	// 解析响应
	var npmResp npmRegistryResponse
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

	// 更新缓存
	f.currentVersion = latestVersion
	f.lastFetchTime = time.Now()

	return latestVersion, nil
}

// StartBackgroundSync 启动后台定期同步任务
// 返回 stop 函数用于停止同步
func (f *VersionFetcher) StartBackgroundSync(ctx context.Context) func() {
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(f.fetchInterval)
		defer ticker.Stop()

		// 启动时立即拉取一次
		if _, err := f.FetchLatestVersion(ctx); err != nil {
			// 静默失败，保持使用 fallback 版本
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				if _, err := f.FetchLatestVersion(ctx); err != nil {
					// 静默失败，保持使用上次成功的版本
				}
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

// UserAgentForCLIVersion returns the Claude Code CLI User-Agent for version.
func UserAgentForCLIVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = CLICurrentVersion
	}
	return fmt.Sprintf("claude-cli/%s (external, cli)", version)
}

// HeadersForCLIVersion returns a copy of DefaultHeaders with a dynamic CLI version.
func HeadersForCLIVersion(version string) map[string]string {
	headers := make(map[string]string, len(DefaultHeaders))
	for key, value := range DefaultHeaders {
		headers[key] = value
	}
	headers["User-Agent"] = UserAgentForCLIVersion(version)
	return headers
}

// DefaultHeaders returns Claude Code headers for the fetcher's cached version.
func (f *VersionFetcher) DefaultHeaders() map[string]string {
	if f == nil {
		return HeadersForCLIVersion(CLICurrentVersion)
	}
	return HeadersForCLIVersion(f.GetCurrentVersion())
}

// DefaultHeadersForCurrentVersion returns Claude Code headers for the global cached version.
func DefaultHeadersForCurrentVersion() map[string]string {
	return GetGlobalFetcher().DefaultHeaders()
}

// GetCurrentCLIVersion 获取当前 CLI 版本（全局便捷方法）
func GetCurrentCLIVersion() string {
	return GetGlobalFetcher().GetCurrentVersion()
}

// GetCurrentUserAgent 获取当前 User-Agent（动态版本）
func GetCurrentUserAgent() string {
	return UserAgentForCLIVersion(GetCurrentCLIVersion())
}
