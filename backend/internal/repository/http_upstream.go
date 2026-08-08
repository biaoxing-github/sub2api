package repository

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"golang.org/x/mod/semver"
)

// 默认配置常量
// 这些值在配置文件未指定时作为回退默认值使用
const (
	// directProxyKey: 无代理时的缓存键标识
	directProxyKey = "direct"
	// defaultMaxIdleConns: 默认最大空闲连接总数
	// HTTP/2 场景下，单连接可多路复用，240 足以支撑高并发
	defaultMaxIdleConns = 240
	// defaultMaxIdleConnsPerHost: 默认每主机最大空闲连接数
	defaultMaxIdleConnsPerHost = 120
	// defaultMaxConnsPerHost: 默认每主机最大连接数（含活跃连接）
	// 达到上限后新请求会等待，而非无限创建连接
	defaultMaxConnsPerHost = 240
	// defaultIdleConnTimeout: 默认空闲连接超时时间（90秒）
	// 超时后连接会被关闭，释放系统资源（建议小于上游 LB 超时）
	defaultIdleConnTimeout = 90 * time.Second
	// defaultResponseHeaderTimeout: 默认等待响应头超时时间（5分钟）
	// LLM 请求可能排队较久，需要较长超时
	defaultResponseHeaderTimeout = 300 * time.Second
	// defaultMaxUpstreamClients: 默认最大客户端缓存数量
	// 超出后会淘汰最久未使用的客户端
	defaultMaxUpstreamClients = 5000
	// defaultClientIdleTTLSeconds: 默认客户端空闲回收阈值（15分钟）
	defaultClientIdleTTLSeconds = 900
	// defaultOpenAIHTTP2FallbackErrorThreshold: OpenAI HTTP/2 兼容错误触发 HTTP/1.1 回退的默认阈值。
	defaultOpenAIHTTP2FallbackErrorThreshold = 2
	// defaultOpenAIHTTP2FallbackWindow: OpenAI HTTP/2 兼容错误统计窗口。
	defaultOpenAIHTTP2FallbackWindow = 60 * time.Second
	// defaultOpenAIHTTP2FallbackTTL: OpenAI HTTP/1.1 回退状态默认保持时长。
	defaultOpenAIHTTP2FallbackTTL = 10 * time.Minute
)

const (
	grokCLIProxyHost       = "cli-chat-proxy.grok.com"
	grokCLIStableVersion   = "0.2.114"
	grokCLIVersionOverride = "XAI_GROK_CLI_VERSION"
)

const (
	upstreamProtocolModeDefault          = service.HTTPUpstreamProtocolModeDefault
	upstreamProtocolModeOpenAIH1         = service.HTTPUpstreamProtocolModeOpenAIH1
	upstreamProtocolModeOpenAIH2         = service.HTTPUpstreamProtocolModeOpenAIH2
	upstreamProtocolModeOpenAIH1Fallback = service.HTTPUpstreamProtocolModeOpenAIH1Fallback
)

var errUpstreamClientLimitReached = errors.New("upstream client cache limit reached")

// poolSettings 连接池配置参数
// 封装 Transport 所需的各项连接池参数
type poolSettings struct {
	maxIdleConns          int           // 最大空闲连接总数
	maxIdleConnsPerHost   int           // 每主机最大空闲连接数
	maxConnsPerHost       int           // 每主机最大连接数（含活跃）
	idleConnTimeout       time.Duration // 空闲连接超时时间
	responseHeaderTimeout time.Duration // 等待响应头超时时间
}

// openAIHTTP2Settings OpenAI HTTP/2 协议策略。
type openAIHTTP2Settings struct {
	enabled                   bool
	allowProxyFallbackToHTTP1 bool
	fallbackErrorThreshold    int
	fallbackWindow            time.Duration
	fallbackTTL               time.Duration
}

// openAIHTTP2FallbackState 记录 OpenAI HTTP/2 在单个回退键上的失败窗口。
type openAIHTTP2FallbackState struct {
	mu            sync.Mutex
	windowStart   time.Time
	errorCount    int
	fallbackUntil time.Time
}

// upstreamClientEntry 上游客户端缓存条目
// 记录客户端实例及其元数据，用于连接池管理和淘汰策略
type upstreamClientEntry struct {
	client       *http.Client // HTTP 客户端实例
	proxyKey     string       // 代理标识（用于检测代理变更）
	poolKey      string       // 连接池配置标识（用于检测配置变更）
	protocolMode string       // 协议模式（用于 OpenAI HTTP/2/H1 回退隔离）
	fallbackKey  string       // OpenAI HTTP/2 回退隔离键（账号+代理+request_base_url）
	lastUsed     int64        // 最后使用时间戳（纳秒），用于 LRU 淘汰
	inFlight     int64        // 当前进行中的请求数，>0 时不可淘汰
	retired      atomic.Bool  // 是否已从活动缓存移除，退休条目在请求归零后关闭空闲连接
	closeOnce    sync.Once    // 保证退休条目的空闲连接只关闭一次
}

// httpUpstreamPoolMetrics 保存上游 HTTP 客户端缓存的固定维度指标。
type httpUpstreamPoolMetrics struct {
	cacheHitTotal    atomic.Int64 // 缓存命中累计次数
	cacheMissTotal   atomic.Int64 // 缓存未命中累计次数
	cacheCreateTotal atomic.Int64 // 客户端创建累计次数
	cacheEvictTotal  atomic.Int64 // 客户端淘汰累计次数
	inFlight         atomic.Int64 // 活动及退休条目的进行中请求数
}

// httpUpstreamService 通用 HTTP 上游服务
// 用于向任意 HTTP API（Claude、OpenAI 等）发送请求，支持可选代理
//
// 架构设计：
// - 根据隔离策略（proxy/account/account_proxy）缓存客户端实例
// - 每个客户端拥有独立的 Transport 连接池
// - 支持 LRU + 空闲时间双重淘汰策略
//
// 性能优化：
// 1. 根据隔离策略缓存客户端实例，避免频繁创建 http.Client
// 2. 复用 Transport 连接池，减少 TCP 握手和 TLS 协商开销
// 3. 支持账号级隔离与空闲回收，降低连接层关联风险
// 4. 达到最大连接数后等待可用连接，而非无限创建
// 5. 仅回收空闲客户端，避免中断活跃请求
// 6. HTTP/2 多路复用，连接上限不等于并发请求上限
// 7. 代理变更时清空旧连接池，避免复用错误代理
// 8. 账号并发数与连接池上限对应（账号隔离策略下）
type httpUpstreamService struct {
	cfg                  *config.Config                  // 全局配置
	mu                   sync.RWMutex                    // 保护 clients map 的读写锁
	clients              map[string]*upstreamClientEntry // 客户端缓存池，key 由隔离策略决定
	openAIHTTP2Fallbacks sync.Map                        // OpenAI HTTP/2 兼容性回退状态，key=账号+代理+baseURL
	metrics              httpUpstreamPoolMetrics         // 客户端缓存固定维度指标
}

// NewHTTPUpstream 创建通用 HTTP 上游服务
// 使用配置中的连接池参数构建 Transport
//
// 参数:
//   - cfg: 全局配置，包含连接池参数和隔离策略
//
// 返回:
//   - service.HTTPUpstream 接口实现
func NewHTTPUpstream(cfg *config.Config) service.HTTPUpstream {
	return &httpUpstreamService{
		cfg:     cfg,
		clients: make(map[string]*upstreamClientEntry),
	}
}

// SnapshotHTTPUpstreamPoolMetrics 返回上游 HTTP 客户端缓存的当前快照。
func (s *httpUpstreamService) SnapshotHTTPUpstreamPoolMetrics() service.HTTPUpstreamPoolMetricsSnapshot {
	if s == nil {
		return service.HTTPUpstreamPoolMetricsSnapshot{}
	}

	nowUnix := time.Now().UnixNano()
	oldestIdleAgeNanos := int64(0)
	s.mu.RLock()
	entries := int64(len(s.clients))
	for _, entry := range s.clients {
		if entry == nil || atomic.LoadInt64(&entry.inFlight) != 0 {
			continue
		}
		lastUsed := atomic.LoadInt64(&entry.lastUsed)
		if lastUsed <= 0 || lastUsed > nowUnix {
			continue
		}
		if idleAge := nowUnix - lastUsed; idleAge > oldestIdleAgeNanos {
			oldestIdleAgeNanos = idleAge
		}
	}
	s.mu.RUnlock()

	return service.HTTPUpstreamPoolMetricsSnapshot{
		CacheHitTotal:    s.metrics.cacheHitTotal.Load(),
		CacheMissTotal:   s.metrics.cacheMissTotal.Load(),
		CacheCreateTotal: s.metrics.cacheCreateTotal.Load(),
		CacheEvictTotal:  s.metrics.cacheEvictTotal.Load(),
		Entries:          entries,
		Capacity:         int64(s.maxUpstreamClients()),
		InFlight:         s.metrics.inFlight.Load(),
		OldestIdleAgeMs:  time.Duration(oldestIdleAgeNanos).Milliseconds(),
	}
}

// recordClientAcquire 记录一次客户端缓存获取尝试。
func (s *httpUpstreamService) recordClientAcquire() {
	service.RecordOpsConnectionPoolEvent(service.OpsConnectionPoolUpstreamHTTP, service.OpsConnectionPoolEventAcquire)
}

// recordClientCacheHit 记录一次客户端缓存命中。
func (s *httpUpstreamService) recordClientCacheHit() {
	s.metrics.cacheHitTotal.Add(1)
	service.RecordOpsConnectionPoolEvent(service.OpsConnectionPoolUpstreamHTTP, service.OpsConnectionPoolEventReuse)
}

// recordClientCacheMiss 记录一次需要创建或重建客户端的缓存未命中。
func (s *httpUpstreamService) recordClientCacheMiss() {
	s.metrics.cacheMissTotal.Add(1)
}

// recordClientCreated 记录一次客户端创建成功。
func (s *httpUpstreamService) recordClientCreated() {
	s.metrics.cacheCreateTotal.Add(1)
	service.RecordOpsConnectionPoolEvent(service.OpsConnectionPoolUpstreamHTTP, service.OpsConnectionPoolEventDial)
}

// recordClientEvicted 记录一次客户端从活动缓存移除。
func (s *httpUpstreamService) recordClientEvicted() {
	s.metrics.cacheEvictTotal.Add(1)
	service.RecordOpsConnectionPoolEvent(service.OpsConnectionPoolUpstreamHTTP, service.OpsConnectionPoolEventEvict)
}

// markClientInFlight 标记客户端开始承载一个请求。
func (s *httpUpstreamService) markClientInFlight(entry *upstreamClientEntry) {
	atomic.AddInt64(&entry.inFlight, 1)
	s.metrics.inFlight.Add(1)
}

// releaseClient 释放客户端上的请求占用，并关闭已经退休且完全空闲的连接。
func (s *httpUpstreamService) releaseClient(entry *upstreamClientEntry) {
	remaining := atomic.AddInt64(&entry.inFlight, -1)
	s.metrics.inFlight.Add(-1)
	atomic.StoreInt64(&entry.lastUsed, time.Now().UnixNano())
	if remaining == 0 {
		entry.closeIdleConnectionsIfRetired()
	}
}

// retire 从活动缓存淘汰条目；仍有请求时推迟连接清理到最后一次释放。
func (entry *upstreamClientEntry) retire() {
	if entry == nil {
		return
	}
	entry.retired.Store(true)
	entry.closeIdleConnectionsIfRetired()
}

// closeIdleConnectionsIfRetired 只关闭已退休且没有进行中请求的空闲连接。
func (entry *upstreamClientEntry) closeIdleConnectionsIfRetired() {
	if entry == nil || !entry.retired.Load() || atomic.LoadInt64(&entry.inFlight) != 0 {
		return
	}
	entry.closeOnce.Do(func() {
		if entry.client != nil {
			entry.client.CloseIdleConnections()
		}
	})
}

// Do 执行 HTTP 请求
// 根据隔离策略获取或创建客户端，并跟踪请求生命周期
//
// 参数:
//   - req: HTTP 请求对象
//   - proxyURL: 代理地址，空字符串表示直连
//   - accountID: 账户 ID，用于账户级隔离
//   - accountConcurrency: 账户并发限制，用于动态调整连接池大小
//
// 返回:
//   - *http.Response: HTTP 响应（Body 已包装，关闭时自动更新计数）
//   - error: 请求错误
//
// 注意:
//   - 调用方必须关闭 resp.Body，否则会导致 inFlight 计数泄漏
//   - inFlight > 0 的客户端不会被淘汰，确保活跃请求不被中断
func (s *httpUpstreamService) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	applyGrokCLIProxyHeaders(req)
	ensureUpstreamUserAgent(req)
	if err := s.validateRequestHost(req); err != nil {
		return nil, err
	}
	if service.HTTPUpstreamProfileFromContext(req.Context()) == service.HTTPUpstreamProfileOpenAI {
		return s.doOpenAIProfile(req, proxyURL, accountID, accountConcurrency, nil)
	}

	// 获取或创建对应的客户端，并标记请求占用
	entry, err := s.acquireClient(proxyURL, accountID, accountConcurrency)
	if err != nil {
		return nil, err
	}

	// 执行请求
	resp, err := entry.client.Do(req)
	if err != nil {
		// 请求失败，立即减少计数
		s.releaseClient(entry)
		return nil, err
	}

	// 如果上游返回了压缩内容，解压后再交给业务层
	decompressResponseBody(resp)

	// 包装响应体，在关闭时自动减少计数并更新时间戳
	// 这确保了流式响应（如 SSE）在完全读取前不会被淘汰
	resp.Body = wrapTrackedBody(resp.Body, func() {
		s.releaseClient(entry)
	})

	return resp, nil
}

// DoWithTLS 执行带 TLS 指纹伪装的 HTTP 请求
//
// profile 为 nil 时不启用 TLS 指纹，行为与 Do 方法相同。
// profile 非 nil 时使用指定的 Profile 进行 TLS 指纹伪装。
func (s *httpUpstreamService) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	if profile == nil {
		return s.Do(req, proxyURL, accountID, accountConcurrency)
	}
	applyGrokCLIProxyHeaders(req)
	ensureUpstreamUserAgent(req)
	if err := s.validateRequestHost(req); err != nil {
		return nil, err
	}
	if service.HTTPUpstreamProfileFromContext(req.Context()) == service.HTTPUpstreamProfileOpenAI {
		return s.doOpenAIProfile(req, proxyURL, accountID, accountConcurrency, profile)
	}

	targetHost := ""
	if req != nil && req.URL != nil {
		targetHost = req.URL.Host
	}
	proxyInfo := "direct"
	if proxyURL != "" {
		proxyInfo = proxyURL
	}
	slog.Debug("tls_fingerprint_enabled", "account_id", accountID, "target", targetHost, "proxy", proxyInfo, "profile", profile.Name)

	if err := s.validateRequestHost(req); err != nil {
		return nil, err
	}

	entry, err := s.acquireClientWithTLS(proxyURL, accountID, accountConcurrency, profile)
	if err != nil {
		slog.Debug("tls_fingerprint_acquire_client_failed", "account_id", accountID, "error", err)
		return nil, err
	}

	resp, err := entry.client.Do(req)
	if err != nil {
		s.releaseClient(entry)
		slog.Debug("tls_fingerprint_request_failed", "account_id", accountID, "error", err)
		return nil, err
	}

	decompressResponseBody(resp)

	resp.Body = wrapTrackedBody(resp.Body, func() {
		s.releaseClient(entry)
	})

	return resp, nil
}

// applyGrokCLIProxyHeaders 在最终传输边界为官方 Grok CLI proxy 注入客户端身份。
func applyGrokCLIProxyHeaders(req *http.Request) {
	if req == nil || req.URL == nil || !strings.EqualFold(strings.TrimSpace(req.URL.Hostname()), grokCLIProxyHost) {
		return
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	version := strings.TrimSpace(os.Getenv(grokCLIVersionOverride))
	if !isSupportedGrokCLIVersion(version) {
		version = grokCLIStableVersion
	}
	req.Header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	req.Header.Set("x-grok-client-version", version)
	req.Header.Set("User-Agent", "xai-grok-workspace/"+version)
}

func isSupportedGrokCLIVersion(version string) bool {
	canonical := "v" + version
	minimum := "v" + grokCLIStableVersion
	return semver.IsValid(canonical) && semver.Canonical(canonical) == canonical && semver.Compare(canonical, minimum) >= 0
}

// ensureUpstreamUserAgent 在最终传输边界兜底 User-Agent：
// 业务层未设置 UA 或显式带入 Go 默认 UA 时补 Codex CLI UA，禁止出站请求落到
// Go-http-client/1.1——部分渠道按客户端 UA 风控会直接 403。
// 其他已设置 UA（账号自定义、各平台自设）一律不覆盖。
func ensureUpstreamUserAgent(req *http.Request) {
	if req == nil {
		return
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	userAgent := strings.TrimSpace(req.Header.Get("User-Agent"))
	if userAgent != "" && !strings.HasPrefix(strings.ToLower(userAgent), "go-http-client/") {
		return
	}
	req.Header.Set("User-Agent", openai.GetCurrentCodexCLIUserAgent())
}

func (s *httpUpstreamService) doOpenAIProfile(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, tlsProfile *tlsfingerprint.Profile) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if err := s.validateRequestHost(req); err != nil {
		return nil, err
	}

	attemptInfo := service.HTTPUpstreamAttemptInfoFromContext(req.Context())
	requestBaseURL := ""
	if attemptInfo != nil {
		requestBaseURL = attemptInfo.RequestBaseURL
	}
	proxyKey, parsedProxy, err := normalizeProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	settings := s.resolveOpenAIPoolSettings()
	fallbackKey := buildOpenAIHTTP2FallbackKey(accountID, proxyKey, requestBaseURL)
	protocolMode := s.resolveOpenAIProtocolMode(parsedProxy, fallbackKey, tlsProfile)
	cacheKey := buildOpenAICacheKey(buildCacheKey(s.getIsolationMode(), proxyKey, accountID), requestBaseURL, protocolMode)
	cacheKey = buildOpenAITransportCacheKey(cacheKey, accountID, tlsProfile, req.Header.Get("Authorization"))
	poolKey := s.buildOpenAIPoolKey(accountConcurrency, protocolMode, settings)

	entry, err := s.getOrCreateOpenAIClient(cacheKey, poolKey, proxyKey, fallbackKey, accountConcurrency, parsedProxy, tlsProfile, protocolMode, settings)
	if err != nil {
		return nil, err
	}
	if attemptInfo != nil {
		attemptInfo.ProtocolMode = protocolMode
		attemptInfo.FallbackKey = fallbackKey
	}

	resp, err := entry.client.Do(req)
	if err != nil {
		s.recordOpenAIHTTP2Failure(protocolMode, fallbackKey, err)
		s.releaseClient(entry)
		return nil, err
	}

	s.recordOpenAIHTTP2Success(protocolMode, fallbackKey)
	decompressResponseBody(resp)
	resp.Body = wrapTrackedBody(resp.Body, func() {
		s.releaseClient(entry)
	})
	return resp, nil
}

func (s *httpUpstreamService) getOrCreateOpenAIClient(cacheKey, poolKey, proxyKey, fallbackKey string, accountConcurrency int, parsedProxy *url.URL, profile *tlsfingerprint.Profile, protocolMode string, settings poolSettings) (*upstreamClientEntry, error) {
	now := time.Now()
	nowUnix := now.UnixNano()
	isolation := s.getIsolationMode()
	s.recordClientAcquire()

	s.mu.RLock()
	if entry, ok := s.clients[cacheKey]; ok && s.shouldReuseOpenAIEntry(entry, isolation, proxyKey, poolKey, protocolMode, fallbackKey) {
		atomic.StoreInt64(&entry.lastUsed, nowUnix)
		s.markClientInFlight(entry)
		s.recordClientCacheHit()
		s.mu.RUnlock()
		return entry, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	if entry, ok := s.clients[cacheKey]; ok {
		if s.shouldReuseOpenAIEntry(entry, isolation, proxyKey, poolKey, protocolMode, fallbackKey) {
			atomic.StoreInt64(&entry.lastUsed, nowUnix)
			s.markClientInFlight(entry)
			s.recordClientCacheHit()
			s.mu.Unlock()
			return entry, nil
		}
		s.removeClientLocked(cacheKey, entry)
	}
	s.recordClientCacheMiss()

	if s.maxUpstreamClients() > 0 {
		s.evictIdleLocked(now)
		if len(s.clients) >= s.maxUpstreamClients() {
			if !s.evictOldestIdleLocked() {
				s.mu.Unlock()
				return nil, errUpstreamClientLimitReached
			}
		}
	}

	transport, err := buildOpenAIUpstreamTransport(settings, parsedProxy, profile, protocolMode)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("build openai transport: %w", err)
	}
	client := &http.Client{Transport: transport}
	if s.shouldValidateResolvedIP() {
		client.CheckRedirect = s.redirectChecker
	}
	entry := &upstreamClientEntry{
		client:       client,
		proxyKey:     proxyKey,
		poolKey:      poolKey,
		protocolMode: protocolMode,
		fallbackKey:  fallbackKey,
	}
	atomic.StoreInt64(&entry.lastUsed, nowUnix)
	s.markClientInFlight(entry)
	s.clients[cacheKey] = entry
	s.recordClientCreated()
	s.evictIdleLocked(now)
	s.evictOverLimitLocked()
	s.mu.Unlock()
	return entry, nil
}

func (s *httpUpstreamService) shouldReuseOpenAIEntry(entry *upstreamClientEntry, isolation, proxyKey, poolKey, protocolMode, fallbackKey string) bool {
	if !s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
		return false
	}
	if entry.protocolMode != protocolMode {
		return false
	}
	if entry.fallbackKey != fallbackKey {
		return false
	}
	return true
}

func (s *httpUpstreamService) resolveOpenAIPoolSettings() poolSettings {
	settings := defaultPoolSettings(s.cfg)
	settings.responseHeaderTimeout = 0
	return settings
}

func (s *httpUpstreamService) buildOpenAIPoolKey(accountConcurrency int, protocolMode string, settings poolSettings) string {
	base := fmt.Sprintf(
		"idle:%d|idle_host:%d|max:%d|idle_timeout:%s|header_timeout:%s|proto:%s",
		settings.maxIdleConns,
		settings.maxIdleConnsPerHost,
		settings.maxConnsPerHost,
		settings.idleConnTimeout,
		settings.responseHeaderTimeout,
		protocolMode,
	)
	if accountConcurrency > 0 {
		return fmt.Sprintf("%s|account:%d", base, accountConcurrency)
	}
	return base
}

func buildOpenAICacheKey(baseCacheKey, requestBaseURL, protocolMode string) string {
	base := strings.TrimSpace(baseCacheKey)
	if base == "" {
		base = "openai"
	}
	baseURL := normalizeOpenAIHTTP2RequestBaseURL(requestBaseURL)
	if baseURL == "" {
		baseURL = "default"
	}
	proto := strings.TrimSpace(protocolMode)
	if proto == "" {
		proto = upstreamProtocolModeDefault
	}
	return fmt.Sprintf("%s|openai_base:%s|openai_proto:%s", base, baseURL, proto)
}

// buildOpenAITransportCacheKey 按账号、TLS 指纹和授权摘要隔离底层 HTTP 连接。
func buildOpenAITransportCacheKey(baseCacheKey string, accountID int64, profile *tlsfingerprint.Profile, authorization string) string {
	authorizationSum := sha256.Sum256([]byte(strings.TrimSpace(authorization)))
	return fmt.Sprintf(
		"%s|openai_account:%d|openai_tls:%s|openai_auth:%x",
		baseCacheKey,
		accountID,
		tlsfingerprint.ProfileCacheKey(profile),
		authorizationSum[:],
	)
}

func buildOpenAIHTTP2FallbackKey(accountID int64, proxyKey, requestBaseURL string) string {
	return fmt.Sprintf("account:%d|proxy:%s|base:%s", accountID, strings.TrimSpace(proxyKey), normalizeOpenAIHTTP2RequestBaseURL(requestBaseURL))
}

func normalizeOpenAIHTTP2RequestBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, "/responses")
	trimmed = strings.TrimSuffix(trimmed, "/codex")
	return strings.ToLower(trimmed)
}

func (s *httpUpstreamService) resolveOpenAIProtocolMode(parsedProxy *url.URL, fallbackKey string, tlsProfile *tlsfingerprint.Profile) string {
	if tlsProfile != nil {
		return upstreamProtocolModeOpenAIH1
	}
	settings := s.resolveOpenAIHTTP2Settings()
	if !settings.enabled {
		return upstreamProtocolModeOpenAIH1
	}
	if parsedProxy == nil {
		return upstreamProtocolModeOpenAIH2
	}
	scheme := strings.ToLower(strings.TrimSpace(parsedProxy.Scheme))
	if scheme != "http" && scheme != "https" {
		return upstreamProtocolModeOpenAIH2
	}
	if settings.allowProxyFallbackToHTTP1 && s.isOpenAIHTTP2FallbackActive(fallbackKey) {
		return upstreamProtocolModeOpenAIH1Fallback
	}
	return upstreamProtocolModeOpenAIH2
}

func (s *httpUpstreamService) resolveOpenAIHTTP2Settings() openAIHTTP2Settings {
	settings := openAIHTTP2Settings{
		enabled:                   false,
		allowProxyFallbackToHTTP1: true,
		fallbackErrorThreshold:    defaultOpenAIHTTP2FallbackErrorThreshold,
		fallbackWindow:            defaultOpenAIHTTP2FallbackWindow,
		fallbackTTL:               defaultOpenAIHTTP2FallbackTTL,
	}
	if s == nil || s.cfg == nil {
		return settings
	}
	cfg := s.cfg.Gateway.OpenAIHTTP2
	settings.enabled = cfg.Enabled
	settings.allowProxyFallbackToHTTP1 = cfg.AllowProxyFallbackToHTTP1
	if cfg.FallbackErrorThreshold > 0 {
		settings.fallbackErrorThreshold = cfg.FallbackErrorThreshold
	}
	if cfg.FallbackWindowSeconds > 0 {
		settings.fallbackWindow = time.Duration(cfg.FallbackWindowSeconds) * time.Second
	}
	if cfg.FallbackTTLSeconds > 0 {
		settings.fallbackTTL = time.Duration(cfg.FallbackTTLSeconds) * time.Second
	}
	return settings
}

func (s *httpUpstreamService) isOpenAIHTTP2FallbackActive(fallbackKey string) bool {
	raw, ok := s.openAIHTTP2Fallbacks.Load(fallbackKey)
	if !ok {
		return false
	}
	state, ok := raw.(*openAIHTTP2FallbackState)
	if !ok || state == nil {
		return false
	}
	return state.isFallbackActive(time.Now())
}

func (s *httpUpstreamService) getOrCreateOpenAIHTTP2FallbackState(fallbackKey string) *openAIHTTP2FallbackState {
	state := &openAIHTTP2FallbackState{}
	actual, _ := s.openAIHTTP2Fallbacks.LoadOrStore(fallbackKey, state)
	cached, ok := actual.(*openAIHTTP2FallbackState)
	if !ok || cached == nil {
		return state
	}
	return cached
}

func isOpenAIHTTP2CompatibilityError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "stream error") ||
		strings.Contains(msg, "http2: transport") ||
		strings.Contains(msg, "http2: no cached connection") ||
		strings.Contains(msg, "http2: timeout awaiting response headers") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "protocol error") ||
		strings.Contains(msg, "goaway") {
		return true
	}
	return false
}

func (s *httpUpstreamService) recordOpenAIHTTP2Failure(protocolMode, fallbackKey string, err error) {
	if s == nil {
		return
	}
	if protocolMode != upstreamProtocolModeOpenAIH2 {
		return
	}
	if !isOpenAIHTTP2CompatibilityError(err) {
		return
	}
	settings := s.resolveOpenAIHTTP2Settings()
	if !settings.enabled || !settings.allowProxyFallbackToHTTP1 {
		return
	}
	state := s.getOrCreateOpenAIHTTP2FallbackState(fallbackKey)
	activated, until := state.recordFailure(time.Now(), settings.fallbackErrorThreshold, settings.fallbackWindow, settings.fallbackTTL)
	if activated {
		slog.Warn("openai_http2_proxy_fallback_activated", "fallback_key", fallbackKey, "fallback_until", until.Format(time.RFC3339))
	}
}

func (s *httpUpstreamService) recordOpenAIHTTP2Success(protocolMode, fallbackKey string) {
	if s == nil {
		return
	}
	if protocolMode == upstreamProtocolModeOpenAIH1Fallback {
		slog.Debug("openai_http1_fallback_hit", "fallback_key", fallbackKey)
	}
	if protocolMode != upstreamProtocolModeOpenAIH2 {
		return
	}
	raw, ok := s.openAIHTTP2Fallbacks.Load(fallbackKey)
	if !ok {
		return
	}
	state, ok := raw.(*openAIHTTP2FallbackState)
	if !ok || state == nil {
		return
	}
	state.resetErrorWindow()
}

func (s *openAIHTTP2FallbackState) isFallbackActive(now time.Time) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fallbackUntil.IsZero() {
		return false
	}
	if now.Before(s.fallbackUntil) {
		return true
	}
	s.fallbackUntil = time.Time{}
	return false
}

func (s *openAIHTTP2FallbackState) resetErrorWindow() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.windowStart = time.Time{}
	s.errorCount = 0
}

func (s *openAIHTTP2FallbackState) recordFailure(now time.Time, threshold int, window, ttl time.Duration) (bool, time.Time) {
	if s == nil {
		return false, time.Time{}
	}
	if threshold <= 0 {
		threshold = defaultOpenAIHTTP2FallbackErrorThreshold
	}
	if window <= 0 {
		window = defaultOpenAIHTTP2FallbackWindow
	}
	if ttl <= 0 {
		ttl = defaultOpenAIHTTP2FallbackTTL
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.fallbackUntil.IsZero() && now.Before(s.fallbackUntil) {
		return false, s.fallbackUntil
	}
	if s.windowStart.IsZero() || now.Sub(s.windowStart) > window {
		s.windowStart = now
		s.errorCount = 0
	}
	s.errorCount++
	if s.errorCount < threshold {
		return false, time.Time{}
	}
	s.fallbackUntil = now.Add(ttl)
	s.windowStart = time.Time{}
	s.errorCount = 0
	return true, s.fallbackUntil
}

// acquireClientWithTLS 获取或创建带 TLS 指纹的客户端
func (s *httpUpstreamService) acquireClientWithTLS(proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*upstreamClientEntry, error) {
	return s.getClientEntryWithTLS(proxyURL, accountID, accountConcurrency, profile, true, true)
}

// getClientEntryWithTLS 获取或创建带 TLS 指纹的客户端条目
// TLS 指纹客户端使用独立的缓存键，与普通客户端隔离
func (s *httpUpstreamService) getClientEntryWithTLS(proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile, markInFlight bool, enforceLimit bool) (*upstreamClientEntry, error) {
	isolation := s.getIsolationMode()
	proxyKey, parsedProxy, err := normalizeProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	s.recordClientAcquire()
	// TLS 指纹客户端使用独立的缓存键，加 "tls:" 前缀
	cacheKey := "tls:" + buildCacheKey(isolation, proxyKey, accountID)
	poolKey := s.buildPoolKey(isolation, accountConcurrency) + ":tls"

	now := time.Now()
	nowUnix := now.UnixNano()

	// 读锁快速路径
	s.mu.RLock()
	if entry, ok := s.clients[cacheKey]; ok && s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
		atomic.StoreInt64(&entry.lastUsed, nowUnix)
		if markInFlight {
			s.markClientInFlight(entry)
		}
		s.recordClientCacheHit()
		s.mu.RUnlock()
		slog.Debug("tls_fingerprint_reusing_client", "account_id", accountID, "cache_key", cacheKey)
		return entry, nil
	}
	s.mu.RUnlock()

	// 写锁慢路径
	s.mu.Lock()
	if entry, ok := s.clients[cacheKey]; ok {
		if s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
			atomic.StoreInt64(&entry.lastUsed, nowUnix)
			if markInFlight {
				s.markClientInFlight(entry)
			}
			s.recordClientCacheHit()
			s.mu.Unlock()
			slog.Debug("tls_fingerprint_reusing_client", "account_id", accountID, "cache_key", cacheKey)
			return entry, nil
		}
		slog.Debug("tls_fingerprint_evicting_stale_client",
			"account_id", accountID,
			"cache_key", cacheKey,
			"proxy_changed", entry.proxyKey != proxyKey,
			"pool_changed", entry.poolKey != poolKey)
		s.removeClientLocked(cacheKey, entry)
	}
	s.recordClientCacheMiss()

	// 超出缓存上限时尝试淘汰
	if enforceLimit && s.maxUpstreamClients() > 0 {
		s.evictIdleLocked(now)
		if len(s.clients) >= s.maxUpstreamClients() {
			if !s.evictOldestIdleLocked() {
				s.mu.Unlock()
				return nil, errUpstreamClientLimitReached
			}
		}
	}

	// 创建带 TLS 指纹的 Transport
	slog.Debug("tls_fingerprint_creating_new_client", "account_id", accountID, "cache_key", cacheKey, "proxy", proxyKey)
	settings := s.resolvePoolSettings(isolation, accountConcurrency)
	transport, err := buildUpstreamTransportWithTLSFingerprint(settings, parsedProxy, profile)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("build TLS fingerprint transport: %w", err)
	}

	client := &http.Client{Transport: transport}
	if s.shouldValidateResolvedIP() {
		client.CheckRedirect = s.redirectChecker
	}

	entry := &upstreamClientEntry{
		client:   client,
		proxyKey: proxyKey,
		poolKey:  poolKey,
	}
	atomic.StoreInt64(&entry.lastUsed, nowUnix)
	if markInFlight {
		s.markClientInFlight(entry)
	}
	s.clients[cacheKey] = entry
	s.recordClientCreated()

	s.evictIdleLocked(now)
	s.evictOverLimitLocked()
	s.mu.Unlock()
	return entry, nil
}

func (s *httpUpstreamService) shouldValidateResolvedIP() bool {
	if s.cfg == nil {
		return false
	}
	if !s.cfg.Security.URLAllowlist.Enabled {
		return false
	}
	return !s.cfg.Security.URLAllowlist.AllowPrivateHosts
}

func (s *httpUpstreamService) validateRequestHost(req *http.Request) error {
	if !s.shouldValidateResolvedIP() {
		return nil
	}
	if req == nil || req.URL == nil {
		return errors.New("request url is nil")
	}
	host := strings.TrimSpace(req.URL.Hostname())
	if host == "" {
		return errors.New("request host is empty")
	}
	if err := urlvalidator.ValidateResolvedIP(host); err != nil {
		return err
	}
	return nil
}

func (s *httpUpstreamService) redirectChecker(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return s.validateRequestHost(req)
}

// acquireClient 获取或创建客户端，并标记为进行中请求
// 用于请求路径，避免在获取后被淘汰
func (s *httpUpstreamService) acquireClient(proxyURL string, accountID int64, accountConcurrency int) (*upstreamClientEntry, error) {
	return s.getClientEntry(proxyURL, accountID, accountConcurrency, true, true)
}

// getOrCreateClient 获取或创建客户端
// 根据隔离策略和参数决定缓存键，处理代理变更和配置变更
//
// 参数:
//   - proxyURL: 代理地址
//   - accountID: 账户 ID
//   - accountConcurrency: 账户并发限制
//
// 返回:
//   - *upstreamClientEntry: 客户端缓存条目
//
// 隔离策略说明:
//   - proxy: 按代理地址隔离，同一代理共享客户端
//   - account: 按账户隔离，同一账户共享客户端（代理变更时重建）
//   - account_proxy: 按账户+代理组合隔离，最细粒度
func (s *httpUpstreamService) getOrCreateClient(proxyURL string, accountID int64, accountConcurrency int) (*upstreamClientEntry, error) {
	return s.getClientEntry(proxyURL, accountID, accountConcurrency, false, false)
}

// getClientEntry 获取或创建客户端条目
// markInFlight=true 时会标记进行中请求，用于请求路径防止被淘汰
// enforceLimit=true 时会限制客户端数量，超限且无法淘汰时返回错误
func (s *httpUpstreamService) getClientEntry(proxyURL string, accountID int64, accountConcurrency int, markInFlight bool, enforceLimit bool) (*upstreamClientEntry, error) {
	// 获取隔离模式
	isolation := s.getIsolationMode()
	// 标准化代理 URL 并解析
	proxyKey, parsedProxy, err := normalizeProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	s.recordClientAcquire()
	// 构建缓存键（根据隔离策略不同）
	cacheKey := buildCacheKey(isolation, proxyKey, accountID)
	// 构建连接池配置键（用于检测配置变更）
	poolKey := s.buildPoolKey(isolation, accountConcurrency)

	now := time.Now()
	nowUnix := now.UnixNano()

	// 读锁快速路径：命中缓存直接返回，减少锁竞争
	s.mu.RLock()
	if entry, ok := s.clients[cacheKey]; ok && s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
		atomic.StoreInt64(&entry.lastUsed, nowUnix)
		if markInFlight {
			s.markClientInFlight(entry)
		}
		s.recordClientCacheHit()
		s.mu.RUnlock()
		return entry, nil
	}
	s.mu.RUnlock()

	// 写锁慢路径：创建或重建客户端
	s.mu.Lock()
	if entry, ok := s.clients[cacheKey]; ok {
		if s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
			atomic.StoreInt64(&entry.lastUsed, nowUnix)
			if markInFlight {
				s.markClientInFlight(entry)
			}
			s.recordClientCacheHit()
			s.mu.Unlock()
			return entry, nil
		}
		s.removeClientLocked(cacheKey, entry)
	}
	s.recordClientCacheMiss()

	// 超出缓存上限时尝试淘汰，无法淘汰则拒绝新建
	if enforceLimit && s.maxUpstreamClients() > 0 {
		s.evictIdleLocked(now)
		if len(s.clients) >= s.maxUpstreamClients() {
			if !s.evictOldestIdleLocked() {
				s.mu.Unlock()
				return nil, errUpstreamClientLimitReached
			}
		}
	}

	// 缓存未命中或需要重建，创建新客户端
	settings := s.resolvePoolSettings(isolation, accountConcurrency)
	transport, err := buildUpstreamTransport(settings, parsedProxy)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("build transport: %w", err)
	}
	client := &http.Client{Transport: transport}
	if s.shouldValidateResolvedIP() {
		client.CheckRedirect = s.redirectChecker
	}
	entry := &upstreamClientEntry{
		client:   client,
		proxyKey: proxyKey,
		poolKey:  poolKey,
	}
	atomic.StoreInt64(&entry.lastUsed, nowUnix)
	if markInFlight {
		s.markClientInFlight(entry)
	}
	s.clients[cacheKey] = entry
	s.recordClientCreated()

	// 执行淘汰策略：先淘汰空闲超时的，再淘汰超出数量限制的
	s.evictIdleLocked(now)
	s.evictOverLimitLocked()
	s.mu.Unlock()
	return entry, nil
}

// shouldReuseEntry 判断缓存条目是否可复用
// 若代理或连接池配置发生变化，则需要重建客户端
func (s *httpUpstreamService) shouldReuseEntry(entry *upstreamClientEntry, isolation, proxyKey, poolKey string) bool {
	if entry == nil {
		return false
	}
	if isolation == config.ConnectionPoolIsolationAccount && entry.proxyKey != proxyKey {
		return false
	}
	if entry.poolKey != poolKey {
		return false
	}
	return true
}

// removeClientLocked 移除客户端（需持有锁）
// 从缓存中删除并关闭空闲连接
//
// 参数:
//   - key: 缓存键
//   - entry: 客户端条目
func (s *httpUpstreamService) removeClientLocked(key string, entry *upstreamClientEntry) {
	delete(s.clients, key)
	s.recordClientEvicted()
	// 活跃条目只退出缓存；最后一个请求释放后再关闭其空闲连接。
	entry.retire()
}

// evictIdleLocked 淘汰空闲超时的客户端（需持有锁）
// 遍历所有客户端，移除超过 TTL 且无活跃请求的条目
//
// 参数:
//   - now: 当前时间
func (s *httpUpstreamService) evictIdleLocked(now time.Time) {
	ttl := s.clientIdleTTL()
	if ttl <= 0 {
		return
	}
	// 计算淘汰截止时间
	cutoff := now.Add(-ttl).UnixNano()
	for key, entry := range s.clients {
		// 跳过有活跃请求的客户端
		if atomic.LoadInt64(&entry.inFlight) != 0 {
			continue
		}
		// 淘汰超时的空闲客户端
		if atomic.LoadInt64(&entry.lastUsed) <= cutoff {
			s.removeClientLocked(key, entry)
		}
	}
}

// evictOldestIdleLocked 淘汰最久未使用且无活跃请求的客户端（需持有锁）
func (s *httpUpstreamService) evictOldestIdleLocked() bool {
	var (
		oldestKey   string
		oldestEntry *upstreamClientEntry
		oldestTime  int64
	)
	// 查找最久未使用且无活跃请求的客户端
	for key, entry := range s.clients {
		// 跳过有活跃请求的客户端
		if atomic.LoadInt64(&entry.inFlight) != 0 {
			continue
		}
		lastUsed := atomic.LoadInt64(&entry.lastUsed)
		if oldestEntry == nil || lastUsed < oldestTime {
			oldestKey = key
			oldestEntry = entry
			oldestTime = lastUsed
		}
	}
	// 所有客户端都有活跃请求，无法淘汰
	if oldestEntry == nil {
		return false
	}
	s.removeClientLocked(oldestKey, oldestEntry)
	return true
}

// evictOverLimitLocked 淘汰超出数量限制的客户端（需持有锁）
// 使用 LRU 策略，优先淘汰最久未使用且无活跃请求的客户端
func (s *httpUpstreamService) evictOverLimitLocked() bool {
	maxClients := s.maxUpstreamClients()
	if maxClients <= 0 {
		return false
	}
	evicted := false
	// 循环淘汰直到满足数量限制
	for len(s.clients) > maxClients {
		if !s.evictOldestIdleLocked() {
			return evicted
		}
		evicted = true
	}
	return evicted
}

// getIsolationMode 获取连接池隔离模式
// 从配置中读取，无效值回退到 account_proxy 模式
//
// 返回:
//   - string: 隔离模式（proxy/account/account_proxy）
func (s *httpUpstreamService) getIsolationMode() string {
	if s.cfg == nil {
		return config.ConnectionPoolIsolationAccountProxy
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.Gateway.ConnectionPoolIsolation))
	if mode == "" {
		return config.ConnectionPoolIsolationAccountProxy
	}
	switch mode {
	case config.ConnectionPoolIsolationProxy, config.ConnectionPoolIsolationAccount, config.ConnectionPoolIsolationAccountProxy:
		return mode
	default:
		return config.ConnectionPoolIsolationAccountProxy
	}
}

// maxUpstreamClients 获取最大客户端缓存数量
// 从配置中读取，无效值使用默认值
func (s *httpUpstreamService) maxUpstreamClients() int {
	if s.cfg == nil {
		return defaultMaxUpstreamClients
	}
	if s.cfg.Gateway.MaxUpstreamClients > 0 {
		return s.cfg.Gateway.MaxUpstreamClients
	}
	return defaultMaxUpstreamClients
}

// clientIdleTTL 获取客户端空闲回收阈值
// 从配置中读取，无效值使用默认值
func (s *httpUpstreamService) clientIdleTTL() time.Duration {
	if s.cfg == nil {
		return time.Duration(defaultClientIdleTTLSeconds) * time.Second
	}
	if s.cfg.Gateway.ClientIdleTTLSeconds > 0 {
		return time.Duration(s.cfg.Gateway.ClientIdleTTLSeconds) * time.Second
	}
	return time.Duration(defaultClientIdleTTLSeconds) * time.Second
}

// resolvePoolSettings 解析连接池配置
// 根据隔离策略和账户并发数动态调整连接池参数
//
// 参数:
//   - isolation: 隔离模式
//   - accountConcurrency: 账户并发限制
//
// 返回:
//   - poolSettings: 连接池配置
//
// 说明:
//   - 账户隔离模式下，连接池大小与账户并发数对应
//   - 这确保了单账户不会占用过多连接资源
func (s *httpUpstreamService) resolvePoolSettings(isolation string, accountConcurrency int) poolSettings {
	settings := defaultPoolSettings(s.cfg)
	// 账户隔离模式下，根据账户并发数调整连接池大小
	if (isolation == config.ConnectionPoolIsolationAccount || isolation == config.ConnectionPoolIsolationAccountProxy) && accountConcurrency > 0 {
		settings.maxIdleConns = accountConcurrency
		settings.maxIdleConnsPerHost = accountConcurrency
		settings.maxConnsPerHost = accountConcurrency
	}
	return settings
}

// buildPoolKey 构建连接池配置键
// 用于检测配置变更，配置变更时需要重建客户端
//
// 参数:
//   - isolation: 隔离模式
//   - accountConcurrency: 账户并发限制
//
// 返回:
//   - string: 配置键
func (s *httpUpstreamService) buildPoolKey(isolation string, accountConcurrency int) string {
	if isolation == config.ConnectionPoolIsolationAccount || isolation == config.ConnectionPoolIsolationAccountProxy {
		if accountConcurrency > 0 {
			return fmt.Sprintf("account:%d", accountConcurrency)
		}
	}
	return "default"
}

// buildCacheKey 构建客户端缓存键
// 根据隔离策略决定缓存键的组成
//
// 参数:
//   - isolation: 隔离模式
//   - proxyKey: 代理标识
//   - accountID: 账户 ID
//
// 返回:
//   - string: 缓存键
//
// 缓存键格式:
//   - proxy 模式: "proxy:{proxyKey}"
//   - account 模式: "account:{accountID}"
//   - account_proxy 模式: "account:{accountID}|proxy:{proxyKey}"
func buildCacheKey(isolation, proxyKey string, accountID int64) string {
	switch isolation {
	case config.ConnectionPoolIsolationAccount:
		return fmt.Sprintf("account:%d", accountID)
	case config.ConnectionPoolIsolationAccountProxy:
		return fmt.Sprintf("account:%d|proxy:%s", accountID, proxyKey)
	default:
		return fmt.Sprintf("proxy:%s", proxyKey)
	}
}

// normalizeProxyURL 标准化代理 URL
// 处理空值和解析错误，返回标准化的键和解析后的 URL
//
// 参数:
//   - raw: 原始代理 URL 字符串
//
// 返回:
//   - string: 标准化的代理键（空返回 "direct"）
//   - *url.URL: 解析后的 URL（空返回 nil）
//   - error: 非空代理 URL 解析失败时返回错误（禁止回退到直连）
func normalizeProxyURL(raw string) (string, *url.URL, error) {
	_, parsed, err := proxyurl.Parse(raw)
	if err != nil {
		return "", nil, err
	}
	if parsed == nil {
		return directProxyKey, nil, nil
	}
	// 规范化：小写 scheme/host，去除路径和查询参数
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.ForceQuery = false
	if hostname := parsed.Hostname(); hostname != "" {
		port := parsed.Port()
		if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
			port = ""
		}
		hostname = strings.ToLower(hostname)
		if port != "" {
			parsed.Host = net.JoinHostPort(hostname, port)
		} else {
			parsed.Host = hostname
		}
	}
	return parsed.String(), parsed, nil
}

// defaultPoolSettings 获取默认连接池配置
// 从全局配置中读取，无效值使用常量默认值
//
// 参数:
//   - cfg: 全局配置
//
// 返回:
//   - poolSettings: 连接池配置
func defaultPoolSettings(cfg *config.Config) poolSettings {
	maxIdleConns := defaultMaxIdleConns
	maxIdleConnsPerHost := defaultMaxIdleConnsPerHost
	maxConnsPerHost := defaultMaxConnsPerHost
	idleConnTimeout := defaultIdleConnTimeout
	responseHeaderTimeout := defaultResponseHeaderTimeout

	if cfg != nil {
		if cfg.Gateway.MaxIdleConns > 0 {
			maxIdleConns = cfg.Gateway.MaxIdleConns
		}
		if cfg.Gateway.MaxIdleConnsPerHost > 0 {
			maxIdleConnsPerHost = cfg.Gateway.MaxIdleConnsPerHost
		}
		if cfg.Gateway.MaxConnsPerHost >= 0 {
			maxConnsPerHost = cfg.Gateway.MaxConnsPerHost
		}
		if cfg.Gateway.IdleConnTimeoutSeconds > 0 {
			idleConnTimeout = time.Duration(cfg.Gateway.IdleConnTimeoutSeconds) * time.Second
		}
		if cfg.Gateway.ResponseHeaderTimeout > 0 {
			responseHeaderTimeout = time.Duration(cfg.Gateway.ResponseHeaderTimeout) * time.Second
		}
	}

	return poolSettings{
		maxIdleConns:          maxIdleConns,
		maxIdleConnsPerHost:   maxIdleConnsPerHost,
		maxConnsPerHost:       maxConnsPerHost,
		idleConnTimeout:       idleConnTimeout,
		responseHeaderTimeout: responseHeaderTimeout,
	}
}

// buildUpstreamTransport 构建上游请求的 Transport
// 使用配置文件中的连接池参数，支持生产环境调优
//
// 参数:
//   - settings: 连接池配置
//   - proxyURL: 代理 URL（nil 表示直连）
//
// 返回:
//   - *http.Transport: 配置好的 Transport 实例
//   - error: 代理配置错误
//
// Transport 参数说明:
//   - MaxIdleConns: 所有主机的最大空闲连接总数
//   - MaxIdleConnsPerHost: 每主机最大空闲连接数（影响连接复用率）
//   - MaxConnsPerHost: 每主机最大连接数（达到后新请求等待）
//   - IdleConnTimeout: 空闲连接超时（超时后关闭）
//   - ResponseHeaderTimeout: 等待响应头超时（不影响流式传输）
func buildUpstreamTransport(settings poolSettings, proxyURL *url.URL) (*http.Transport, error) {
	transport := &http.Transport{
		MaxIdleConns:          settings.maxIdleConns,
		MaxIdleConnsPerHost:   settings.maxIdleConnsPerHost,
		MaxConnsPerHost:       settings.maxConnsPerHost,
		IdleConnTimeout:       settings.idleConnTimeout,
		ResponseHeaderTimeout: settings.responseHeaderTimeout,
	}
	if err := proxyutil.ConfigureTransportProxy(transport, proxyURL); err != nil {
		return nil, err
	}
	return transport, nil
}

func buildOpenAIUpstreamTransport(settings poolSettings, proxyURL *url.URL, profile *tlsfingerprint.Profile, protocolMode string) (*http.Transport, error) {
	if profile != nil {
		return buildUpstreamTransportWithTLSFingerprint(settings, proxyURL, profile)
	}
	transport, err := buildUpstreamTransport(settings, proxyURL)
	if err != nil {
		return nil, err
	}
	switch protocolMode {
	case upstreamProtocolModeOpenAIH2:
		transport.ForceAttemptHTTP2 = true
	case upstreamProtocolModeOpenAIH1, upstreamProtocolModeOpenAIH1Fallback:
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	}
	return transport, nil
}

// buildUpstreamTransportWithTLSFingerprint 构建带 TLS 指纹伪装的 Transport
// 使用 utls 库模拟 Claude CLI 的 TLS 指纹
//
// 参数:
//   - settings: 连接池配置
//   - proxyURL: 代理 URL（nil 表示直连）
//   - profile: TLS 指纹配置
//
// 返回:
//   - *http.Transport: 配置好的 Transport 实例
//   - error: 配置错误
//
// 代理类型处理:
//   - nil/空: 直连，使用 TLSFingerprintDialer
//   - http/https: HTTP 代理，使用 HTTPProxyDialer（CONNECT 隧道 + utls 握手）
//   - socks5: SOCKS5 代理，使用 SOCKS5ProxyDialer（SOCKS5 隧道 + utls 握手）
func buildUpstreamTransportWithTLSFingerprint(settings poolSettings, proxyURL *url.URL, profile *tlsfingerprint.Profile) (*http.Transport, error) {
	transport := &http.Transport{
		MaxIdleConns:          settings.maxIdleConns,
		MaxIdleConnsPerHost:   settings.maxIdleConnsPerHost,
		MaxConnsPerHost:       settings.maxConnsPerHost,
		IdleConnTimeout:       settings.idleConnTimeout,
		ResponseHeaderTimeout: settings.responseHeaderTimeout,
		// 禁用默认的 TLS，我们使用自定义的 DialTLSContext
		ForceAttemptHTTP2: false,
	}

	// 根据代理类型选择合适的 TLS 指纹 Dialer
	if proxyURL == nil {
		// 直连：使用 TLSFingerprintDialer
		slog.Debug("tls_fingerprint_transport_direct")
		dialer := tlsfingerprint.NewDialer(profile, nil)
		transport.DialTLSContext = dialer.DialTLSContext
	} else {
		scheme := strings.ToLower(proxyURL.Scheme)
		switch scheme {
		case "socks5", "socks5h":
			// SOCKS5 代理：使用 SOCKS5ProxyDialer
			slog.Debug("tls_fingerprint_transport_socks5", "proxy", proxyURL.Host)
			socks5Dialer := tlsfingerprint.NewSOCKS5ProxyDialer(profile, proxyURL)
			transport.DialTLSContext = socks5Dialer.DialTLSContext
		case "http", "https":
			// HTTP/HTTPS 代理：使用 HTTPProxyDialer（CONNECT 隧道）
			slog.Debug("tls_fingerprint_transport_http_connect", "proxy", proxyURL.Host)
			httpDialer := tlsfingerprint.NewHTTPProxyDialer(profile, proxyURL)
			transport.DialTLSContext = httpDialer.DialTLSContext
		default:
			// 未知代理类型，回退到普通代理配置（无 TLS 指纹）
			slog.Debug("tls_fingerprint_transport_unknown_scheme_fallback", "scheme", scheme)
			if err := proxyutil.ConfigureTransportProxy(transport, proxyURL); err != nil {
				return nil, err
			}
		}
	}

	return transport, nil
}

// trackedBody 带跟踪功能的响应体包装器
// 在 Close 时执行回调，用于更新请求计数
type trackedBody struct {
	io.ReadCloser // 原始响应体
	once          sync.Once
	onClose       func() // 关闭时的回调函数
}

// Close 关闭响应体并执行回调
// 使用 sync.Once 确保回调只执行一次
func (b *trackedBody) Close() error {
	err := b.ReadCloser.Close()
	if b.onClose != nil {
		b.once.Do(b.onClose)
	}
	return err
}

// wrapTrackedBody 包装响应体以跟踪关闭事件
// 用于在响应体关闭时更新 inFlight 计数
//
// 参数:
//   - body: 原始响应体
//   - onClose: 关闭时的回调函数
//
// 返回:
//   - io.ReadCloser: 包装后的响应体
func wrapTrackedBody(body io.ReadCloser, onClose func()) io.ReadCloser {
	if body == nil {
		return body
	}
	return &trackedBody{ReadCloser: body, onClose: onClose}
}

// decompressResponseBody 根据 Content-Encoding 解压响应体。
// 当请求显式设置了 accept-encoding 时，Go 的 Transport 不会自动解压，需要手动处理。
// 解压成功后会删除 Content-Encoding 和 Content-Length header（长度已不准确）。
func decompressResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	ce := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if ce == "" {
		return
	}

	originalBody := resp.Body
	var reader io.Reader
	switch ce {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return // 解压失败，保持原样
		}
		reader = gr
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "deflate":
		reader = flate.NewReader(resp.Body)
	case "zstd":
		bufferedBody := bufio.NewReader(resp.Body)
		resp.Body = &decompressedBody{reader: bufferedBody, closer: originalBody}

		headerBytes, _ := bufferedBody.Peek(zstd.HeaderMaxSize)
		var header zstd.Header
		if err := header.Decode(headerBytes); err != nil {
			slog.Warn("zstd_decompress_failed", "error", err)
			return
		}

		zr, err := zstd.NewReader(bufferedBody)
		if err != nil {
			slog.Warn("zstd_decompress_failed", "error", err)
			return
		}
		reader = &zstdResponseReader{ReadCloser: zr.IOReadCloser()}
	default:
		return
	}

	resp.Body = &decompressedBody{reader: reader, closer: originalBody}
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length") // 解压后长度不确定
	resp.ContentLength = -1
}

// zstdResponseReader 在读取 zstd 解压流失败时记录一次警告，避免上游坏 2xx 静默吞掉解压错误。
type zstdResponseReader struct {
	io.ReadCloser
	warnOnce sync.Once
}

// Read 代理 zstd 解压读取，并把非 EOF 解压错误写入结构化日志。
func (r *zstdResponseReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.warnOnce.Do(func() {
			slog.Warn("zstd_decompress_failed", "error", err)
		})
	}
	return n, err
}

// decompressedBody 组合解压 reader 和原始 body 的 close。
type decompressedBody struct {
	reader io.Reader
	closer io.Closer
}

func (d *decompressedBody) Read(p []byte) (int, error) {
	return d.reader.Read(p)
}

func (d *decompressedBody) Close() error {
	// 如果 reader 本身也是 Closer（如 gzip.Reader），先关闭它
	if rc, ok := d.reader.(io.Closer); ok {
		_ = rc.Close()
	}
	return d.closer.Close()
}
