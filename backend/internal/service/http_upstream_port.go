package service

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// HTTPUpstream 上游 HTTP 请求接口
// 用于向上游 API（Claude、OpenAI、Gemini 等）发送请求
type HTTPUpstream interface {
	// Do 执行 HTTP 请求（不启用 TLS 指纹）
	Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error)

	// DoWithTLS 执行带 TLS 指纹伪装的 HTTP 请求
	//
	// profile 参数:
	//   - nil: 不启用 TLS 指纹，行为与 Do 方法相同
	//   - non-nil: 使用指定的 Profile 进行 TLS 指纹伪装
	//
	// Profile 由调用方通过 TLSFingerprintProfileService 解析后传入，
	// 支持按账号绑定的数据库 profile 或内置默认 profile。
	DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error)
}

// HTTPUpstreamPoolMetricsSnapshot 表示上游 HTTP 客户端缓存的进程内快照。
type HTTPUpstreamPoolMetricsSnapshot struct {
	// CacheHitTotal 表示复用已有客户端的累计次数。
	CacheHitTotal int64 `json:"cache_hit_total"`
	// CacheMissTotal 表示需要创建或重建客户端的累计次数。
	CacheMissTotal int64 `json:"cache_miss_total"`
	// CacheCreateTotal 表示成功创建客户端的累计次数。
	CacheCreateTotal int64 `json:"cache_create_total"`
	// CacheEvictTotal 表示从活动缓存移除客户端的累计次数。
	CacheEvictTotal int64 `json:"cache_evict_total"`
	// Entries 表示当前活动缓存条目数，不包含等待请求释放的退休条目。
	Entries int64 `json:"entries"`
	// Capacity 表示客户端缓存当前配置的最大活动条目数。
	Capacity int64 `json:"capacity"`
	// InFlight 表示所有活动及退休条目上的进行中请求数。
	InFlight int64 `json:"in_flight"`
	// OldestIdleAgeMs 表示当前活动缓存中最久未使用空闲条目的空闲毫秒数。
	OldestIdleAgeMs int64 `json:"oldest_idle_age_ms"`
}

// HTTPUpstreamPoolMetricsSource 是 HTTPUpstream 的可选观测接口。
// 调用方通过类型断言读取快照，避免强制所有测试桩实现指标方法。
type HTTPUpstreamPoolMetricsSource interface {
	// SnapshotHTTPUpstreamPoolMetrics 返回上游 HTTP 客户端缓存的当前快照。
	SnapshotHTTPUpstreamPoolMetrics() HTTPUpstreamPoolMetricsSnapshot
}
