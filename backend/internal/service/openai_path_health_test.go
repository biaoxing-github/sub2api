package service

import (
	"errors"
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

func TestOpenAIPathHealthRequiresFailuresWithinWindowBeforeCooldown(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		FailureWindow:            time.Minute,
		Cooldown:                 time.Minute,
		DegradedFailureThreshold: 2,
		OpenFailureThreshold:     3,
	})
	now := time.Unix(200, 0).UTC()
	tracker.now = func() time.Time { return now }
	key := OpenAIPathHealthKey{AccountID: 7}

	tracker.RecordFailure(key, "unexpected EOF", nil)
	snapshot := tracker.Snapshot(key)
	if snapshot.State != OpenAIPathHealthStateHealthy {
		t.Fatalf("state after one failure = %q, want healthy", snapshot.State)
	}
	if snapshot.WindowFailures != 1 {
		t.Fatalf("window failures after one failure = %d, want 1", snapshot.WindowFailures)
	}

	now = now.Add(2 * time.Minute)
	tracker.RecordFailure(key, "http2: timeout awaiting response headers", nil)
	snapshot = tracker.Snapshot(key)
	if snapshot.State != OpenAIPathHealthStateHealthy {
		t.Fatalf("state after old-window failure = %q, want healthy", snapshot.State)
	}
	if snapshot.WindowFailures != 1 {
		t.Fatalf("window failures after reset = %d, want 1", snapshot.WindowFailures)
	}

	now = now.Add(10 * time.Second)
	tracker.RecordFailure(key, "context deadline exceeded", nil)
	snapshot = tracker.Snapshot(key)
	if snapshot.State != OpenAIPathHealthStateDegraded {
		t.Fatalf("state after second in-window failure = %q, want degraded", snapshot.State)
	}

	now = now.Add(10 * time.Second)
	tracker.RecordFailure(key, "unexpected EOF", nil)
	snapshot = tracker.Snapshot(key)
	if snapshot.State != OpenAIPathHealthStateOpenCircuit {
		t.Fatalf("state after threshold = %q, want open_circuit", snapshot.State)
	}
	if snapshot.CooldownUntil == nil {
		t.Fatal("CooldownUntil = nil, want cooldown")
	}
}

func TestOpenAIPathHealthSuccessClearsFailureWindow(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		FailureWindow:            time.Minute,
		DegradedFailureThreshold: 2,
		OpenFailureThreshold:     3,
	})
	key := OpenAIPathHealthKey{AccountID: 8}

	tracker.RecordFailure(key, "unexpected EOF", nil)
	ttft := 700
	tracker.RecordSuccess(key, &ttft, nil)
	snapshot := tracker.Snapshot(key)
	if snapshot.WindowFailures != 0 || snapshot.ConsecutiveFailures != 0 {
		t.Fatalf("failure counters after success = window:%d consecutive:%d, want 0/0", snapshot.WindowFailures, snapshot.ConsecutiveFailures)
	}
	if snapshot.FailureWindowStarted != nil {
		t.Fatal("FailureWindowStarted after success is not nil")
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

func TestOpenAIPathHealth429DoesNotOpenCircuit(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		DegradedFailureThreshold: 1,
		OpenFailureThreshold:     1,
	})
	key := OpenAIPathHealthKey{AccountID: 9}

	tracker.RecordFailure(key, "429 rate limit", nil)
	snapshot := tracker.Snapshot(key)
	if snapshot.State != OpenAIPathHealthStateHealthy {
		t.Fatalf("state after 429 = %q, want healthy", snapshot.State)
	}
	if snapshot.Status429Count != 1 {
		t.Fatalf("Status429Count = %d, want 1", snapshot.Status429Count)
	}
	if snapshot.WindowFailures != 0 {
		t.Fatalf("WindowFailures = %d, want 0", snapshot.WindowFailures)
	}
}

func TestOpenAIPathHealthHTTP2Counters(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		DegradedFailureThreshold: 2,
		OpenFailureThreshold:     3,
	})
	key := OpenAIPathHealthKey{AccountID: 14, Upstream: "https://fast.example.com/v1"}

	tracker.RecordFailure(key, OpenAIPathFailureHTTP2HeaderTimeout, nil)
	tracker.RecordFailure(key, OpenAIPathFailureHTTP2ProtocolError, nil)
	tracker.RecordSignal(key, OpenAIPathSignalHTTP1FallbackHit, nil)
	snapshot := tracker.Snapshot(key)

	if snapshot.HTTP2HeaderTimeoutCount != 1 {
		t.Fatalf("HTTP2HeaderTimeoutCount = %d, want 1", snapshot.HTTP2HeaderTimeoutCount)
	}
	if snapshot.HTTP2ProtocolErrorCount != 1 {
		t.Fatalf("HTTP2ProtocolErrorCount = %d, want 1", snapshot.HTTP2ProtocolErrorCount)
	}
	if snapshot.HTTP1FallbackHitCount != 1 {
		t.Fatalf("HTTP1FallbackHitCount = %d, want 1", snapshot.HTTP1FallbackHitCount)
	}
	if snapshot.WindowFailures != 2 {
		t.Fatalf("WindowFailures = %d, want 2; fallback hit should not count as failure", snapshot.WindowFailures)
	}
}

func TestOpenAIPathHealthReasonForHTTP2Attempt(t *testing.T) {
	attempt := &HTTPUpstreamAttemptInfo{ProtocolMode: HTTPUpstreamProtocolModeOpenAIH2}

	headerReason := openAIPathHealthReasonForHTTPAttempt("header timeout", errors.New("http2: timeout awaiting response headers"), attempt)
	if headerReason != OpenAIPathFailureHTTP2HeaderTimeout {
		t.Fatalf("header reason = %q, want %q", headerReason, OpenAIPathFailureHTTP2HeaderTimeout)
	}

	protocolReason := openAIPathHealthReasonForHTTPAttempt("unexpected_eof", errors.New("stream error: stream ID 1; INTERNAL_ERROR"), attempt)
	if protocolReason != OpenAIPathFailureHTTP2ProtocolError {
		t.Fatalf("protocol reason = %q, want %q", protocolReason, OpenAIPathFailureHTTP2ProtocolError)
	}

	plainReason := openAIPathHealthReasonForHTTPAttempt("header timeout", errors.New("http2: timeout awaiting response headers"), &HTTPUpstreamAttemptInfo{ProtocolMode: HTTPUpstreamProtocolModeOpenAIH1})
	if plainReason != "header timeout" {
		t.Fatalf("plain reason = %q, want original reason", plainReason)
	}
}

func TestOpenAIPathHealthBusinessLimitsAreNotRecordedAsFailures(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		DegradedFailureThreshold: 1,
		OpenFailureThreshold:     1,
	})
	key := OpenAIPathHealthKey{AccountID: 13}

	tracker.RecordFailure(key, "No active subscription found for this group", nil)
	tracker.RecordFailure(key, "count_tokens is not enabled for this group", nil)
	tracker.RecordFailure(key, "model claude-opus is not in whitelist", nil)
	snapshot := tracker.Snapshot(key)

	if snapshot.FailureCount != 0 {
		t.Fatalf("FailureCount = %d, want 0", snapshot.FailureCount)
	}
	if snapshot.Samples != 0 {
		t.Fatalf("Samples = %d, want 0", snapshot.Samples)
	}
	if snapshot.State != OpenAIPathHealthStateHealthy {
		t.Fatalf("state = %q, want healthy", snapshot.State)
	}
}

func TestOpenAIPathHealthScheduleFailureReportDoesNotDoubleCount(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		DegradedFailureThreshold: 1,
		OpenFailureThreshold:     1,
	})
	account := &Account{ID: 11, Platform: PlatformOpenAI}
	key := OpenAIPathHealthKeyForAccount(account, string(OpenAIUpstreamTransportHTTPSSE))
	tracker.RecordFailure(key, OpenAIPathFailureHTTP429, nil)

	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: []Account{*account}},
		openaiPathHealth: tracker,
	}
	svc.ReportOpenAIAccountScheduleResult(account.ID, false, nil)

	snapshot := tracker.Snapshot(key)
	if snapshot.Status429Count != 1 {
		t.Fatalf("Status429Count = %d, want 1", snapshot.Status429Count)
	}
	if snapshot.FailureCount != 1 {
		t.Fatalf("FailureCount = %d, want no double count", snapshot.FailureCount)
	}
	if snapshot.WindowFailures != 0 {
		t.Fatalf("WindowFailures = %d, want 0", snapshot.WindowFailures)
	}
	if snapshot.State != OpenAIPathHealthStateHealthy {
		t.Fatalf("state = %q, want healthy", snapshot.State)
	}
}

func TestOpenAIPathHealthHalfOpenFailureReopensCircuit(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:               true,
		CircuitBreakerEnabled: true,
		FailureWindow:         time.Minute,
		Cooldown:              time.Second,
		OpenFailureThreshold:  1,
	})
	now := time.Unix(300, 0).UTC()
	tracker.now = func() time.Time { return now }
	key := OpenAIPathHealthKey{AccountID: 10}

	tracker.RecordFailure(key, "unexpected EOF", nil)
	now = now.Add(2 * time.Second)
	if got := tracker.Snapshot(key).State; got != OpenAIPathHealthStateHalfOpen {
		t.Fatalf("state after cooldown = %q, want half_open", got)
	}
	tracker.RecordFailure(key, "unexpected EOF", nil)
	if got := tracker.Snapshot(key).State; got != OpenAIPathHealthStateOpenCircuit {
		t.Fatalf("state after half-open failure = %q, want open_circuit", got)
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

func TestOpenAIPathHealthRecordFailureWithActionStoresLastActionLabel(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:               true,
		CircuitBreakerEnabled: true,
	})
	key := OpenAIPathHealthKey{AccountID: 101}

	tracker.RecordFailureWithAction(key, OpenAIPathFailureEOF, string(OpenAIStreamActionAvoidAccountTTL), nil)

	snapshot := tracker.Snapshot(key)
	if snapshot.LastActionLabel != string(OpenAIStreamActionAvoidAccountTTL) {
		t.Fatalf("LastActionLabel = %q, want %q", snapshot.LastActionLabel, OpenAIStreamActionAvoidAccountTTL)
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

	fastBoost, fastSample := tracker.ScoreBoost(fast, 1, 1, 0)
	slowBoost, slowSample := tracker.ScoreBoost(slow, 1, 1, 0)
	if !fastSample || !slowSample {
		t.Fatal("expected both paths to have samples")
	}
	if fastBoost <= slowBoost {
		t.Fatalf("fast boost %.3f <= slow boost %.3f", fastBoost, slowBoost)
	}
}

func TestOpenAIPathHealthScoreBoostUsesHeaderWait(t *testing.T) {
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{Enabled: true})
	fastHeader := OpenAIPathHealthKey{AccountID: 1}
	slowHeader := OpenAIPathHealthKey{AccountID: 2}
	fastWait := int64(400)
	slowWait := int64(6000)
	tracker.RecordSuccess(fastHeader, nil, &fastWait)
	tracker.RecordSuccess(slowHeader, nil, &slowWait)

	fastBoost, fastSample := tracker.ScoreBoost(fastHeader, 1, 0, 1)
	slowBoost, slowSample := tracker.ScoreBoost(slowHeader, 1, 0, 1)
	if !fastSample || !slowSample {
		t.Fatal("expected both paths to have header wait samples")
	}
	if fastBoost <= slowBoost {
		t.Fatalf("fast header boost %.3f <= slow header boost %.3f", fastBoost, slowBoost)
	}
}

func TestOpenAIPathHealthDefaultTransportMatchesHTTPSSE(t *testing.T) {
	key := OpenAIPathHealthKeyForAccount(&Account{ID: 1}, string(OpenAIUpstreamTransportAny))
	if key.Transport != string(OpenAIUpstreamTransportHTTPSSE) {
		t.Fatalf("transport = %q, want %q", key.Transport, OpenAIUpstreamTransportHTTPSSE)
	}
}

func TestOpenAIPathHealthKeyForAccountBaseURLUsesRequestURL(t *testing.T) {
	account := &Account{
		ID:       12,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://primary.example.com/v1",
		},
	}

	key := OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), " https://fast.example.com/v1/ ")

	if key.AccountID != 12 {
		t.Fatalf("AccountID = %d, want 12", key.AccountID)
	}
	if key.Upstream != "https://fast.example.com/v1/" {
		t.Fatalf("Upstream = %q", key.Upstream)
	}
}
