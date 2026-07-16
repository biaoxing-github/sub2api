package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/redis/go-redis/v9"
)

func TestOpsRuntimeMetricsAggregatesFixedRequestDimensions(t *testing.T) {
	metrics := NewOpsRuntimeMetrics()

	metrics.RecordRequestStage(
		OpsRequestStageTTFT,
		12500*time.Microsecond,
		OpsRequestMetricLabels{
			Result:     OpsRequestResultSuccess,
			Protocol:   OpsRequestProtocolHTTP2,
			ErrorClass: OpsRequestErrorClassNone,
		},
	)
	metrics.RecordRequestStage(
		OpsRequestStageTTFT,
		7500*time.Microsecond,
		OpsRequestMetricLabels{
			Result:     OpsRequestResultSuccess,
			Protocol:   OpsRequestProtocolHTTP2,
			ErrorClass: OpsRequestErrorClassNone,
		},
	)

	// Dynamic values must be folded into the fixed unknown bucket.
	metrics.RecordRequestStage(
		OpsRequestStage("account-142"),
		time.Millisecond,
		OpsRequestMetricLabels{
			Result:     OpsRequestResult("provider-specific-result"),
			Protocol:   OpsRequestProtocol("h3"),
			ErrorClass: OpsRequestErrorClass("provider-message"),
		},
	)

	snapshot := metrics.Snapshot()
	metric, found := findOpsRequestStageMetric(
		snapshot,
		OpsRequestStageTTFT,
		OpsRequestResultSuccess,
		OpsRequestProtocolHTTP2,
		OpsRequestErrorClassNone,
	)
	if !found {
		t.Fatal("ttft metric was not included in the snapshot")
	}
	if metric.Count != 2 {
		t.Fatalf("ttft count = %d, want 2", metric.Count)
	}
	if metric.TotalDurationMs != 20 {
		t.Fatalf("ttft total duration = %vms, want 20ms", metric.TotalDurationMs)
	}
	if metric.MaxDurationMs != 12.5 {
		t.Fatalf("ttft max duration = %vms, want 12.5ms", metric.MaxDurationMs)
	}

	unknown, found := findOpsRequestStageMetric(
		snapshot,
		OpsRequestStageUnknown,
		OpsRequestResultUnknown,
		OpsRequestProtocolUnknown,
		OpsRequestErrorClassUnknown,
	)
	if !found {
		t.Fatal("invalid labels must be folded into an unknown metric")
	}
	if unknown.Count != 1 || unknown.TotalDurationMs != 1 {
		t.Fatalf("unknown metric = %+v, want one 1ms sample", unknown)
	}
}

func TestOpsRuntimeMetricsRecordsComponentCountersAndSupportsConcurrentWriters(t *testing.T) {
	metrics := NewOpsRuntimeMetrics()
	metrics.RecordSchedulingEvent(OpsSchedulingEventSnapshotHit)
	metrics.RecordSchedulingEvent(OpsSchedulingEvent("account-142"))
	metrics.RecordCacheEvent(OpsCacheAPIKeyL1, OpsCacheEventHit)
	metrics.RecordCacheEvent(OpsCacheName("tenant-42"), OpsCacheEvent("provider-error"))
	metrics.RecordConnectionPoolEvent(OpsConnectionPoolUpstreamHTTP, OpsConnectionPoolEventDial)
	metrics.RecordConnectionPoolEvent(OpsConnectionPoolName("provider-142"), OpsConnectionPoolEvent("queue-full"))

	const writers = 32
	const samplesPerWriter = 50
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range samplesPerWriter {
				metrics.RecordRequestStage(
					OpsRequestStageSelection,
					time.Millisecond,
					OpsRequestMetricLabels{Result: OpsRequestResultSuccess},
				)
			}
		}()
	}
	wg.Wait()

	snapshot := metrics.Snapshot()
	selection, found := findOpsRequestStageMetric(
		snapshot,
		OpsRequestStageSelection,
		OpsRequestResultSuccess,
		OpsRequestProtocolUnknown,
		OpsRequestErrorClassNone,
	)
	if !found {
		t.Fatal("concurrently recorded selection metric was not included in the snapshot")
	}
	if selection.Count != writers*samplesPerWriter {
		t.Fatalf("selection count = %d, want %d", selection.Count, writers*samplesPerWriter)
	}
	if selection.TotalDurationMs != writers*samplesPerWriter {
		t.Fatalf("selection total duration = %vms, want %dms", selection.TotalDurationMs, writers*samplesPerWriter)
	}

	assertOpsRuntimeCounter(t, snapshot.Scheduling, "scheduler", string(OpsSchedulingEventSnapshotHit), 1)
	assertOpsRuntimeCounter(t, snapshot.Scheduling, "scheduler", "unknown", 1)
	assertOpsRuntimeCounter(t, snapshot.Caches, string(OpsCacheAPIKeyL1), string(OpsCacheEventHit), 1)
	assertOpsRuntimeCounter(t, snapshot.Caches, "unknown", "unknown", 1)
	assertOpsRuntimeCounter(t, snapshot.ConnectionPools, string(OpsConnectionPoolUpstreamHTTP), string(OpsConnectionPoolEventDial), 1)
	assertOpsRuntimeCounter(t, snapshot.ConnectionPools, "unknown", "unknown", 1)
}

func TestOpsRuntimeMetricsSnapshotsStandardConnectionPoolStats(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(7)
	db.SetMaxIdleConns(3)
	mock.ExpectPing()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping sql mock: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  time.Millisecond,
		ReadTimeout:  time.Millisecond,
		WriteTimeout: time.Millisecond,
		PoolTimeout:  time.Millisecond,
	})
	t.Cleanup(func() { _ = redisClient.Close() })

	metrics := NewOpsRuntimeMetrics()
	metrics.ConfigureConnectionPoolSources(db, redisClient)
	snapshot := metrics.Snapshot()
	if len(snapshot.ConnectionPoolStats) != 2 {
		t.Fatalf("connection pool stats length = %d, want 2", len(snapshot.ConnectionPoolStats))
	}

	dbWant := db.Stats()
	dbGot := findOpsConnectionPoolStats(snapshot, OpsConnectionPoolPostgres)
	if dbGot == nil {
		t.Fatal("postgres connection pool stats were not included")
	}
	if dbGot.Capacity != dbWant.MaxOpenConnections ||
		dbGot.Open != dbWant.OpenConnections ||
		dbGot.InUse != dbWant.InUse ||
		dbGot.Idle != dbWant.Idle ||
		dbGot.WaitCount != uint64(dbWant.WaitCount) ||
		dbGot.WaitDurationMs != float64(dbWant.WaitDuration.Microseconds())/1000 {
		t.Fatalf("postgres connection pool stats = %+v, want %+v", *dbGot, dbWant)
	}

	redisWant := redisClient.PoolStats()
	redisGot := findOpsConnectionPoolStats(snapshot, OpsConnectionPoolRedis)
	if redisGot == nil {
		t.Fatal("redis connection pool stats were not included")
	}
	if redisGot.Open != int(redisWant.TotalConns) ||
		redisGot.Capacity != redisClient.Options().MaxActiveConns ||
		redisGot.BaseSize != redisClient.Options().PoolSize ||
		redisGot.Idle != int(redisWant.IdleConns) ||
		redisGot.Hits != uint64(redisWant.Hits) ||
		redisGot.Misses != uint64(redisWant.Misses) ||
		redisGot.Timeouts != uint64(redisWant.Timeouts) ||
		redisGot.Stale != uint64(redisWant.StaleConns) {
		t.Fatalf("redis connection pool stats = %+v, want %+v", *redisGot, *redisWant)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql mock expectations: %v", err)
	}
}

func TestOpsRuntimeMetricsOmitsUnconfiguredConnectionPoolStats(t *testing.T) {
	snapshot := NewOpsRuntimeMetrics().Snapshot()
	if snapshot.ConnectionPoolStats == nil {
		t.Fatal("connection pool stats must serialize as an empty array")
	}
	if len(snapshot.ConnectionPoolStats) != 0 {
		t.Fatalf("connection pool stats length = %d, want 0", len(snapshot.ConnectionPoolStats))
	}
}

func TestOpsRuntimeMetricsSnapshotsHTTPUpstreamPoolStats(t *testing.T) {
	metrics := NewOpsRuntimeMetrics()
	metrics.ConfigureHTTPUpstreamPoolSource(opsRuntimeHTTPPoolMetricsSource{
		snapshot: HTTPUpstreamPoolMetricsSnapshot{
			CacheHitTotal:    11,
			CacheMissTotal:   3,
			CacheCreateTotal: 2,
			CacheEvictTotal:  1,
			Entries:          7,
			Capacity:         50,
			InFlight:         4,
			OldestIdleAgeMs:  1250,
		},
	})

	snapshot := metrics.Snapshot()
	got := findOpsConnectionPoolStats(snapshot, OpsConnectionPoolUpstreamHTTP)
	if got == nil {
		t.Fatal("upstream HTTP pool stats were not included")
	}
	if got.Capacity != 50 ||
		got.CacheHitTotal != 11 ||
		got.CacheMissTotal != 3 ||
		got.CacheCreateTotal != 2 ||
		got.CacheEvictTotal != 1 ||
		got.Entries != 7 ||
		got.InFlight != 4 ||
		got.OldestIdleAgeMs != 1250 {
		t.Fatalf("upstream HTTP pool stats = %+v", *got)
	}
}

type opsRuntimeHTTPPoolMetricsSource struct {
	snapshot HTTPUpstreamPoolMetricsSnapshot
}

func (s opsRuntimeHTTPPoolMetricsSource) SnapshotHTTPUpstreamPoolMetrics() HTTPUpstreamPoolMetricsSnapshot {
	return s.snapshot
}

func findOpsConnectionPoolStats(snapshot OpsRuntimeMetricsSnapshot, name OpsConnectionPoolName) *OpsConnectionPoolStatsSnapshot {
	for index := range snapshot.ConnectionPoolStats {
		if snapshot.ConnectionPoolStats[index].Name == name {
			return &snapshot.ConnectionPoolStats[index]
		}
	}
	return nil
}

func findOpsRequestStageMetric(
	snapshot OpsRuntimeMetricsSnapshot,
	stage OpsRequestStage,
	result OpsRequestResult,
	protocol OpsRequestProtocol,
	errorClass OpsRequestErrorClass,
) (OpsRequestStageMetricSnapshot, bool) {
	for _, metric := range snapshot.RequestStages {
		if metric.Stage == stage &&
			metric.Result == result &&
			metric.Protocol == protocol &&
			metric.ErrorClass == errorClass {
			return metric, true
		}
	}
	return OpsRequestStageMetricSnapshot{}, false
}

func assertOpsRuntimeCounter(
	t *testing.T,
	counters []OpsRuntimeCounterSnapshot,
	name string,
	event string,
	want uint64,
) {
	t.Helper()
	for _, counter := range counters {
		if counter.Name == name && counter.Event == event {
			if counter.Count != want {
				t.Fatalf("counter %s/%s = %d, want %d", name, event, counter.Count, want)
			}
			return
		}
	}
	t.Fatalf("counter %s/%s was not included in the snapshot", name, event)
}
