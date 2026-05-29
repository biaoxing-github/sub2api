package service

import (
	"strings"
	"sync"
	"time"
)

const (
	OpenAIPathHealthStateHealthy     = "healthy"
	OpenAIPathHealthStateDegraded    = "degraded"
	OpenAIPathHealthStateOpenCircuit = "open_circuit"
	OpenAIPathHealthStateHalfOpen    = "half_open"

	OpenAIPathFailureEOF           = "unexpected_eof"
	OpenAIPathFailureHeaderTimeout = "header_timeout"
	OpenAIPathFailureHTTP401       = "http_401"
	OpenAIPathFailureHTTP429       = "http_429"
	OpenAIPathFailureOther         = "other"
)

type OpenAIPathHealthKey struct {
	AccountID int64
	ProxyID   int64
	Upstream  string
	Transport string
}

type OpenAIPathHealthRecord struct {
	Key                  OpenAIPathHealthKey `json:"key"`
	State                string              `json:"state"`
	SuccessCount         int64               `json:"success_count"`
	FailureCount         int64               `json:"failure_count"`
	ConsecutiveFailures  int64               `json:"consecutive_failures"`
	WindowFailures       int64               `json:"window_failures"`
	FailureWindowStarted *time.Time          `json:"failure_window_started_at,omitempty"`
	EOFCount             int64               `json:"eof_count"`
	HeaderTimeoutCount   int64               `json:"header_timeout_count"`
	Status401Count       int64               `json:"status_401_count"`
	Status429Count       int64               `json:"status_429_count"`
	TTFTEWMAMs           float64             `json:"ttft_ewma_ms,omitempty"`
	HeaderWaitEWMAMs     float64             `json:"header_wait_ewma_ms,omitempty"`
	Samples              int64               `json:"samples"`
	LastFailureReason    string              `json:"last_failure_reason,omitempty"`
	LastFailureAt        *time.Time          `json:"last_failure_at,omitempty"`
	CooldownUntil        *time.Time          `json:"cooldown_until,omitempty"`
	ConsecutiveSuccesses int64               `json:"consecutive_successes"`
}

type OpenAIPathHealthOptions struct {
	Enabled                  bool
	CircuitBreakerEnabled    bool
	FailureWindow            time.Duration
	Cooldown                 time.Duration
	DegradedFailureThreshold int64
	OpenFailureThreshold     int64
	HalfOpenMaxProbes        int64
	EWMAAlpha                float64
}

type OpenAIPathHealthTracker struct {
	mu      sync.Mutex
	options OpenAIPathHealthOptions
	records map[OpenAIPathHealthKey]*OpenAIPathHealthRecord
	now     func() time.Time
}

func NewOpenAIPathHealthTracker(options OpenAIPathHealthOptions) *OpenAIPathHealthTracker {
	if options.Cooldown <= 0 {
		options.Cooldown = time.Minute
	}
	if options.FailureWindow <= 0 {
		options.FailureWindow = 2 * time.Minute
	}
	if options.DegradedFailureThreshold <= 0 {
		options.DegradedFailureThreshold = 2
	}
	if options.OpenFailureThreshold <= 0 {
		options.OpenFailureThreshold = 4
	}
	if options.HalfOpenMaxProbes <= 0 {
		options.HalfOpenMaxProbes = 2
	}
	if options.EWMAAlpha <= 0 || options.EWMAAlpha > 1 {
		options.EWMAAlpha = 0.2
	}
	return &OpenAIPathHealthTracker{
		options: options,
		records: make(map[OpenAIPathHealthKey]*OpenAIPathHealthRecord),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func OpenAIPathHealthKeyForAccount(account *Account, transport string) OpenAIPathHealthKey {
	key := OpenAIPathHealthKey{Transport: normalizeOpenAIPathHealthTransport(transport)}
	if account == nil {
		return key
	}
	key.AccountID = account.ID
	if account.ProxyID != nil {
		key.ProxyID = *account.ProxyID
	}
	key.Upstream = normalizeOpenAIPathHealthPart(account.GetOpenAIBaseURL())
	return key
}

func OpenAIPathHealthKeyForAccountBaseURL(account *Account, transport string, requestBaseURL string) OpenAIPathHealthKey {
	key := OpenAIPathHealthKeyForAccount(account, transport)
	if upstream := strings.TrimSpace(requestBaseURL); upstream != "" {
		key.Upstream = normalizeOpenAIPathHealthPart(upstream)
	}
	return key
}

func (t *OpenAIPathHealthTracker) Snapshot(key OpenAIPathHealthKey) OpenAIPathHealthRecord {
	key = normalizeOpenAIPathHealthKey(key)
	if t == nil {
		return OpenAIPathHealthRecord{Key: key, State: OpenAIPathHealthStateHealthy}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	record := t.ensureLocked(key)
	t.refreshStateLocked(record, t.now())
	return cloneOpenAIPathHealthRecord(record)
}

func (t *OpenAIPathHealthTracker) RecordSuccess(key OpenAIPathHealthKey, ttftMs *int, headerWaitMs *int64) {
	if t == nil || !t.options.Enabled {
		return
	}
	key = normalizeOpenAIPathHealthKey(key)
	t.mu.Lock()
	defer t.mu.Unlock()
	record := t.ensureLocked(key)
	now := t.now()
	t.refreshStateLocked(record, now)
	record.SuccessCount++
	record.Samples++
	record.ConsecutiveFailures = 0
	record.WindowFailures = 0
	record.FailureWindowStarted = nil
	record.ConsecutiveSuccesses++
	if ttftMs != nil && *ttftMs > 0 {
		record.TTFTEWMAMs = updateOpenAIPathEWMA(record.TTFTEWMAMs, float64(*ttftMs), t.options.EWMAAlpha)
	}
	if headerWaitMs != nil && *headerWaitMs > 0 {
		record.HeaderWaitEWMAMs = updateOpenAIPathEWMA(record.HeaderWaitEWMAMs, float64(*headerWaitMs), t.options.EWMAAlpha)
	}
	if record.State == OpenAIPathHealthStateHalfOpen && record.ConsecutiveSuccesses >= t.options.HalfOpenMaxProbes {
		record.State = OpenAIPathHealthStateHealthy
		record.CooldownUntil = nil
		record.LastFailureReason = ""
	}
}

func (t *OpenAIPathHealthTracker) RecordFailure(key OpenAIPathHealthKey, reason string, headerWaitMs *int64) {
	if t == nil || !t.options.Enabled {
		return
	}
	key = normalizeOpenAIPathHealthKey(key)
	reason = NormalizeOpenAIPathFailureReason(reason)
	t.mu.Lock()
	defer t.mu.Unlock()
	record := t.ensureLocked(key)
	now := t.now()
	t.refreshStateLocked(record, now)
	countsForCircuit := openAIPathFailureCountsForCircuit(reason)
	if countsForCircuit {
		t.prepareFailureWindowLocked(record, now)
	}
	record.FailureCount++
	record.Samples++
	if countsForCircuit {
		record.ConsecutiveFailures++
		record.WindowFailures++
	}
	record.ConsecutiveSuccesses = 0
	record.LastFailureReason = reason
	record.LastFailureAt = &now
	if headerWaitMs != nil && *headerWaitMs > 0 {
		record.HeaderWaitEWMAMs = updateOpenAIPathEWMA(record.HeaderWaitEWMAMs, float64(*headerWaitMs), t.options.EWMAAlpha)
	}
	switch reason {
	case OpenAIPathFailureEOF:
		record.EOFCount++
	case OpenAIPathFailureHeaderTimeout:
		record.HeaderTimeoutCount++
	case OpenAIPathFailureHTTP401:
		record.Status401Count++
	case OpenAIPathFailureHTTP429:
		record.Status429Count++
	}
	if !countsForCircuit {
		return
	}
	if !t.options.CircuitBreakerEnabled {
		if record.WindowFailures >= t.options.DegradedFailureThreshold {
			record.State = OpenAIPathHealthStateDegraded
		}
		return
	}
	if record.State == OpenAIPathHealthStateHalfOpen {
		record.State = OpenAIPathHealthStateOpenCircuit
		cooldownUntil := now.Add(t.options.Cooldown)
		record.CooldownUntil = &cooldownUntil
		return
	}
	if record.WindowFailures >= t.options.OpenFailureThreshold {
		record.State = OpenAIPathHealthStateOpenCircuit
		cooldownUntil := now.Add(t.options.Cooldown)
		record.CooldownUntil = &cooldownUntil
		return
	}
	if record.WindowFailures >= t.options.DegradedFailureThreshold {
		record.State = OpenAIPathHealthStateDegraded
	}
}

func (t *OpenAIPathHealthTracker) IsOpenCircuit(key OpenAIPathHealthKey) bool {
	if t == nil || !t.options.Enabled || !t.options.CircuitBreakerEnabled {
		return false
	}
	snapshot := t.Snapshot(key)
	return snapshot.State == OpenAIPathHealthStateOpenCircuit
}

func (t *OpenAIPathHealthTracker) ScoreBoost(key OpenAIPathHealthKey, minSamples int64, ttftWeight, headerWaitWeight float64) (boost float64, hasSample bool) {
	snapshot := t.Snapshot(key)
	if snapshot.State == OpenAIPathHealthStateOpenCircuit {
		return -1, snapshot.Samples >= minSamples
	}
	if snapshot.Samples < minSamples || (snapshot.TTFTEWMAMs <= 0 && snapshot.HeaderWaitEWMAMs <= 0) {
		return 0, false
	}
	if ttftWeight <= 0 && headerWaitWeight <= 0 {
		ttftWeight = 1
	}
	const targetMs = 1000.0
	const maxMs = 10000.0
	scoreSum := 0.0
	weightSum := 0.0
	if snapshot.TTFTEWMAMs > 0 && ttftWeight > 0 {
		scoreSum += ttftWeight * (1 - clamp01((snapshot.TTFTEWMAMs-targetMs)/(maxMs-targetMs)))
		weightSum += ttftWeight
	}
	if snapshot.HeaderWaitEWMAMs > 0 && headerWaitWeight > 0 {
		scoreSum += headerWaitWeight * (1 - clamp01((snapshot.HeaderWaitEWMAMs-targetMs)/(maxMs-targetMs)))
		weightSum += headerWaitWeight
	}
	if weightSum <= 0 {
		return 0, false
	}
	latencyScore := scoreSum / weightSum
	switch snapshot.State {
	case OpenAIPathHealthStateHealthy:
		return latencyScore, true
	case OpenAIPathHealthStateHalfOpen:
		return latencyScore * 0.6, true
	case OpenAIPathHealthStateDegraded:
		return latencyScore * 0.3, true
	default:
		return 0, true
	}
}

func (t *OpenAIPathHealthTracker) ensureLocked(key OpenAIPathHealthKey) *OpenAIPathHealthRecord {
	record := t.records[key]
	if record == nil {
		record = &OpenAIPathHealthRecord{Key: key, State: OpenAIPathHealthStateHealthy}
		t.records[key] = record
	}
	return record
}

func (t *OpenAIPathHealthTracker) refreshStateLocked(record *OpenAIPathHealthRecord, now time.Time) {
	if record == nil || record.State != OpenAIPathHealthStateOpenCircuit || record.CooldownUntil == nil {
		return
	}
	if !now.Before(*record.CooldownUntil) {
		record.State = OpenAIPathHealthStateHalfOpen
		record.CooldownUntil = nil
		record.ConsecutiveSuccesses = 0
	}
}

func (t *OpenAIPathHealthTracker) prepareFailureWindowLocked(record *OpenAIPathHealthRecord, now time.Time) {
	if record == nil {
		return
	}
	if record.FailureWindowStarted == nil || now.Sub(*record.FailureWindowStarted) > t.options.FailureWindow {
		record.WindowFailures = 0
		record.ConsecutiveFailures = 0
		started := now
		record.FailureWindowStarted = &started
	}
}

func openAIPathFailureCountsForCircuit(reason string) bool {
	return reason != OpenAIPathFailureHTTP429
}

func NormalizeOpenAIPathFailureReason(reason string) string {
	msg := strings.TrimSpace(reason)
	classification := ClassifyUpstreamError(UpstreamErrorInput{Message: msg})
	switch classification.Category {
	case UpstreamErrorCategoryUnexpectedEOF:
		return OpenAIPathFailureEOF
	case UpstreamErrorCategoryHeaderTimeout, UpstreamErrorCategoryTimeout:
		return OpenAIPathFailureHeaderTimeout
	case UpstreamErrorCategoryUnauthorized:
		return OpenAIPathFailureHTTP401
	case UpstreamErrorCategoryRateLimited, UpstreamErrorCategoryClientIPCircuitOpen:
		return OpenAIPathFailureHTTP429
	default:
		msg = strings.ToLower(msg)
		if msg == "" {
			return OpenAIPathFailureOther
		}
		return msg
	}
}

func normalizeOpenAIPathHealthKey(key OpenAIPathHealthKey) OpenAIPathHealthKey {
	key.Upstream = normalizeOpenAIPathHealthPart(key.Upstream)
	key.Transport = normalizeOpenAIPathHealthTransport(key.Transport)
	return key
}

func normalizeOpenAIPathHealthPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "default"
	}
	return value
}

func normalizeOpenAIPathHealthTransport(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return string(OpenAIUpstreamTransportHTTPSSE)
	}
	return value
}

func updateOpenAIPathEWMA(current, sample, alpha float64) float64 {
	if current <= 0 {
		return sample
	}
	return alpha*sample + (1-alpha)*current
}

func cloneOpenAIPathHealthRecord(record *OpenAIPathHealthRecord) OpenAIPathHealthRecord {
	if record == nil {
		return OpenAIPathHealthRecord{State: OpenAIPathHealthStateHealthy}
	}
	out := *record
	if record.LastFailureAt != nil {
		v := *record.LastFailureAt
		out.LastFailureAt = &v
	}
	if record.FailureWindowStarted != nil {
		v := *record.FailureWindowStarted
		out.FailureWindowStarted = &v
	}
	if record.CooldownUntil != nil {
		v := *record.CooldownUntil
		out.CooldownUntil = &v
	}
	return out
}
