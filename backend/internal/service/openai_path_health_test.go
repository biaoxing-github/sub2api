package service

import (
	"sync"
	"testing"
	"time"
)

func TestOpenAIPathHealthCircuitBreakerTransitions(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:               true,
		CircuitBreakerEnabled: true,
		Cooldown:              time.Second,
		OpenFailureThreshold:  2,
	})
	key := OpenAIPathHealthKey{AccountID: 1, ProxyID: 2, Upstream: "https://api.openai.com", Transport: "http_sse"}

	tracker.RecordFailure(key, "unexpected EOF", nil)
	if got := tracker.Snapshot(key).State; got != OpenAIPathHealthStateHealthy && got != OpenAIPathHealthStateDegraded {
		t.Fatalf("state after first failure = %q", got)
	}
	tracker.RecordFailure(key, "http2: timeout awaiting response headers", nil)
	snapshot := tracker.Snapshot(key)
	if snapshot.State != OpenAIPathHealthStateOpenCircuit {
		t.Fatalf("state = %q, want open_circuit", snapshot.State)
	}
	if !tracker.IsOpenCircuit(key) {
		t.Fatal("IsOpenCircuit = false, want true")
	}
	if snapshot.EOFCount != 1 || snapshot.HeaderTimeoutCount != 1 {
		t.Fatalf("counts = eof:%d header:%d", snapshot.EOFCount, snapshot.HeaderTimeoutCount)
	}
}

func TestOpenAIPathHealthHalfOpenAfterCooldown(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:               true,
		CircuitBreakerEnabled: true,
		Cooldown:              time.Second,
		OpenFailureThreshold:  1,
		HalfOpenMaxProbes:     1,
	})
	now := time.Unix(100, 0).UTC()
	tracker.now = func() time.Time { return now }
	key := OpenAIPathHealthKey{AccountID: 1}

	tracker.RecordFailure(key, "401 unauthorized", nil)
	if got := tracker.Snapshot(key).State; got != OpenAIPathHealthStateOpenCircuit {
		t.Fatalf("state = %q, want open_circuit", got)
	}
	now = now.Add(2 * time.Second)
	if got := tracker.Snapshot(key).State; got != OpenAIPathHealthStateHalfOpen {
		t.Fatalf("state = %q, want half_open", got)
	}
	ttft := 800
	tracker.RecordSuccess(key, &ttft, nil)
	if got := tracker.Snapshot(key).State; got != OpenAIPathHealthStateHealthy {
		t.Fatalf("state = %q, want healthy", got)
	}
}

func TestOpenAIPathHealthConcurrentRecord(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled: true,
	})
	key := OpenAIPathHealthKey{AccountID: 99}
	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				ttft := 100 + i
				tracker.RecordSuccess(key, &ttft, nil)
			} else {
				tracker.RecordFailure(key, "429 rate limit", nil)
			}
		}(i)
	}
	wg.Wait()
	snapshot := tracker.Snapshot(key)
	if snapshot.SuccessCount != workers/2 || snapshot.FailureCount != workers/2 {
		t.Fatalf("counts = success:%d failure:%d", snapshot.SuccessCount, snapshot.FailureCount)
	}
	if snapshot.Status429Count != workers/2 {
		t.Fatalf("Status429Count = %d, want %d", snapshot.Status429Count, workers/2)
	}
}

func TestOpenAIPathHealthScoreBoostUsesTTFT(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{Enabled: true})
	fast := OpenAIPathHealthKey{AccountID: 1}
	slow := OpenAIPathHealthKey{AccountID: 2}
	fastTTFT := 500
	slowTTFT := 5000
	tracker.RecordSuccess(fast, &fastTTFT, nil)
	tracker.RecordSuccess(slow, &slowTTFT, nil)

	fastBoost, fastSample := tracker.ScoreBoost(fast, 1)
	slowBoost, slowSample := tracker.ScoreBoost(slow, 1)
	if !fastSample || !slowSample {
		t.Fatal("expected both paths to have samples")
	}
	if fastBoost <= slowBoost {
		t.Fatalf("fast boost %.3f <= slow boost %.3f", fastBoost, slowBoost)
	}
}

func TestOpenAIPathHealthDefaultTransportMatchesHTTPSSE(t *testing.T) {
	key := OpenAIPathHealthKeyForAccount(&Account{ID: 1}, string(OpenAIUpstreamTransportAny))
	if key.Transport != string(OpenAIUpstreamTransportHTTPSSE) {
		t.Fatalf("transport = %q, want %q", key.Transport, OpenAIUpstreamTransportHTTPSSE)
	}
}
