package repository

import (
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// httpUpstreamCloseTrackingTransport 记录客户端空闲连接关闭次数。
type httpUpstreamCloseTrackingTransport struct {
	// closeIdleCalls 表示 CloseIdleConnections 的累计调用次数。
	closeIdleCalls atomic.Int64
}

// RoundTrip 满足 http.RoundTripper；本测试只验证连接清理，不发送请求。
func (t *httpUpstreamCloseTrackingTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected round trip")
}

// CloseIdleConnections 记录客户端触发的空闲连接清理。
func (t *httpUpstreamCloseTrackingTransport) CloseIdleConnections() {
	t.closeIdleCalls.Add(1)
}

// TestHTTPUpstreamPoolMetricsSnapshotAndOpsEvents 验证客户端缓存计数、容量和空闲年龄快照。
func TestHTTPUpstreamPoolMetricsSnapshotAndOpsEvents(t *testing.T) {
	svc := newHTTPUpstreamPoolTestService(&config.GatewayConfig{
		ConnectionPoolIsolation: config.ConnectionPoolIsolationAccount,
	})
	before := service.SnapshotOpsRuntimeMetrics()

	entry := mustGetOrCreateClient(t, svc, "", 7, 2)
	reused := mustGetOrCreateClient(t, svc, "", 7, 2)
	require.Same(t, entry, reused)
	atomic.StoreInt64(&entry.lastUsed, time.Now().Add(-3*time.Second).UnixNano())

	snapshot := svc.SnapshotHTTPUpstreamPoolMetrics()
	require.Equal(t, int64(1), snapshot.CacheHitTotal)
	require.Equal(t, int64(1), snapshot.CacheMissTotal)
	require.Equal(t, int64(1), snapshot.CacheCreateTotal)
	require.Zero(t, snapshot.CacheEvictTotal)
	require.Equal(t, int64(1), snapshot.Entries)
	require.Equal(t, int64(svc.maxUpstreamClients()), snapshot.Capacity)
	require.Zero(t, snapshot.InFlight)
	require.GreaterOrEqual(t, snapshot.OldestIdleAgeMs, int64(2900))

	svc.mu.Lock()
	require.True(t, svc.evictOldestIdleLocked())
	svc.mu.Unlock()
	after := service.SnapshotOpsRuntimeMetrics()

	snapshot = svc.SnapshotHTTPUpstreamPoolMetrics()
	require.Equal(t, int64(1), snapshot.CacheEvictTotal)
	require.Zero(t, snapshot.Entries)
	require.Equal(t, uint64(2), httpUpstreamPoolEventDelta(before, after, service.OpsConnectionPoolEventAcquire))
	require.Equal(t, uint64(1), httpUpstreamPoolEventDelta(before, after, service.OpsConnectionPoolEventReuse))
	require.Equal(t, uint64(1), httpUpstreamPoolEventDelta(before, after, service.OpsConnectionPoolEventDial))
	require.Equal(t, uint64(1), httpUpstreamPoolEventDelta(before, after, service.OpsConnectionPoolEventEvict))
}

// TestHTTPUpstreamEvictionDefersCloseUntilInFlightReleased 验证淘汰活动条目时不会提前关闭连接。
func TestHTTPUpstreamEvictionDefersCloseUntilInFlightReleased(t *testing.T) {
	svc := newHTTPUpstreamPoolTestService(nil)
	transport := &httpUpstreamCloseTrackingTransport{}
	entry := &upstreamClientEntry{client: &http.Client{Transport: transport}}
	atomic.StoreInt64(&entry.lastUsed, time.Now().UnixNano())
	svc.markClientInFlight(entry)
	svc.clients["active"] = entry

	svc.mu.Lock()
	svc.removeClientLocked("active", entry)
	svc.mu.Unlock()

	snapshot := svc.SnapshotHTTPUpstreamPoolMetrics()
	require.Zero(t, transport.closeIdleCalls.Load(), "活动请求释放前不得关闭连接")
	require.Zero(t, snapshot.Entries)
	require.Equal(t, int64(1), snapshot.InFlight)
	require.Equal(t, int64(1), snapshot.CacheEvictTotal)

	svc.releaseClient(entry)
	require.Equal(t, int64(1), transport.closeIdleCalls.Load(), "最后一个请求释放后应关闭退休条目的空闲连接")
	require.Zero(t, svc.SnapshotHTTPUpstreamPoolMetrics().InFlight)
	entry.closeIdleConnectionsIfRetired()
	require.Equal(t, int64(1), transport.closeIdleCalls.Load(), "重复清理不得重复关闭")
}

// TestHTTPUpstreamProxyChangeCleansIdlePreviousEntry 验证账号代理变化后清理旧客户端。
func TestHTTPUpstreamProxyChangeCleansIdlePreviousEntry(t *testing.T) {
	svc := newHTTPUpstreamPoolTestService(&config.GatewayConfig{
		ConnectionPoolIsolation: config.ConnectionPoolIsolationAccount,
	})
	oldEntry := mustGetOrCreateClient(t, svc, "http://proxy-a.local:8080", 9, 2)
	oldTransport := &httpUpstreamCloseTrackingTransport{}
	oldEntry.client.Transport = oldTransport

	newEntry := mustGetOrCreateClient(t, svc, "http://proxy-b.local:8080", 9, 2)
	require.NotSame(t, oldEntry, newEntry)
	require.False(t, hasEntry(svc, oldEntry))
	require.Equal(t, int64(1), oldTransport.closeIdleCalls.Load())

	snapshot := svc.SnapshotHTTPUpstreamPoolMetrics()
	require.Equal(t, int64(2), snapshot.CacheMissTotal)
	require.Equal(t, int64(2), snapshot.CacheCreateTotal)
	require.Equal(t, int64(1), snapshot.CacheEvictTotal)
	require.Equal(t, int64(1), snapshot.Entries)
}

// TestHTTPUpstreamOpenAIBaseURLChangeEvictsExpiredIdleEntry 验证 base URL 变化时会清理已过期的旧客户端。
func TestHTTPUpstreamOpenAIBaseURLChangeEvictsExpiredIdleEntry(t *testing.T) {
	svc := newHTTPUpstreamPoolTestService(&config.GatewayConfig{
		ConnectionPoolIsolation: config.ConnectionPoolIsolationAccountProxy,
		ClientIdleTTLSeconds:    1,
	})
	oldEntry, oldCacheKey := acquireOpenAIPoolEntryForTest(t, svc, 11, "https://relay-a.example.com/v1/responses")
	svc.releaseClient(oldEntry)
	oldTransport := &httpUpstreamCloseTrackingTransport{}
	oldEntry.client.Transport = oldTransport
	atomic.StoreInt64(&oldEntry.lastUsed, time.Now().Add(-2*time.Second).UnixNano())

	newEntry, newCacheKey := acquireOpenAIPoolEntryForTest(t, svc, 11, "https://relay-b.example.com/v1/responses")
	t.Cleanup(func() { svc.releaseClient(newEntry) })

	require.NotEqual(t, oldCacheKey, newCacheKey)
	require.NotSame(t, oldEntry, newEntry)
	require.False(t, hasEntry(svc, oldEntry))
	require.Equal(t, int64(1), oldTransport.closeIdleCalls.Load())
	snapshot := svc.SnapshotHTTPUpstreamPoolMetrics()
	require.Equal(t, int64(1), snapshot.CacheEvictTotal)
	require.Equal(t, int64(1), snapshot.Entries)
	require.Equal(t, int64(1), snapshot.InFlight)
}

// TestOpenAIHTTP2FallbackTTLRestoresHTTP2 验证回退 TTL 到期后协议恢复为 HTTP/2。
func TestOpenAIHTTP2FallbackTTLRestoresHTTP2(t *testing.T) {
	svc := newHTTPUpstreamPoolTestService(&config.GatewayConfig{
		OpenAIHTTP2: config.GatewayOpenAIHTTP2Config{
			Enabled:                   true,
			AllowProxyFallbackToHTTP1: true,
		},
	})
	proxyKey, parsedProxy, err := normalizeProxyURL("http://proxy.local:8080")
	require.NoError(t, err)
	fallbackKey := buildOpenAIHTTP2FallbackKey(12, proxyKey, "https://relay.example.com/v1/responses")
	state := svc.getOrCreateOpenAIHTTP2FallbackState(fallbackKey)
	startedAt := time.Now()
	activated, fallbackUntil := state.recordFailure(startedAt, 1, time.Minute, 2*time.Second)

	require.True(t, activated)
	require.Equal(t, startedAt.Add(2*time.Second), fallbackUntil)
	require.Equal(t, upstreamProtocolModeOpenAIH1Fallback, svc.resolveOpenAIProtocolMode(parsedProxy, fallbackKey, nil))
	require.True(t, state.isFallbackActive(startedAt.Add(2*time.Second-time.Nanosecond)))
	require.False(t, state.isFallbackActive(startedAt.Add(2*time.Second)))
	require.Equal(t, upstreamProtocolModeOpenAIH2, svc.resolveOpenAIProtocolMode(parsedProxy, fallbackKey, nil))
}

// newHTTPUpstreamPoolTestService 创建启用本地地址的客户端池测试实例。
func newHTTPUpstreamPoolTestService(gateway *config.GatewayConfig) *httpUpstreamService {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{AllowPrivateHosts: true},
		},
	}
	if gateway != nil {
		cfg.Gateway = *gateway
	}
	return NewHTTPUpstream(cfg).(*httpUpstreamService)
}

// acquireOpenAIPoolEntryForTest 使用指定 base URL 获取 OpenAI 客户端池条目。
func acquireOpenAIPoolEntryForTest(t *testing.T, svc *httpUpstreamService, accountID int64, requestBaseURL string) (*upstreamClientEntry, string) {
	t.Helper()
	proxyKey, parsedProxy, err := normalizeProxyURL("")
	require.NoError(t, err)
	protocolMode := upstreamProtocolModeOpenAIH2
	fallbackKey := buildOpenAIHTTP2FallbackKey(accountID, proxyKey, requestBaseURL)
	cacheKey := buildOpenAICacheKey(buildCacheKey(svc.getIsolationMode(), proxyKey, accountID), requestBaseURL, protocolMode)
	settings := svc.resolveOpenAIPoolSettings()
	poolKey := svc.buildOpenAIPoolKey(2, protocolMode, settings)
	entry, err := svc.getOrCreateOpenAIClient(cacheKey, poolKey, proxyKey, fallbackKey, 2, parsedProxy, nil, protocolMode, settings)
	require.NoError(t, err)
	return entry, cacheKey
}

// httpUpstreamPoolEventDelta 返回两份快照间指定上游 HTTP 连接池事件的增量。
func httpUpstreamPoolEventDelta(before, after service.OpsRuntimeMetricsSnapshot, event service.OpsConnectionPoolEvent) uint64 {
	return httpUpstreamPoolEventCount(after, event) - httpUpstreamPoolEventCount(before, event)
}

// httpUpstreamPoolEventCount 返回快照中指定上游 HTTP 连接池事件的累计值。
func httpUpstreamPoolEventCount(snapshot service.OpsRuntimeMetricsSnapshot, event service.OpsConnectionPoolEvent) uint64 {
	for _, counter := range snapshot.ConnectionPools {
		if counter.Name == string(service.OpsConnectionPoolUpstreamHTTP) && counter.Event == string(event) {
			return counter.Count
		}
	}
	return 0
}
