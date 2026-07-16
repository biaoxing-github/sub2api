package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const (
	opsMetricsCollectorJobName     = "ops_metrics_collector"
	opsMetricsCollectorMinInterval = 60 * time.Second
	opsMetricsCollectorMaxInterval = 1 * time.Hour

	opsMetricsCollectorTimeout = 10 * time.Second

	opsMetricsCollectorLeaderLockKey = "ops:metrics:collector:leader"
	opsMetricsCollectorLeaderLockTTL = 90 * time.Second

	opsMetricsCollectorHeartbeatTimeout = 2 * time.Second

	bytesPerMB = 1024 * 1024
)

var opsMetricsCollectorAdvisoryLockID = hashAdvisoryLockID(opsMetricsCollectorLeaderLockKey)

// OpsRequestStage 表示一次上游请求的固定阶段。禁止使用账号、模型或上游名称作为阶段值。
type OpsRequestStage string

const (
	// OpsRequestStageUnknown 归并未识别的阶段，避免动态标签造成高基数。
	OpsRequestStageUnknown OpsRequestStage = "unknown"
	// OpsRequestStageSelection 表示调度器选择候选账号的阶段。
	OpsRequestStageSelection OpsRequestStage = "selection"
	// OpsRequestStageSlotWait 表示等待并发槽位的阶段。
	OpsRequestStageSlotWait OpsRequestStage = "slot_wait"
	// OpsRequestStageConnect 表示建立或复用上游连接的阶段。
	OpsRequestStageConnect OpsRequestStage = "connect"
	// OpsRequestStageHeaderWait 表示等待上游响应头的阶段。
	OpsRequestStageHeaderWait OpsRequestStage = "header_wait"
	// OpsRequestStageTTFT 表示从发送请求到收到首个流式 token 的阶段。
	OpsRequestStageTTFT OpsRequestStage = "ttft"
	// OpsRequestStageStream 表示首 token 后的流式传输阶段。
	OpsRequestStageStream OpsRequestStage = "stream"
)

// OpsRequestResult 表示请求阶段的固定结果分类。
type OpsRequestResult string

const (
	// OpsRequestResultUnknown 归并未识别的结果值。
	OpsRequestResultUnknown OpsRequestResult = "unknown"
	// OpsRequestResultSuccess 表示阶段成功完成。
	OpsRequestResultSuccess OpsRequestResult = "success"
	// OpsRequestResultFailure 表示阶段失败。
	OpsRequestResultFailure OpsRequestResult = "failure"
	// OpsRequestResultCanceled 表示调用方或上游取消了阶段。
	OpsRequestResultCanceled OpsRequestResult = "canceled"
)

// OpsRequestProtocol 表示连接协议的固定分类。
type OpsRequestProtocol string

const (
	// OpsRequestProtocolUnknown 归并未识别的协议值。
	OpsRequestProtocolUnknown OpsRequestProtocol = "unknown"
	// OpsRequestProtocolHTTP1 表示 HTTP/1.1 请求。
	OpsRequestProtocolHTTP1 OpsRequestProtocol = "http1"
	// OpsRequestProtocolHTTP2 表示 HTTP/2 请求。
	OpsRequestProtocolHTTP2 OpsRequestProtocol = "http2"
	// OpsRequestProtocolWebSocket 表示 WebSocket 请求。
	OpsRequestProtocolWebSocket OpsRequestProtocol = "websocket"
)

// OpsRequestErrorClass 表示固定的错误分类，禁止传入原始错误文本。
type OpsRequestErrorClass string

const (
	// OpsRequestErrorClassUnknown 归并未识别的错误分类。
	OpsRequestErrorClassUnknown OpsRequestErrorClass = "unknown"
	// OpsRequestErrorClassNone 表示阶段没有错误。
	OpsRequestErrorClassNone OpsRequestErrorClass = "none"
	// OpsRequestErrorClassTimeout 表示连接、响应头或流读取超时。
	OpsRequestErrorClassTimeout OpsRequestErrorClass = "timeout"
	// OpsRequestErrorClassConnection 表示拨号、TLS 或连接中断错误。
	OpsRequestErrorClassConnection OpsRequestErrorClass = "connection"
	// OpsRequestErrorClassRateLimit 表示上游限流。
	OpsRequestErrorClassRateLimit OpsRequestErrorClass = "rate_limit"
	// OpsRequestErrorClassUpstream4xx 表示上游 4xx 响应。
	OpsRequestErrorClassUpstream4xx OpsRequestErrorClass = "upstream_4xx"
	// OpsRequestErrorClassUpstream5xx 表示上游 5xx 响应。
	OpsRequestErrorClassUpstream5xx OpsRequestErrorClass = "upstream_5xx"
	// OpsRequestErrorClassCanceled 表示上下文取消。
	OpsRequestErrorClassCanceled OpsRequestErrorClass = "canceled"
	// OpsRequestErrorClassInternal 表示网关内部错误。
	OpsRequestErrorClassInternal OpsRequestErrorClass = "internal"
)

// OpsRequestMetricLabels 是请求阶段指标允许使用的低基数分类字段。
type OpsRequestMetricLabels struct {
	// Result 表示阶段结果；未知值将归并到 unknown。
	Result OpsRequestResult
	// Protocol 表示连接协议；未知值将归并到 unknown。
	Protocol OpsRequestProtocol
	// ErrorClass 表示错误分类；空值表示 none，未知值将归并到 unknown。
	ErrorClass OpsRequestErrorClass
}

// OpsSchedulingEvent 表示调度器的固定事件分类。
type OpsSchedulingEvent string

const (
	// OpsSchedulingEventUnknown 归并未识别的调度事件。
	OpsSchedulingEventUnknown OpsSchedulingEvent = "unknown"
	// OpsSchedulingEventSnapshotHit 表示命中调度快照。
	OpsSchedulingEventSnapshotHit OpsSchedulingEvent = "snapshot_hit"
	// OpsSchedulingEventSnapshotMiss 表示未命中调度快照。
	OpsSchedulingEventSnapshotMiss OpsSchedulingEvent = "snapshot_miss"
	// OpsSchedulingEventSlotWait 表示请求进入并发槽位等待。
	OpsSchedulingEventSlotWait OpsSchedulingEvent = "slot_wait"
	// OpsSchedulingEventSlotWaitTimeout 表示等待并发槽位超时。
	OpsSchedulingEventSlotWaitTimeout OpsSchedulingEvent = "slot_wait_timeout"
	// OpsSchedulingEventFailover 表示请求切换到下一个候选账号。
	OpsSchedulingEventFailover OpsSchedulingEvent = "failover"
)

// OpsCacheName 表示可观测缓存的固定名称。
type OpsCacheName string

const (
	// OpsCacheUnknown 归并未识别的缓存名称。
	OpsCacheUnknown OpsCacheName = "unknown"
	// OpsCacheAPIKeyL1 表示进程内 API Key 一级缓存。
	OpsCacheAPIKeyL1 OpsCacheName = "api_key_l1"
	// OpsCacheAPIKeyL2 表示 Redis API Key 二级缓存。
	OpsCacheAPIKeyL2 OpsCacheName = "api_key_l2"
	// OpsCacheBilling 表示计费缓存。
	OpsCacheBilling OpsCacheName = "billing"
	// OpsCacheContextJournal 表示 Redis Context Journal 缓存。
	OpsCacheContextJournal OpsCacheName = "context_journal"
	// OpsCacheSchedulerSnapshot 表示调度快照缓存。
	OpsCacheSchedulerSnapshot OpsCacheName = "scheduler_snapshot"
)

// OpsCacheEvent 表示缓存的固定事件分类。
type OpsCacheEvent string

const (
	// OpsCacheEventUnknown 归并未识别的缓存事件。
	OpsCacheEventUnknown OpsCacheEvent = "unknown"
	// OpsCacheEventHit 表示缓存命中。
	OpsCacheEventHit OpsCacheEvent = "hit"
	// OpsCacheEventMiss 表示缓存未命中。
	OpsCacheEventMiss OpsCacheEvent = "miss"
	// OpsCacheEventInvalidate 表示缓存失效。
	OpsCacheEventInvalidate OpsCacheEvent = "invalidate"
	// OpsCacheEventWrite 表示缓存写入成功。
	OpsCacheEventWrite OpsCacheEvent = "write"
	// OpsCacheEventWriteDrop 表示缓存异步写入被丢弃。
	OpsCacheEventWriteDrop OpsCacheEvent = "write_drop"
	// OpsCacheEventWriteError 表示缓存写入已执行但失败或超时。
	OpsCacheEventWriteError OpsCacheEvent = "write_error"
)

// OpsConnectionPoolName 表示连接池的固定名称。
type OpsConnectionPoolName string

const (
	// OpsConnectionPoolUnknown 归并未识别的连接池名称。
	OpsConnectionPoolUnknown OpsConnectionPoolName = "unknown"
	// OpsConnectionPoolUpstreamHTTP 表示上游 HTTP 连接池。
	OpsConnectionPoolUpstreamHTTP OpsConnectionPoolName = "upstream_http"
	// OpsConnectionPoolRedis 表示 Redis 连接池。
	OpsConnectionPoolRedis OpsConnectionPoolName = "redis"
	// OpsConnectionPoolPostgres 表示 PostgreSQL 连接池。
	OpsConnectionPoolPostgres OpsConnectionPoolName = "postgres"
	// OpsConnectionPoolOpenAIWS 表示 OpenAI WebSocket 连接池。
	OpsConnectionPoolOpenAIWS OpsConnectionPoolName = "openai_ws"
)

// OpsConnectionPoolEvent 表示连接池的固定事件分类。
type OpsConnectionPoolEvent string

const (
	// OpsConnectionPoolEventUnknown 归并未识别的连接池事件。
	OpsConnectionPoolEventUnknown OpsConnectionPoolEvent = "unknown"
	// OpsConnectionPoolEventAcquire 表示获取连接。
	OpsConnectionPoolEventAcquire OpsConnectionPoolEvent = "acquire"
	// OpsConnectionPoolEventReuse 表示复用已有连接。
	OpsConnectionPoolEventReuse OpsConnectionPoolEvent = "reuse"
	// OpsConnectionPoolEventDial 表示新建连接。
	OpsConnectionPoolEventDial OpsConnectionPoolEvent = "dial"
	// OpsConnectionPoolEventEvict 表示驱逐空闲或过期连接。
	OpsConnectionPoolEventEvict OpsConnectionPoolEvent = "evict"
	// OpsConnectionPoolEventWaitTimeout 表示等待连接超时。
	OpsConnectionPoolEventWaitTimeout OpsConnectionPoolEvent = "wait_timeout"
	// OpsConnectionPoolEventError 表示连接池操作失败。
	OpsConnectionPoolEventError OpsConnectionPoolEvent = "error"
)

const (
	opsRequestStageDimension      = 7
	opsRequestResultDimension     = 4
	opsRequestProtocolDimension   = 4
	opsRequestErrorClassDimension = 9
	opsSchedulingEventDimension   = 6
	opsCacheNameDimension         = 6
	opsCacheEventDimension        = 7
	opsConnectionPoolDimension    = 5
	opsConnectionPoolEventDim     = 7
)

// OpsRequestStageMetricSnapshot 是一个固定维度请求阶段的进程内累计值。
type OpsRequestStageMetricSnapshot struct {
	// Stage 表示请求阶段。
	Stage OpsRequestStage `json:"stage"`
	// Result 表示阶段结果。
	Result OpsRequestResult `json:"result"`
	// Protocol 表示连接协议。
	Protocol OpsRequestProtocol `json:"protocol"`
	// ErrorClass 表示标准化的错误分类。
	ErrorClass OpsRequestErrorClass `json:"error_class"`
	// Count 表示该维度的样本数。
	Count uint64 `json:"count"`
	// TotalDurationMs 表示样本累计耗时，单位为毫秒。
	TotalDurationMs float64 `json:"total_duration_ms"`
	// MaxDurationMs 表示单样本最大耗时，单位为毫秒。
	MaxDurationMs float64 `json:"max_duration_ms"`
}

// OpsRuntimeCounterSnapshot 是调度、缓存或连接池的一个固定维度累计计数。
type OpsRuntimeCounterSnapshot struct {
	// Name 表示固定组件名称。
	Name string `json:"name"`
	// Event 表示固定事件分类。
	Event string `json:"event"`
	// Count 表示事件累计次数。
	Count uint64 `json:"count"`
}

// OpsConnectionPoolStatsSnapshot 是数据库或 Redis 客户端暴露的实时连接池统计。
// Gauge 字段表示采样瞬间状态，累计字段表示当前进程启动后的单调增量。
type OpsConnectionPoolStatsSnapshot struct {
	// Name 表示固定连接池名称。
	Name OpsConnectionPoolName `json:"name"`
	// Capacity 表示连接池配置的最大打开连接数；0 表示驱动未暴露或未限制。
	Capacity int `json:"capacity"`
	// BaseSize 表示 go-redis 配置的基础连接池大小；其他连接池为 0。
	BaseSize int `json:"base_size"`
	// Open 表示当前打开或已建立的连接总数。
	Open int `json:"open"`
	// InUse 表示当前正在使用的连接数。
	InUse int `json:"in_use"`
	// Idle 表示当前空闲连接数。
	Idle int `json:"idle"`
	// WaitCount 表示 PostgreSQL 因连接池容量受限而等待的累计次数。
	WaitCount uint64 `json:"wait_count"`
	// WaitDurationMs 表示 PostgreSQL 等待连接的累计时长，单位为毫秒。
	WaitDurationMs float64 `json:"wait_duration_ms"`
	// Hits 表示 Redis 从空闲池直接取得连接的累计次数。
	Hits uint64 `json:"hits"`
	// Misses 表示 Redis 未命中空闲连接并新建连接的累计次数。
	Misses uint64 `json:"misses"`
	// Timeouts 表示 Redis 等待连接超时的累计次数。
	Timeouts uint64 `json:"timeouts"`
	// ClosedIdle 表示 PostgreSQL 因超过最大空闲数关闭连接的累计次数。
	ClosedIdle uint64 `json:"closed_idle"`
	// ClosedIdleTime 表示 PostgreSQL 因超过最大空闲时间关闭连接的累计次数。
	ClosedIdleTime uint64 `json:"closed_idle_time"`
	// ClosedLifetime 表示 PostgreSQL 因超过最大生命周期关闭连接的累计次数。
	ClosedLifetime uint64 `json:"closed_lifetime"`
	// Stale 表示 Redis 从池中移除失效连接的累计次数。
	Stale uint64 `json:"stale"`
	// CacheHitTotal 表示上游 HTTP 客户端缓存命中的累计次数。
	CacheHitTotal uint64 `json:"cache_hit_total"`
	// CacheMissTotal 表示上游 HTTP 客户端缓存未命中的累计次数。
	CacheMissTotal uint64 `json:"cache_miss_total"`
	// CacheCreateTotal 表示上游 HTTP 客户端创建的累计次数。
	CacheCreateTotal uint64 `json:"cache_create_total"`
	// CacheEvictTotal 表示上游 HTTP 客户端淘汰的累计次数。
	CacheEvictTotal uint64 `json:"cache_evict_total"`
	// Entries 表示上游 HTTP 客户端缓存当前活动条目数。
	Entries int `json:"entries"`
	// InFlight 表示上游 HTTP 客户端活动与退休条目上的进行中请求数。
	InFlight int `json:"in_flight"`
	// OldestIdleAgeMs 表示上游 HTTP 客户端缓存最久空闲条目的空闲毫秒数。
	OldestIdleAgeMs float64 `json:"oldest_idle_age_ms"`
}

// OpsRuntimeMetricsSnapshot 是管理端读取的进程内实时指标快照。
type OpsRuntimeMetricsSnapshot struct {
	// GeneratedAt 表示快照生成时间。
	GeneratedAt time.Time `json:"generated_at"`
	// RequestStages 包含有样本的请求阶段时间与计数。
	RequestStages []OpsRequestStageMetricSnapshot `json:"request_stages"`
	// Scheduling 包含调度器固定事件计数。
	Scheduling []OpsRuntimeCounterSnapshot `json:"scheduling"`
	// Caches 包含缓存固定事件计数。
	Caches []OpsRuntimeCounterSnapshot `json:"caches"`
	// ConnectionPools 包含连接池固定事件计数。
	ConnectionPools []OpsRuntimeCounterSnapshot `json:"connection_pools"`
	// ConnectionPoolStats 包含 PostgreSQL 与 Redis 驱动的实时 gauge 和累计增量。
	ConnectionPoolStats []OpsConnectionPoolStatsSnapshot `json:"connection_pool_stats"`
}

type opsRuntimeRequestStageMetric struct {
	// count 保存阶段样本数。
	count atomic.Uint64
	// totalMicros 保存阶段累计耗时，单位为微秒。
	totalMicros atomic.Uint64
	// maxMicros 保存阶段单样本最大耗时，单位为微秒。
	maxMicros atomic.Uint64
}

// OpsRuntimeMetrics 保存固定维度的进程内实时指标，适合请求热路径调用。
type OpsRuntimeMetrics struct {
	// requestStages 按阶段、结果、协议和错误分类保存请求时间指标。
	requestStages [opsRequestStageDimension][opsRequestResultDimension][opsRequestProtocolDimension][opsRequestErrorClassDimension]opsRuntimeRequestStageMetric
	// scheduling 按调度事件保存累计次数。
	scheduling [opsSchedulingEventDimension]atomic.Uint64
	// caches 按缓存名称和事件保存累计次数。
	caches [opsCacheNameDimension][opsCacheEventDimension]atomic.Uint64
	// connectionPools 按连接池名称和事件保存累计次数。
	connectionPools [opsConnectionPoolDimension][opsConnectionPoolEventDim]atomic.Uint64
	// connectionPoolSourcesMu 保护只在依赖注入阶段更新的连接池统计源。
	connectionPoolSourcesMu sync.RWMutex
	// connectionPoolDB 提供 database/sql 原生连接池统计。
	connectionPoolDB *sql.DB
	// connectionPoolRedis 提供 go-redis 原生连接池统计。
	connectionPoolRedis *redis.Client
	// connectionPoolHTTP 提供上游 HTTP 客户端缓存统计。
	connectionPoolHTTP HTTPUpstreamPoolMetricsSource
}

var defaultOpsRuntimeMetrics = NewOpsRuntimeMetrics()

// NewOpsRuntimeMetrics 创建独立的运行时指标注册表，主要供隔离测试或专用实例使用。
func NewOpsRuntimeMetrics() *OpsRuntimeMetrics {
	return &OpsRuntimeMetrics{}
}

// DefaultOpsRuntimeMetrics 返回进程共享的运行时指标注册表。
func DefaultOpsRuntimeMetrics() *OpsRuntimeMetrics {
	return defaultOpsRuntimeMetrics
}

// RecordOpsRequestStage 向共享注册表记录一个请求阶段的耗时与计数。
func RecordOpsRequestStage(stage OpsRequestStage, duration time.Duration, labels OpsRequestMetricLabels) {
	defaultOpsRuntimeMetrics.RecordRequestStage(stage, duration, labels)
}

// RecordOpsSchedulingEvent 向共享注册表记录一个调度器事件。
func RecordOpsSchedulingEvent(event OpsSchedulingEvent) {
	defaultOpsRuntimeMetrics.RecordSchedulingEvent(event)
}

// RecordOpsCacheEvent 向共享注册表记录一个缓存事件。
func RecordOpsCacheEvent(name OpsCacheName, event OpsCacheEvent) {
	defaultOpsRuntimeMetrics.RecordCacheEvent(name, event)
}

// RecordOpsConnectionPoolEvent 向共享注册表记录一个连接池事件。
func RecordOpsConnectionPoolEvent(name OpsConnectionPoolName, event OpsConnectionPoolEvent) {
	defaultOpsRuntimeMetrics.RecordConnectionPoolEvent(name, event)
}

// SnapshotOpsRuntimeMetrics 返回共享注册表的当前快照。
func SnapshotOpsRuntimeMetrics() OpsRuntimeMetricsSnapshot {
	return defaultOpsRuntimeMetrics.Snapshot()
}

// RecordRequestStage 记录一个请求阶段的耗时与计数。未知标签会归并到固定 unknown 桶。
func (m *OpsRuntimeMetrics) RecordRequestStage(stage OpsRequestStage, duration time.Duration, labels OpsRequestMetricLabels) {
	if m == nil {
		return
	}
	stageIndex, _ := normalizeOpsRequestStage(stage)
	resultIndex, _ := normalizeOpsRequestResult(labels.Result)
	protocolIndex, _ := normalizeOpsRequestProtocol(labels.Protocol)
	errorClassIndex, _ := normalizeOpsRequestErrorClass(labels.ErrorClass)

	if duration < 0 {
		duration = 0
	}
	micros := uint64(duration.Microseconds())
	metric := &m.requestStages[stageIndex][resultIndex][protocolIndex][errorClassIndex]
	metric.count.Add(1)
	metric.totalMicros.Add(micros)
	updateOpsMetricMax(&metric.maxMicros, micros)
}

// RecordSchedulingEvent 记录一个固定分类的调度器事件。
func (m *OpsRuntimeMetrics) RecordSchedulingEvent(event OpsSchedulingEvent) {
	if m == nil {
		return
	}
	index, _ := normalizeOpsSchedulingEvent(event)
	m.scheduling[index].Add(1)
}

// RecordCacheEvent 记录一个固定分类的缓存事件。
func (m *OpsRuntimeMetrics) RecordCacheEvent(name OpsCacheName, event OpsCacheEvent) {
	if m == nil {
		return
	}
	nameIndex, _ := normalizeOpsCacheName(name)
	eventIndex, _ := normalizeOpsCacheEvent(event)
	m.caches[nameIndex][eventIndex].Add(1)
}

// RecordConnectionPoolEvent 记录一个固定分类的连接池事件。
func (m *OpsRuntimeMetrics) RecordConnectionPoolEvent(name OpsConnectionPoolName, event OpsConnectionPoolEvent) {
	if m == nil {
		return
	}
	nameIndex, _ := normalizeOpsConnectionPoolName(name)
	eventIndex, _ := normalizeOpsConnectionPoolEvent(event)
	m.connectionPools[nameIndex][eventIndex].Add(1)
}

// ConfigureConnectionPoolSources 注册只读统计源，供管理端快照按需采样。
// 采样只访问客户端进程内计数，不执行数据库或 Redis 命令。
func (m *OpsRuntimeMetrics) ConfigureConnectionPoolSources(db *sql.DB, redisClient *redis.Client) {
	if m == nil {
		return
	}
	m.connectionPoolSourcesMu.Lock()
	m.connectionPoolDB = db
	m.connectionPoolRedis = redisClient
	m.connectionPoolSourcesMu.Unlock()
}

// ConfigureHTTPUpstreamPoolSource 注册上游 HTTP 客户端缓存的只读统计源。
func (m *OpsRuntimeMetrics) ConfigureHTTPUpstreamPoolSource(source HTTPUpstreamPoolMetricsSource) {
	if m == nil {
		return
	}
	m.connectionPoolSourcesMu.Lock()
	m.connectionPoolHTTP = source
	m.connectionPoolSourcesMu.Unlock()
}

// Snapshot 返回有值的固定维度指标，便于管理端直接序列化。
func (m *OpsRuntimeMetrics) Snapshot() OpsRuntimeMetricsSnapshot {
	if m == nil {
		return OpsRuntimeMetricsSnapshot{}
	}

	snapshot := OpsRuntimeMetricsSnapshot{
		GeneratedAt:         time.Now().UTC(),
		RequestStages:       make([]OpsRequestStageMetricSnapshot, 0),
		Scheduling:          make([]OpsRuntimeCounterSnapshot, 0),
		Caches:              make([]OpsRuntimeCounterSnapshot, 0),
		ConnectionPools:     make([]OpsRuntimeCounterSnapshot, 0),
		ConnectionPoolStats: m.snapshotConnectionPoolStats(),
	}
	for stageIndex, stage := range opsRequestStages {
		for resultIndex, result := range opsRequestResults {
			for protocolIndex, protocol := range opsRequestProtocols {
				for errorClassIndex, errorClass := range opsRequestErrorClasses {
					metric := &m.requestStages[stageIndex][resultIndex][protocolIndex][errorClassIndex]
					count := metric.count.Load()
					if count == 0 {
						continue
					}
					snapshot.RequestStages = append(snapshot.RequestStages, OpsRequestStageMetricSnapshot{
						Stage:           stage,
						Result:          result,
						Protocol:        protocol,
						ErrorClass:      errorClass,
						Count:           count,
						TotalDurationMs: float64(metric.totalMicros.Load()) / 1000,
						MaxDurationMs:   float64(metric.maxMicros.Load()) / 1000,
					})
				}
			}
		}
	}

	for eventIndex, event := range opsSchedulingEvents {
		if count := m.scheduling[eventIndex].Load(); count > 0 {
			snapshot.Scheduling = append(snapshot.Scheduling, OpsRuntimeCounterSnapshot{
				Name:  "scheduler",
				Event: string(event),
				Count: count,
			})
		}
	}
	for nameIndex, name := range opsCacheNames {
		for eventIndex, event := range opsCacheEvents {
			if count := m.caches[nameIndex][eventIndex].Load(); count > 0 {
				snapshot.Caches = append(snapshot.Caches, OpsRuntimeCounterSnapshot{
					Name:  string(name),
					Event: string(event),
					Count: count,
				})
			}
		}
	}
	for nameIndex, name := range opsConnectionPoolNames {
		for eventIndex, event := range opsConnectionPoolEvents {
			if count := m.connectionPools[nameIndex][eventIndex].Load(); count > 0 {
				snapshot.ConnectionPools = append(snapshot.ConnectionPools, OpsRuntimeCounterSnapshot{
					Name:  string(name),
					Event: string(event),
					Count: count,
				})
			}
		}
	}

	return snapshot
}

// snapshotConnectionPoolStats 将标准库和 go-redis 的统计归一为固定低基数快照。
func (m *OpsRuntimeMetrics) snapshotConnectionPoolStats() []OpsConnectionPoolStatsSnapshot {
	m.connectionPoolSourcesMu.RLock()
	db := m.connectionPoolDB
	redisClient := m.connectionPoolRedis
	httpSource := m.connectionPoolHTTP
	m.connectionPoolSourcesMu.RUnlock()

	stats := make([]OpsConnectionPoolStatsSnapshot, 0, 3)
	if httpSource != nil {
		httpStats := httpSource.SnapshotHTTPUpstreamPoolMetrics()
		stats = append(stats, OpsConnectionPoolStatsSnapshot{
			Name:             OpsConnectionPoolUpstreamHTTP,
			Capacity:         int(httpStats.Capacity),
			CacheHitTotal:    uint64(httpStats.CacheHitTotal),
			CacheMissTotal:   uint64(httpStats.CacheMissTotal),
			CacheCreateTotal: uint64(httpStats.CacheCreateTotal),
			CacheEvictTotal:  uint64(httpStats.CacheEvictTotal),
			Entries:          int(httpStats.Entries),
			InFlight:         int(httpStats.InFlight),
			OldestIdleAgeMs:  float64(httpStats.OldestIdleAgeMs),
		})
	}
	if db != nil {
		dbStats := db.Stats()
		stats = append(stats, OpsConnectionPoolStatsSnapshot{
			Name:           OpsConnectionPoolPostgres,
			Capacity:       dbStats.MaxOpenConnections,
			Open:           dbStats.OpenConnections,
			InUse:          dbStats.InUse,
			Idle:           dbStats.Idle,
			WaitCount:      uint64(dbStats.WaitCount),
			WaitDurationMs: float64(dbStats.WaitDuration.Microseconds()) / 1000,
			ClosedIdle:     uint64(dbStats.MaxIdleClosed),
			ClosedIdleTime: uint64(dbStats.MaxIdleTimeClosed),
			ClosedLifetime: uint64(dbStats.MaxLifetimeClosed),
		})
	}
	if redisClient != nil {
		redisStats := redisClient.PoolStats()
		if redisStats != nil {
			capacity := 0
			baseSize := 0
			if options := redisClient.Options(); options != nil {
				capacity = options.MaxActiveConns
				baseSize = options.PoolSize
			}
			open := int(redisStats.TotalConns)
			idle := int(redisStats.IdleConns)
			inUse := open - idle
			if inUse < 0 {
				inUse = 0
			}
			stats = append(stats, OpsConnectionPoolStatsSnapshot{
				Name:     OpsConnectionPoolRedis,
				Capacity: capacity,
				BaseSize: baseSize,
				Open:     open,
				InUse:    inUse,
				Idle:     idle,
				Hits:     uint64(redisStats.Hits),
				Misses:   uint64(redisStats.Misses),
				Timeouts: uint64(redisStats.Timeouts),
				Stale:    uint64(redisStats.StaleConns),
			})
		}
	}
	return stats
}

func updateOpsMetricMax(target *atomic.Uint64, candidate uint64) {
	for current := target.Load(); candidate > current; current = target.Load() {
		if target.CompareAndSwap(current, candidate) {
			return
		}
	}
}

var (
	opsRequestStages = [...]OpsRequestStage{
		OpsRequestStageUnknown,
		OpsRequestStageSelection,
		OpsRequestStageSlotWait,
		OpsRequestStageConnect,
		OpsRequestStageHeaderWait,
		OpsRequestStageTTFT,
		OpsRequestStageStream,
	}
	opsRequestResults = [...]OpsRequestResult{
		OpsRequestResultUnknown,
		OpsRequestResultSuccess,
		OpsRequestResultFailure,
		OpsRequestResultCanceled,
	}
	opsRequestProtocols = [...]OpsRequestProtocol{
		OpsRequestProtocolUnknown,
		OpsRequestProtocolHTTP1,
		OpsRequestProtocolHTTP2,
		OpsRequestProtocolWebSocket,
	}
	opsRequestErrorClasses = [...]OpsRequestErrorClass{
		OpsRequestErrorClassUnknown,
		OpsRequestErrorClassNone,
		OpsRequestErrorClassTimeout,
		OpsRequestErrorClassConnection,
		OpsRequestErrorClassRateLimit,
		OpsRequestErrorClassUpstream4xx,
		OpsRequestErrorClassUpstream5xx,
		OpsRequestErrorClassCanceled,
		OpsRequestErrorClassInternal,
	}
	opsSchedulingEvents = [...]OpsSchedulingEvent{
		OpsSchedulingEventUnknown,
		OpsSchedulingEventSnapshotHit,
		OpsSchedulingEventSnapshotMiss,
		OpsSchedulingEventSlotWait,
		OpsSchedulingEventSlotWaitTimeout,
		OpsSchedulingEventFailover,
	}
	opsCacheNames = [...]OpsCacheName{
		OpsCacheUnknown,
		OpsCacheAPIKeyL1,
		OpsCacheAPIKeyL2,
		OpsCacheBilling,
		OpsCacheContextJournal,
		OpsCacheSchedulerSnapshot,
	}
	opsCacheEvents = [...]OpsCacheEvent{
		OpsCacheEventUnknown,
		OpsCacheEventHit,
		OpsCacheEventMiss,
		OpsCacheEventInvalidate,
		OpsCacheEventWrite,
		OpsCacheEventWriteDrop,
		OpsCacheEventWriteError,
	}
	opsConnectionPoolNames = [...]OpsConnectionPoolName{
		OpsConnectionPoolUnknown,
		OpsConnectionPoolUpstreamHTTP,
		OpsConnectionPoolRedis,
		OpsConnectionPoolPostgres,
		OpsConnectionPoolOpenAIWS,
	}
	opsConnectionPoolEvents = [...]OpsConnectionPoolEvent{
		OpsConnectionPoolEventUnknown,
		OpsConnectionPoolEventAcquire,
		OpsConnectionPoolEventReuse,
		OpsConnectionPoolEventDial,
		OpsConnectionPoolEventEvict,
		OpsConnectionPoolEventWaitTimeout,
		OpsConnectionPoolEventError,
	}
)

func normalizeOpsRequestStage(value OpsRequestStage) (int, OpsRequestStage) {
	for index, candidate := range opsRequestStages {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsRequestStageUnknown
}

func normalizeOpsRequestResult(value OpsRequestResult) (int, OpsRequestResult) {
	for index, candidate := range opsRequestResults {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsRequestResultUnknown
}

func normalizeOpsRequestProtocol(value OpsRequestProtocol) (int, OpsRequestProtocol) {
	for index, candidate := range opsRequestProtocols {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsRequestProtocolUnknown
}

func normalizeOpsRequestErrorClass(value OpsRequestErrorClass) (int, OpsRequestErrorClass) {
	if value == "" {
		return 1, OpsRequestErrorClassNone
	}
	for index, candidate := range opsRequestErrorClasses {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsRequestErrorClassUnknown
}

func normalizeOpsSchedulingEvent(value OpsSchedulingEvent) (int, OpsSchedulingEvent) {
	for index, candidate := range opsSchedulingEvents {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsSchedulingEventUnknown
}

func normalizeOpsCacheName(value OpsCacheName) (int, OpsCacheName) {
	for index, candidate := range opsCacheNames {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsCacheUnknown
}

func normalizeOpsCacheEvent(value OpsCacheEvent) (int, OpsCacheEvent) {
	for index, candidate := range opsCacheEvents {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsCacheEventUnknown
}

func normalizeOpsConnectionPoolName(value OpsConnectionPoolName) (int, OpsConnectionPoolName) {
	for index, candidate := range opsConnectionPoolNames {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsConnectionPoolUnknown
}

func normalizeOpsConnectionPoolEvent(value OpsConnectionPoolEvent) (int, OpsConnectionPoolEvent) {
	for index, candidate := range opsConnectionPoolEvents {
		if value == candidate {
			return index, candidate
		}
	}
	return 0, OpsConnectionPoolEventUnknown
}

type OpsMetricsCollector struct {
	opsRepo     OpsRepository
	settingRepo SettingRepository
	cfg         *config.Config

	accountRepo        AccountRepository
	concurrencyService *ConcurrencyService

	db          *sql.DB
	redisClient *redis.Client
	instanceID  string

	lastCgroupCPUUsageNanos uint64
	lastCgroupCPUSampleAt   time.Time

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once

	skipLogMu sync.Mutex
	skipLogAt time.Time
}

func NewOpsMetricsCollector(
	opsRepo OpsRepository,
	settingRepo SettingRepository,
	accountRepo AccountRepository,
	concurrencyService *ConcurrencyService,
	db *sql.DB,
	redisClient *redis.Client,
	httpUpstream HTTPUpstream,
	cfg *config.Config,
) *OpsMetricsCollector {
	runtimeMetrics := DefaultOpsRuntimeMetrics()
	runtimeMetrics.ConfigureConnectionPoolSources(db, redisClient)
	httpMetrics, _ := httpUpstream.(HTTPUpstreamPoolMetricsSource)
	runtimeMetrics.ConfigureHTTPUpstreamPoolSource(httpMetrics)
	return &OpsMetricsCollector{
		opsRepo:            opsRepo,
		settingRepo:        settingRepo,
		cfg:                cfg,
		accountRepo:        accountRepo,
		concurrencyService: concurrencyService,
		db:                 db,
		redisClient:        redisClient,
		instanceID:         uuid.NewString(),
	}
}

func (c *OpsMetricsCollector) Start() {
	if c == nil {
		return
	}
	c.startOnce.Do(func() {
		if c.stopCh == nil {
			c.stopCh = make(chan struct{})
		}
		go c.run()
	})
}

func (c *OpsMetricsCollector) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() {
		if c.stopCh != nil {
			close(c.stopCh)
		}
	})
}

func (c *OpsMetricsCollector) run() {
	// First run immediately so the dashboard has data soon after startup.
	c.collectOnce()

	for {
		interval := c.getInterval()
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			c.collectOnce()
		case <-c.stopCh:
			timer.Stop()
			return
		}
	}
}

func (c *OpsMetricsCollector) getInterval() time.Duration {
	interval := opsMetricsCollectorMinInterval

	if c.settingRepo == nil {
		return interval
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	raw, err := c.settingRepo.GetValue(ctx, SettingKeyOpsMetricsIntervalSeconds)
	if err != nil {
		return interval
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return interval
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return interval
	}
	if seconds < int(opsMetricsCollectorMinInterval.Seconds()) {
		seconds = int(opsMetricsCollectorMinInterval.Seconds())
	}
	if seconds > int(opsMetricsCollectorMaxInterval.Seconds()) {
		seconds = int(opsMetricsCollectorMaxInterval.Seconds())
	}
	return time.Duration(seconds) * time.Second
}

func (c *OpsMetricsCollector) collectOnce() {
	if c == nil {
		return
	}
	if c.cfg != nil && !c.cfg.Ops.Enabled {
		return
	}
	if c.opsRepo == nil {
		return
	}
	if c.db == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opsMetricsCollectorTimeout)
	defer cancel()

	if !c.isMonitoringEnabled(ctx) {
		return
	}

	release, ok := c.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	startedAt := time.Now().UTC()
	err := c.collectAndPersist(ctx)
	finishedAt := time.Now().UTC()

	durationMs := finishedAt.Sub(startedAt).Milliseconds()
	dur := durationMs
	runAt := startedAt

	if err != nil {
		msg := truncateString(err.Error(), 2048)
		errAt := finishedAt
		hbCtx, hbCancel := context.WithTimeout(context.Background(), opsMetricsCollectorHeartbeatTimeout)
		defer hbCancel()
		_ = c.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
			JobName:        opsMetricsCollectorJobName,
			LastRunAt:      &runAt,
			LastErrorAt:    &errAt,
			LastError:      &msg,
			LastDurationMs: &dur,
		})
		log.Printf("[OpsMetricsCollector] collect failed: %v", err)
		return
	}

	successAt := finishedAt
	hbCtx, hbCancel := context.WithTimeout(context.Background(), opsMetricsCollectorHeartbeatTimeout)
	defer hbCancel()
	_ = c.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsMetricsCollectorJobName,
		LastRunAt:      &runAt,
		LastSuccessAt:  &successAt,
		LastDurationMs: &dur,
	})
}

func (c *OpsMetricsCollector) isMonitoringEnabled(ctx context.Context) bool {
	if c == nil {
		return false
	}
	if c.cfg != nil && !c.cfg.Ops.Enabled {
		return false
	}
	if c.settingRepo == nil {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	value, err := c.settingRepo.GetValue(ctx, SettingKeyOpsMonitoringEnabled)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return true
		}
		// Fail-open: collector should not become a hard dependency.
		return true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "disabled":
		return false
	default:
		return true
	}
}

func (c *OpsMetricsCollector) collectAndPersist(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Align to stable minute boundaries to avoid partial buckets and to maximize cache hits.
	now := time.Now().UTC()
	windowEnd := now.Truncate(time.Minute)
	windowStart := windowEnd.Add(-1 * time.Minute)

	sys, err := c.collectSystemStats(ctx)
	if err != nil {
		// Continue; system stats are best-effort.
		log.Printf("[OpsMetricsCollector] system stats error: %v", err)
	}

	dbOK := c.checkDB(ctx)
	redisOK := c.checkRedis(ctx)
	active, idle := c.dbPoolStats()
	redisTotal, redisIdle, redisStatsOK := c.redisPoolStats()

	successCount, tokenConsumed, err := c.queryUsageCounts(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query usage counts: %w", err)
	}

	duration, ttft, err := c.queryUsageLatency(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query usage latency: %w", err)
	}

	errorTotal, businessLimited, errorSLA, upstreamExcl, upstream429, upstream529, err := c.queryErrorCounts(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query error counts: %w", err)
	}

	accountSwitchCount, err := c.queryAccountSwitchCount(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query account switch counts: %w", err)
	}

	windowSeconds := windowEnd.Sub(windowStart).Seconds()
	if windowSeconds <= 0 {
		windowSeconds = 60
	}
	requestTotal := successCount + errorTotal
	qps := float64(requestTotal) / windowSeconds
	tps := float64(tokenConsumed) / windowSeconds

	goroutines := runtime.NumGoroutine()
	concurrencyQueueDepth := c.collectConcurrencyQueueDepth(ctx)

	input := &OpsInsertSystemMetricsInput{
		CreatedAt:     windowEnd,
		WindowMinutes: 1,

		SuccessCount:         successCount,
		ErrorCountTotal:      errorTotal,
		BusinessLimitedCount: businessLimited,
		ErrorCountSLA:        errorSLA,

		UpstreamErrorCountExcl429529: upstreamExcl,
		Upstream429Count:             upstream429,
		Upstream529Count:             upstream529,

		TokenConsumed:      tokenConsumed,
		AccountSwitchCount: accountSwitchCount,
		QPS:                float64Ptr(roundTo1DP(qps)),
		TPS:                float64Ptr(roundTo1DP(tps)),

		DurationP50Ms: duration.p50,
		DurationP90Ms: duration.p90,
		DurationP95Ms: duration.p95,
		DurationP99Ms: duration.p99,
		DurationAvgMs: duration.avg,
		DurationMaxMs: duration.max,

		TTFTP50Ms: ttft.p50,
		TTFTP90Ms: ttft.p90,
		TTFTP95Ms: ttft.p95,
		TTFTP99Ms: ttft.p99,
		TTFTAvgMs: ttft.avg,
		TTFTMaxMs: ttft.max,

		CPUUsagePercent:    sys.cpuUsagePercent,
		MemoryUsedMB:       sys.memoryUsedMB,
		MemoryTotalMB:      sys.memoryTotalMB,
		MemoryUsagePercent: sys.memoryUsagePercent,

		DBOK:    boolPtr(dbOK),
		RedisOK: boolPtr(redisOK),

		RedisConnTotal: func() *int {
			if !redisStatsOK {
				return nil
			}
			return intPtr(redisTotal)
		}(),
		RedisConnIdle: func() *int {
			if !redisStatsOK {
				return nil
			}
			return intPtr(redisIdle)
		}(),

		DBConnActive:          intPtr(active),
		DBConnIdle:            intPtr(idle),
		GoroutineCount:        intPtr(goroutines),
		ConcurrencyQueueDepth: concurrencyQueueDepth,
	}

	return c.opsRepo.InsertSystemMetrics(ctx, input)
}

func (c *OpsMetricsCollector) collectConcurrencyQueueDepth(parentCtx context.Context) *int {
	if c == nil || c.accountRepo == nil || c.concurrencyService == nil {
		return nil
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	// Best-effort: never let concurrency sampling break the metrics collector.
	ctx, cancel := context.WithTimeout(parentCtx, 2*time.Second)
	defer cancel()

	accounts, err := c.accountRepo.ListSchedulable(ctx)
	if err != nil {
		return nil
	}
	if len(accounts) == 0 {
		zero := 0
		return &zero
	}

	batch := make([]AccountWithConcurrency, 0, len(accounts))
	for _, acc := range accounts {
		if acc.ID <= 0 {
			continue
		}
		batch = append(batch, AccountWithConcurrency{
			ID:             acc.ID,
			MaxConcurrency: acc.EffectiveLoadFactor(),
		})
	}
	if len(batch) == 0 {
		zero := 0
		return &zero
	}

	loadMap, err := c.concurrencyService.GetAccountsLoadBatch(ctx, batch)
	if err != nil {
		return nil
	}

	var total int64
	for _, info := range loadMap {
		if info == nil || info.WaitingCount <= 0 {
			continue
		}
		total += int64(info.WaitingCount)
	}
	if total < 0 {
		total = 0
	}

	maxInt := int64(^uint(0) >> 1)
	if total > maxInt {
		total = maxInt
	}
	v := int(total)
	return &v
}

type opsCollectedPercentiles struct {
	p50 *int
	p90 *int
	p95 *int
	p99 *int
	avg *float64
	max *int
}

func (c *OpsMetricsCollector) queryUsageCounts(ctx context.Context, start, end time.Time) (successCount int64, tokenConsumed int64, err error) {
	q := `
SELECT
  COALESCE(COUNT(*), 0) AS success_count,
  COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS token_consumed
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2`

	var tokens sql.NullInt64
	if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&successCount, &tokens); err != nil {
		return 0, 0, err
	}
	if tokens.Valid {
		tokenConsumed = tokens.Int64
	}
	return successCount, tokenConsumed, nil
}

func (c *OpsMetricsCollector) queryUsageLatency(ctx context.Context, start, end time.Time) (duration opsCollectedPercentiles, ttft opsCollectedPercentiles, err error) {
	{
		q := `
SELECT
  percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms) AS p50,
  percentile_cont(0.90) WITHIN GROUP (ORDER BY duration_ms) AS p90,
  percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95,
  percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms) AS p99,
  AVG(duration_ms) AS avg_ms,
  MAX(duration_ms) AS max_ms
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2
  AND duration_ms IS NOT NULL`

		var p50, p90, p95, p99 sql.NullFloat64
		var avg sql.NullFloat64
		var max sql.NullInt64
		if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&p50, &p90, &p95, &p99, &avg, &max); err != nil {
			return opsCollectedPercentiles{}, opsCollectedPercentiles{}, err
		}
		duration.p50 = floatToIntPtr(p50)
		duration.p90 = floatToIntPtr(p90)
		duration.p95 = floatToIntPtr(p95)
		duration.p99 = floatToIntPtr(p99)
		if avg.Valid {
			v := roundTo1DP(avg.Float64)
			duration.avg = &v
		}
		if max.Valid {
			v := int(max.Int64)
			duration.max = &v
		}
	}

	{
		q := `
SELECT
  percentile_cont(0.50) WITHIN GROUP (ORDER BY first_token_ms) AS p50,
  percentile_cont(0.90) WITHIN GROUP (ORDER BY first_token_ms) AS p90,
  percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_ms) AS p95,
  percentile_cont(0.99) WITHIN GROUP (ORDER BY first_token_ms) AS p99,
  AVG(first_token_ms) AS avg_ms,
  MAX(first_token_ms) AS max_ms
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2
  AND first_token_ms IS NOT NULL`

		var p50, p90, p95, p99 sql.NullFloat64
		var avg sql.NullFloat64
		var max sql.NullInt64
		if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&p50, &p90, &p95, &p99, &avg, &max); err != nil {
			return opsCollectedPercentiles{}, opsCollectedPercentiles{}, err
		}
		ttft.p50 = floatToIntPtr(p50)
		ttft.p90 = floatToIntPtr(p90)
		ttft.p95 = floatToIntPtr(p95)
		ttft.p99 = floatToIntPtr(p99)
		if avg.Valid {
			v := roundTo1DP(avg.Float64)
			ttft.avg = &v
		}
		if max.Valid {
			v := int(max.Int64)
			ttft.max = &v
		}
	}

	return duration, ttft, nil
}

func (c *OpsMetricsCollector) queryErrorCounts(ctx context.Context, start, end time.Time) (
	errorTotal int64,
	businessLimited int64,
	errorSLA int64,
	upstreamExcl429529 int64,
	upstream429 int64,
	upstream529 int64,
	err error,
) {
	q := `
SELECT
  COALESCE(COUNT(*) FILTER (WHERE COALESCE(status_code, 0) >= 400), 0) AS error_total,
  COALESCE(COUNT(*) FILTER (WHERE COALESCE(status_code, 0) >= 400 AND is_business_limited), 0) AS business_limited,
  COALESCE(COUNT(*) FILTER (WHERE COALESCE(status_code, 0) >= 400 AND NOT is_business_limited), 0) AS error_sla,
  COALESCE(COUNT(*) FILTER (WHERE error_owner = 'provider' AND NOT is_business_limited AND COALESCE(upstream_status_code, status_code, 0) NOT IN (429, 529)), 0) AS upstream_excl,
  COALESCE(COUNT(*) FILTER (WHERE error_owner = 'provider' AND NOT is_business_limited AND COALESCE(upstream_status_code, status_code, 0) = 429), 0) AS upstream_429,
  COALESCE(COUNT(*) FILTER (WHERE error_owner = 'provider' AND NOT is_business_limited AND COALESCE(upstream_status_code, status_code, 0) = 529), 0) AS upstream_529
FROM ops_error_logs
WHERE created_at >= $1 AND created_at < $2`

	if err := c.db.QueryRowContext(ctx, q, start, end).Scan(
		&errorTotal,
		&businessLimited,
		&errorSLA,
		&upstreamExcl429529,
		&upstream429,
		&upstream529,
	); err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	return errorTotal, businessLimited, errorSLA, upstreamExcl429529, upstream429, upstream529, nil
}

func (c *OpsMetricsCollector) queryAccountSwitchCount(ctx context.Context, start, end time.Time) (int64, error) {
	q := `
SELECT
  COALESCE(SUM(CASE
    WHEN split_part(ev->>'kind', ':', 1) IN ('failover', 'retry_exhausted_failover', 'failover_on_400') THEN 1
    ELSE 0
  END), 0) AS switch_count
FROM ops_error_logs o
CROSS JOIN LATERAL jsonb_array_elements(
  COALESCE(NULLIF(o.upstream_errors, 'null'::jsonb), '[]'::jsonb)
) AS ev
WHERE o.created_at >= $1 AND o.created_at < $2
  AND o.is_count_tokens = FALSE`

	var count int64
	if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

type opsCollectedSystemStats struct {
	cpuUsagePercent    *float64
	memoryUsedMB       *int64
	memoryTotalMB      *int64
	memoryUsagePercent *float64
}

func (c *OpsMetricsCollector) collectSystemStats(ctx context.Context) (*opsCollectedSystemStats, error) {
	out := &opsCollectedSystemStats{}
	if ctx == nil {
		ctx = context.Background()
	}

	sampleAt := time.Now().UTC()

	// Prefer cgroup (container) metrics when available.
	if cpuPct := c.tryCgroupCPUPercent(sampleAt); cpuPct != nil {
		out.cpuUsagePercent = cpuPct
	}

	cgroupUsed, cgroupTotal, cgroupOK := readCgroupMemoryBytes()
	if cgroupOK {
		usedMB := int64(cgroupUsed / bytesPerMB)
		out.memoryUsedMB = &usedMB
		if cgroupTotal > 0 {
			totalMB := int64(cgroupTotal / bytesPerMB)
			out.memoryTotalMB = &totalMB
			pct := roundTo1DP(float64(cgroupUsed) / float64(cgroupTotal) * 100)
			out.memoryUsagePercent = &pct
		}
	}

	// Fallback to host metrics if cgroup metrics are unavailable (or incomplete).
	if out.cpuUsagePercent == nil {
		if cpuPercents, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(cpuPercents) > 0 {
			v := roundTo1DP(cpuPercents[0])
			out.cpuUsagePercent = &v
		}
	}

	// If total memory isn't available from cgroup (e.g. memory.max = "max"), fill total from host.
	if out.memoryUsedMB == nil || out.memoryTotalMB == nil || out.memoryUsagePercent == nil {
		if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil && vm != nil {
			if out.memoryUsedMB == nil {
				usedMB := int64(vm.Used / bytesPerMB)
				out.memoryUsedMB = &usedMB
			}
			if out.memoryTotalMB == nil {
				totalMB := int64(vm.Total / bytesPerMB)
				out.memoryTotalMB = &totalMB
			}
			if out.memoryUsagePercent == nil {
				if out.memoryUsedMB != nil && out.memoryTotalMB != nil && *out.memoryTotalMB > 0 {
					pct := roundTo1DP(float64(*out.memoryUsedMB) / float64(*out.memoryTotalMB) * 100)
					out.memoryUsagePercent = &pct
				} else {
					pct := roundTo1DP(vm.UsedPercent)
					out.memoryUsagePercent = &pct
				}
			}
		}
	}

	return out, nil
}

func (c *OpsMetricsCollector) tryCgroupCPUPercent(now time.Time) *float64 {
	usageNanos, ok := readCgroupCPUUsageNanos()
	if !ok {
		return nil
	}

	// Initialize baseline sample.
	if c.lastCgroupCPUSampleAt.IsZero() {
		c.lastCgroupCPUUsageNanos = usageNanos
		c.lastCgroupCPUSampleAt = now
		return nil
	}

	elapsed := now.Sub(c.lastCgroupCPUSampleAt)
	if elapsed <= 0 {
		c.lastCgroupCPUUsageNanos = usageNanos
		c.lastCgroupCPUSampleAt = now
		return nil
	}

	prev := c.lastCgroupCPUUsageNanos
	c.lastCgroupCPUUsageNanos = usageNanos
	c.lastCgroupCPUSampleAt = now

	if usageNanos < prev {
		// Counter reset (container restarted).
		return nil
	}

	deltaUsageSec := float64(usageNanos-prev) / 1e9
	elapsedSec := elapsed.Seconds()
	if elapsedSec <= 0 {
		return nil
	}

	cores := readCgroupCPULimitCores()
	if cores <= 0 {
		// Can't reliably normalize; skip and fall back to gopsutil.
		return nil
	}

	pct := (deltaUsageSec / (elapsedSec * cores)) * 100
	if pct < 0 {
		pct = 0
	}
	// Clamp to avoid noise/jitter showing impossible values.
	if pct > 100 {
		pct = 100
	}
	v := roundTo1DP(pct)
	return &v
}

func readCgroupMemoryBytes() (usedBytes uint64, totalBytes uint64, ok bool) {
	// cgroup v2 (most common in modern containers)
	if used, ok1 := readUintFile("/sys/fs/cgroup/memory.current"); ok1 {
		usedBytes = used
		rawMax, err := os.ReadFile("/sys/fs/cgroup/memory.max")
		if err == nil {
			s := strings.TrimSpace(string(rawMax))
			if s != "" && s != "max" {
				if v, err := strconv.ParseUint(s, 10, 64); err == nil {
					totalBytes = v
				}
			}
		}
		return usedBytes, totalBytes, true
	}

	// cgroup v1 fallback
	if used, ok1 := readUintFile("/sys/fs/cgroup/memory/memory.usage_in_bytes"); ok1 {
		usedBytes = used
		if limit, ok2 := readUintFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok2 {
			// Some environments report a very large number when unlimited.
			if limit > 0 && limit < (1<<60) {
				totalBytes = limit
			}
		}
		return usedBytes, totalBytes, true
	}

	return 0, 0, false
}

func readCgroupCPUUsageNanos() (usageNanos uint64, ok bool) {
	// cgroup v2: cpu.stat has usage_usec
	if raw, err := os.ReadFile("/sys/fs/cgroup/cpu.stat"); err == nil {
		lines := strings.Split(string(raw), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			if fields[0] != "usage_usec" {
				continue
			}
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			return v * 1000, true
		}
	}

	// cgroup v1: cpuacct.usage is in nanoseconds
	if v, ok := readUintFile("/sys/fs/cgroup/cpuacct/cpuacct.usage"); ok {
		return v, true
	}

	return 0, false
}

func readCgroupCPULimitCores() float64 {
	// cgroup v2: cpu.max => "<quota> <period>" or "max <period>"
	if raw, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) >= 2 && fields[0] != "max" {
			quota, err1 := strconv.ParseFloat(fields[0], 64)
			period, err2 := strconv.ParseFloat(fields[1], 64)
			if err1 == nil && err2 == nil && quota > 0 && period > 0 {
				return quota / period
			}
		}
	}

	// cgroup v1: cpu.cfs_quota_us / cpu.cfs_period_us
	quota, okQuota := readIntFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	period, okPeriod := readIntFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if okQuota && okPeriod && quota > 0 && period > 0 {
		return float64(quota) / float64(period)
	}

	return 0
}

func readUintFile(path string) (uint64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func readIntFile(path string) (int64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (c *OpsMetricsCollector) checkDB(ctx context.Context) bool {
	if c == nil || c.db == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var one int
	if err := c.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return false
	}
	return one == 1
}

func (c *OpsMetricsCollector) checkRedis(ctx context.Context) bool {
	if c == nil || c.redisClient == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.redisClient.Ping(ctx).Err() == nil
}

func (c *OpsMetricsCollector) redisPoolStats() (total int, idle int, ok bool) {
	if c == nil || c.redisClient == nil {
		return 0, 0, false
	}
	stats := c.redisClient.PoolStats()
	if stats == nil {
		return 0, 0, false
	}
	return int(stats.TotalConns), int(stats.IdleConns), true
}

func (c *OpsMetricsCollector) dbPoolStats() (active int, idle int) {
	if c == nil || c.db == nil {
		return 0, 0
	}
	stats := c.db.Stats()
	return stats.InUse, stats.Idle
}

var opsMetricsCollectorReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (c *OpsMetricsCollector) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if c == nil || c.redisClient == nil {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ok, err := c.redisClient.SetNX(ctx, opsMetricsCollectorLeaderLockKey, c.instanceID, opsMetricsCollectorLeaderLockTTL).Result()
	if err != nil {
		// Prefer fail-closed to avoid stampeding the database when Redis is flaky.
		// Fallback to a DB advisory lock when Redis is present but unavailable.
		release, ok := tryAcquireDBAdvisoryLock(ctx, c.db, opsMetricsCollectorAdvisoryLockID)
		if !ok {
			c.maybeLogSkip()
			return nil, false
		}
		return release, true
	}
	if !ok {
		c.maybeLogSkip()
		return nil, false
	}

	release := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = opsMetricsCollectorReleaseScript.Run(ctx, c.redisClient, []string{opsMetricsCollectorLeaderLockKey}, c.instanceID).Result()
	}
	return release, true
}

func (c *OpsMetricsCollector) maybeLogSkip() {
	c.skipLogMu.Lock()
	defer c.skipLogMu.Unlock()

	now := time.Now()
	if !c.skipLogAt.IsZero() && now.Sub(c.skipLogAt) < time.Minute {
		return
	}
	c.skipLogAt = now
	log.Printf("[OpsMetricsCollector] leader lock held by another instance; skipping")
}

func floatToIntPtr(v sql.NullFloat64) *int {
	if !v.Valid {
		return nil
	}
	n := int(math.Round(v.Float64))
	return &n
}

func roundTo1DP(v float64) float64 {
	return math.Round(v*10) / 10
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func boolPtr(v bool) *bool {
	out := v
	return &out
}

func intPtr(v int) *int {
	out := v
	return &out
}

func float64Ptr(v float64) *float64 {
	out := v
	return &out
}
