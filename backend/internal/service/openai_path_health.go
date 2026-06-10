package service

import (
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	OpenAIPathHealthStateHealthy     = "healthy"
	OpenAIPathHealthStateDegraded    = "degraded"
	OpenAIPathHealthStateOpenCircuit = "open_circuit"
	OpenAIPathHealthStateHalfOpen    = "half_open"

	OpenAIPathFailureEOF                = "unexpected_eof"
	OpenAIPathFailureHeaderTimeout      = "header_timeout"
	OpenAIPathFailureHTTP2HeaderTimeout = "http2_header_timeout"
	OpenAIPathFailureHTTP2ProtocolError = "http2_protocol_error"
	OpenAIPathSignalHTTP1FallbackHit    = "http1_fallback_hit"
	OpenAIPathFailureHTTP401            = "http_401"
	OpenAIPathFailureHTTP429            = "http_429"
	OpenAIPathFailureOther              = "other"
)

type OpenAIPathHealthKey struct {
	AccountID int64
	ProxyID   int64
	Upstream  string
	Transport string
}

type OpenAIPathHealthRecord struct {
	Key                     OpenAIPathHealthKey `json:"key"`
	State                   string              `json:"state"`
	SuccessCount            int64               `json:"success_count"`
	FailureCount            int64               `json:"failure_count"`
	ConsecutiveFailures     int64               `json:"consecutive_failures"`
	WindowFailures          int64               `json:"window_failures"`
	FailureWindowStarted    *time.Time          `json:"failure_window_started_at,omitempty"`
	EOFCount                int64               `json:"eof_count"`
	HeaderTimeoutCount      int64               `json:"header_timeout_count"`
	HTTP2HeaderTimeoutCount int64               `json:"http2_header_timeout_count"`
	HTTP2ProtocolErrorCount int64               `json:"http2_protocol_error_count"`
	HTTP1FallbackHitCount   int64               `json:"http1_fallback_hit_count"`
	Status401Count          int64               `json:"status_401_count"`
	Status429Count          int64               `json:"status_429_count"`
	TTFTEWMAMs              float64             `json:"ttft_ewma_ms,omitempty"`
	HeaderWaitEWMAMs        float64             `json:"header_wait_ewma_ms,omitempty"`
	Samples                 int64               `json:"samples"`
	LastFailureReason       string              `json:"last_failure_reason,omitempty"`
	LastActionLabel         string              `json:"last_action_label,omitempty"`
	LastFailureAt           *time.Time          `json:"last_failure_at,omitempty"`
	CooldownUntil           *time.Time          `json:"cooldown_until,omitempty"`
	ConsecutiveSuccesses    int64               `json:"consecutive_successes"`
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

func newOpenAIPathHealthTrackerFromConfig(cfg *config.Config) *OpenAIPathHealthTracker {
	pathHealth := config.GatewayOpenAIPathHealthConfig{
		Enabled:               true,
		CircuitBreakerEnabled: true,
		FailureWindowSeconds:  120,
		CooldownSeconds:       60,
		DegradedFailures:      2,
		OpenFailures:          4,
		HalfOpenMaxProbes:     2,
	}
	if cfg != nil {
		pathHealth = cfg.Gateway.OpenAIPathHealth
	}
	return NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  pathHealth.Enabled,
		CircuitBreakerEnabled:    pathHealth.CircuitBreakerEnabled,
		FailureWindow:            time.Duration(pathHealth.FailureWindowSeconds) * time.Second,
		Cooldown:                 time.Duration(pathHealth.CooldownSeconds) * time.Second,
		DegradedFailureThreshold: int64(pathHealth.DegradedFailures),
		OpenFailureThreshold:     int64(pathHealth.OpenFailures),
		HalfOpenMaxProbes:        int64(pathHealth.HalfOpenMaxProbes),
	})
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

// OpenAIPathHealthBucketKeyForAccount 构造同代理、同上游、同传输的聚合健康桶 key。
// 聚合桶不带 AccountID，便于一个账号触发的路径熔断影响同路径其他账号。
func OpenAIPathHealthBucketKeyForAccount(account *Account, transport string) OpenAIPathHealthKey {
	key := OpenAIPathHealthKeyForAccount(account, transport)
	key.AccountID = 0
	return key
}

// OpenAIPathHealthBucketKeyForAccountBaseURL 构造指定 request_base_url 的聚合健康桶 key。
func OpenAIPathHealthBucketKeyForAccountBaseURL(account *Account, transport string, requestBaseURL string) OpenAIPathHealthKey {
	key := OpenAIPathHealthKeyForAccountBaseURL(account, transport, requestBaseURL)
	key.AccountID = 0
	return key
}

func (s *OpenAIGatewayService) SnapshotOpenAIPathHealthForAccount(account *Account, transport OpenAIUpstreamTransport) (OpenAIPathHealthRecord, bool) {
	if s == nil || s.openaiPathHealth == nil || account == nil {
		return OpenAIPathHealthRecord{}, false
	}
	key := OpenAIPathHealthKeyForAccount(account, string(transport))
	return s.openaiPathHealth.Snapshot(key), true
}

func (s *OpenAIGatewayService) OpenAIPathHealthTracker() *OpenAIPathHealthTracker {
	if s == nil {
		return nil
	}
	return s.openaiPathHealth
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
	t.RecordFailureWithAction(key, reason, "", headerWaitMs)
}

func (t *OpenAIPathHealthTracker) RecordFailureWithAction(key OpenAIPathHealthKey, reason string, actionLabel string, headerWaitMs *int64) {
	if t == nil || !t.options.Enabled {
		return
	}
	key = normalizeOpenAIPathHealthKey(key)
	reason = NormalizeOpenAIPathFailureReason(reason)
	if reason == "" {
		return
	}
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
	record.LastActionLabel = strings.TrimSpace(actionLabel)
	record.LastFailureAt = &now
	if headerWaitMs != nil && *headerWaitMs > 0 {
		record.HeaderWaitEWMAMs = updateOpenAIPathEWMA(record.HeaderWaitEWMAMs, float64(*headerWaitMs), t.options.EWMAAlpha)
	}
	switch reason {
	case OpenAIPathFailureEOF:
		record.EOFCount++
	case OpenAIPathFailureHeaderTimeout:
		record.HeaderTimeoutCount++
	case OpenAIPathFailureHTTP2HeaderTimeout:
		record.HTTP2HeaderTimeoutCount++
	case OpenAIPathFailureHTTP2ProtocolError:
		record.HTTP2ProtocolErrorCount++
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
		cooldownUntil := now.Add(t.getCooldownDuration(reason))
		record.CooldownUntil = &cooldownUntil
		return
	}
	if record.WindowFailures >= t.options.OpenFailureThreshold {
		record.State = OpenAIPathHealthStateOpenCircuit
		cooldownUntil := now.Add(t.getCooldownDuration(reason))
		record.CooldownUntil = &cooldownUntil
		return
	}
	if record.WindowFailures >= t.options.DegradedFailureThreshold {
		record.State = OpenAIPathHealthStateDegraded
	}
}

func (t *OpenAIPathHealthTracker) RecordSignal(key OpenAIPathHealthKey, signal string, headerWaitMs *int64) {
	if t == nil || !t.options.Enabled {
		return
	}
	key = normalizeOpenAIPathHealthKey(key)
	signal = strings.ToLower(strings.TrimSpace(signal))
	t.mu.Lock()
	defer t.mu.Unlock()
	record := t.ensureLocked(key)
	t.refreshStateLocked(record, t.now())
	switch signal {
	case OpenAIPathSignalHTTP1FallbackHit:
		record.HTTP1FallbackHitCount++
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
	switch reason {
	case OpenAIPathFailureHTTP429, OpenAIPathSignalHTTP1FallbackHit:
		return false
	default:
		return true
	}
}

func (t *OpenAIPathHealthTracker) getCooldownDuration(reason string) time.Duration {
	// 瞬时网络故障使用短冷却，快速恢复探测；但尊重更短的测试配置
	baseCooldown := t.options.Cooldown
	switch reason {
	case OpenAIPathFailureEOF, OpenAIPathFailureHeaderTimeout:
		shortCooldown := 30 * time.Second
		if baseCooldown < shortCooldown {
			return baseCooldown
		}
		return shortCooldown
	default:
		return baseCooldown
	}
}

func NormalizeOpenAIPathFailureReason(reason string) string {
	msg := strings.TrimSpace(reason)
	if msg == "" {
		return ""
	}
	switch msg {
	case OpenAIPathFailureHTTP2HeaderTimeout, OpenAIPathFailureHTTP2ProtocolError, OpenAIPathSignalHTTP1FallbackHit:
		return msg
	}
	classification := ClassifyUpstreamError(UpstreamErrorInput{Message: msg})
	if strings.TrimSpace(classification.PathHealthReason) == "" {
		return ""
	}
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

func openAIPathHealthReasonForHTTPAttempt(reason string, err error, attempt *HTTPUpstreamAttemptInfo) string {
	if attempt == nil || attempt.ProtocolMode != HTTPUpstreamProtocolModeOpenAIH2 {
		return reason
	}

	classification := ClassifyUpstreamError(UpstreamErrorInput{Message: reason, Err: err})
	if classification.Category == UpstreamErrorCategoryHeaderTimeout {
		return OpenAIPathFailureHTTP2HeaderTimeout
	}

	errText := ""
	if err != nil {
		errText = strings.ToLower(err.Error())
	}
	if classification.Category == UpstreamErrorCategoryUnexpectedEOF ||
		strings.Contains(errText, "stream id") ||
		strings.Contains(errText, "internal_error") {
		return OpenAIPathFailureHTTP2ProtocolError
	}
	return reason
}

func normalizeOpenAIPathHealthKey(key OpenAIPathHealthKey) OpenAIPathHealthKey {
	key.Upstream = normalizeOpenAIPathHealthPart(key.Upstream)
	key.Transport = normalizeOpenAIPathHealthTransport(key.Transport)
	return key
}

// openAIPathHealthWorseState 返回两个 path-health 状态中对调度影响更重的一个。
func openAIPathHealthWorseState(left string, right string) string {
	if openAIPathHealthStateRank(right) > openAIPathHealthStateRank(left) {
		return right
	}
	return left
}

// openAIPathHealthStateRank 将状态映射为调度严重度，数值越大越应避让。
func openAIPathHealthStateRank(state string) int {
	switch state {
	case OpenAIPathHealthStateOpenCircuit:
		return 3
	case OpenAIPathHealthStateHalfOpen:
		return 2
	case OpenAIPathHealthStateDegraded:
		return 1
	default:
		return 0
	}
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
