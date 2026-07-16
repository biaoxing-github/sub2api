package repository

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// httpClientSink 用于防止编译器优化掉基准测试中的赋值操作
// 这是 Go 基准测试的常见模式，确保测试结果准确
var httpClientSink *http.Client

// BenchmarkHTTPUpstreamProxyClient 对比重复创建与复用代理客户端的开销
//
// 测试目的：
// - 验证连接池复用相比每次新建的性能提升
// - 量化内存分配差异
//
// 预期结果：
// - "复用" 子测试应显著快于 "新建"
// - "复用" 子测试应零内存分配
func BenchmarkHTTPUpstreamProxyClient(b *testing.B) {
	// 创建测试配置
	cfg := &config.Config{
		Gateway: config.GatewayConfig{ResponseHeaderTimeout: 300},
	}
	upstream := NewHTTPUpstream(cfg)
	svc, ok := upstream.(*httpUpstreamService)
	if !ok {
		b.Fatalf("类型断言失败，无法获取 httpUpstreamService")
	}

	proxyURL := "http://127.0.0.1:8080"
	b.ReportAllocs() // 报告内存分配统计

	// 子测试：每次新建客户端
	// 模拟未优化前的行为，每次请求都创建新的 http.Client
	b.Run("新建", func(b *testing.B) {
		parsedProxy, err := url.Parse(proxyURL)
		if err != nil {
			b.Fatalf("解析代理地址失败: %v", err)
		}
		settings := defaultPoolSettings(cfg)
		for i := 0; i < b.N; i++ {
			// 每次迭代都创建新客户端，包含 Transport 分配
			transport, err := buildUpstreamTransport(settings, parsedProxy)
			if err != nil {
				b.Fatalf("创建 Transport 失败: %v", err)
			}
			httpClientSink = &http.Client{
				Transport: transport,
			}
		}
	})

	// 子测试：复用已缓存的客户端
	// 模拟优化后的行为，从缓存获取客户端
	b.Run("复用", func(b *testing.B) {
		// 预热：确保客户端已缓存
		entry, err := svc.getOrCreateClient(proxyURL, 1, 1)
		if err != nil {
			b.Fatalf("getOrCreateClient: %v", err)
		}
		client := entry.client
		b.ResetTimer() // 重置计时器，排除预热时间
		for i := 0; i < b.N; i++ {
			// 直接使用缓存的客户端，无内存分配
			httpClientSink = client
		}
	})
}

// BenchmarkHTTPUpstreamPoolTopologies 测量三类中转站拓扑的真实客户端缓存获取成本。
func BenchmarkHTTPUpstreamPoolTopologies(b *testing.B) {
	b.Run("共享代理_128账号", func(b *testing.B) {
		svc := newHTTPUpstreamPoolBenchmarkService(config.ConnectionPoolIsolationProxy)
		benchmarkHTTPUpstreamClients(b, svc, func(index int) (string, int64) {
			return "http://shared-proxy.local:8080", int64(index%128 + 1)
		})
	})

	b.Run("账号独享代理_64账号", func(b *testing.B) {
		svc := newHTTPUpstreamPoolBenchmarkService(config.ConnectionPoolIsolationAccountProxy)
		proxies := make([]string, 64)
		for index := range proxies {
			proxies[index] = fmt.Sprintf("http://proxy-%d.local:8080", index)
		}
		benchmarkHTTPUpstreamClients(b, svc, func(index int) (string, int64) {
			accountIndex := index % len(proxies)
			return proxies[accountIndex], int64(accountIndex + 1)
		})
	})

	b.Run("多BaseURL中转站_16地址", func(b *testing.B) {
		svc := newHTTPUpstreamPoolBenchmarkService(config.ConnectionPoolIsolationAccountProxy)
		baseURLs := make([]string, 16)
		for index := range baseURLs {
			baseURLs[index] = fmt.Sprintf("https://relay-%d.example.com/v1/responses", index)
		}
		proxyKey, parsedProxy, err := normalizeProxyURL("")
		if err != nil {
			b.Fatalf("normalize proxy: %v", err)
		}
		settings := svc.resolveOpenAIPoolSettings()
		poolKey := svc.buildOpenAIPoolKey(8, upstreamProtocolModeOpenAIH2, settings)

		acquire := func(index int) {
			accountIndex := index % len(baseURLs)
			accountID := int64(accountIndex + 1)
			baseURL := baseURLs[accountIndex]
			fallbackKey := buildOpenAIHTTP2FallbackKey(accountID, proxyKey, baseURL)
			cacheKey := buildOpenAICacheKey(buildCacheKey(svc.getIsolationMode(), proxyKey, accountID), baseURL, upstreamProtocolModeOpenAIH2)
			entry, getErr := svc.getOrCreateOpenAIClient(cacheKey, poolKey, proxyKey, fallbackKey, 8, parsedProxy, nil, upstreamProtocolModeOpenAIH2, settings)
			if getErr != nil {
				b.Fatalf("get OpenAI client: %v", getErr)
			}
			svc.releaseClient(entry)
		}
		for index := range len(baseURLs) {
			acquire(index)
		}
		before := svc.SnapshotHTTPUpstreamPoolMetrics()

		b.ReportAllocs()
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			acquire(index)
		}
		b.StopTimer()
		reportHTTPUpstreamPoolBenchmarkMetrics(b, before, svc.SnapshotHTTPUpstreamPoolMetrics())
	})
}

// benchmarkHTTPUpstreamClients 预热拓扑后测量通用客户端缓存查找。
func benchmarkHTTPUpstreamClients(
	b *testing.B,
	svc *httpUpstreamService,
	resolve func(index int) (proxyURL string, accountID int64),
) {
	const warmEntries = 128
	for index := range warmEntries {
		proxyURL, accountID := resolve(index)
		if _, err := svc.getOrCreateClient(proxyURL, accountID, 8); err != nil {
			b.Fatalf("warm client cache: %v", err)
		}
	}
	before := svc.SnapshotHTTPUpstreamPoolMetrics()

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		proxyURL, accountID := resolve(index)
		if _, err := svc.getOrCreateClient(proxyURL, accountID, 8); err != nil {
			b.Fatalf("get cached client: %v", err)
		}
	}
	b.StopTimer()
	reportHTTPUpstreamPoolBenchmarkMetrics(b, before, svc.SnapshotHTTPUpstreamPoolMetrics())
}

// newHTTPUpstreamPoolBenchmarkService 创建容量足够覆盖基准拓扑的客户端池。
func newHTTPUpstreamPoolBenchmarkService(isolation string) *httpUpstreamService {
	return NewHTTPUpstream(&config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{AllowPrivateHosts: true},
		},
		Gateway: config.GatewayConfig{
			ConnectionPoolIsolation: isolation,
			MaxUpstreamClients:      512,
		},
	}).(*httpUpstreamService)
}

// reportHTTPUpstreamPoolBenchmarkMetrics 输出拓扑条目数和计时窗口内的缓存命中率。
func reportHTTPUpstreamPoolBenchmarkMetrics(
	b *testing.B,
	before service.HTTPUpstreamPoolMetricsSnapshot,
	after service.HTTPUpstreamPoolMetricsSnapshot,
) {
	b.Helper()
	hits := after.CacheHitTotal - before.CacheHitTotal
	misses := after.CacheMissTotal - before.CacheMissTotal
	total := hits + misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}
	b.ReportMetric(float64(after.Entries), "entries")
	b.ReportMetric(hitRate, "cache_hit_%")
}
