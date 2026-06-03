package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/google/uuid"
)

const (
	APIKeyProbeProfileQuick    = "quick"
	APIKeyProbeProfileStandard = "standard"

	APIKeyProbeStatusSuccess = "success"
	APIKeyProbeStatusPartial = "partial"
	APIKeyProbeStatusFailed  = "failed"
	APIKeyProbeStatusRunning = "running"

	APIKeyProbeSampleSuccess = "success"
	APIKeyProbeSampleFailed  = "failed"
	APIKeyProbeSampleWarning = "warning"

	APIKeyProbeUsageLogPersisted = "persisted"
	APIKeyProbeUsageLogMissing   = "missing"
)

type APIKeyProbeRunRequest struct {
	UserID                int64
	APIKeyID              int64
	Profile               string
	Model                 string
	IncludeCodexStability bool
	IncludeLongContext    bool
	BaseURL               string
}

type APIKeyProbePlan struct {
	Profile                  string
	EstimatedRequests        int
	EstimatedInputTokensMin  int
	EstimatedInputTokensMax  int
	EstimatedOutputTokensMin int
	EstimatedOutputTokensMax int
	Samples                  []APIKeyProbePlannedSample
}

type APIKeyProbePlannedSample struct {
	Type            string
	Label           string
	ValidationKey   string
	Prompt          string
	Timeout         time.Duration
	MaxOutputTokens int
	InputTokenMin   int
	InputTokenMax   int
	OutputTokenMin  int
	OutputTokenMax  int
}

type APIKeyProbeEstimate struct {
	Requests        int `json:"requests"`
	InputTokensMin  int `json:"input_tokens_min"`
	InputTokensMax  int `json:"input_tokens_max"`
	OutputTokensMin int `json:"output_tokens_min"`
	OutputTokensMax int `json:"output_tokens_max"`
	TotalTokensMin  int `json:"total_tokens_min"`
	TotalTokensMax  int `json:"total_tokens_max"`
}

type APIKeyProbeLatencyStats struct {
	P50Millis int `json:"p50_ms"`
	P95Millis int `json:"p95_ms"`
	AvgMillis int `json:"avg_ms"`
	MaxMillis int `json:"max_ms"`
}

type APIKeyProbeUsageStats struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	TotalCost    float64 `json:"total_cost"`
	ActualCost   float64 `json:"actual_cost"`
}

type APIKeyProbeResult struct {
	ID                    int64                   `json:"id"`
	UserID                int64                   `json:"user_id"`
	APIKeyID              int64                   `json:"api_key_id"`
	Profile               string                  `json:"mode"`
	Status                string                  `json:"status"`
	Model                 string                  `json:"model"`
	IncludeCodexStability bool                    `json:"codex_stability"`
	IncludeLongContext    bool                    `json:"long_context"`
	Estimate              APIKeyProbeEstimate     `json:"estimate"`
	Latency               APIKeyProbeLatencyStats `json:"latency"`
	Usage                 APIKeyProbeUsageStats   `json:"usage"`
	RequestCount          int                     `json:"request_count"`
	SuccessCount          int                     `json:"success_count"`
	FailureCount          int                     `json:"failure_count"`
	TotalTokens           int                     `json:"total_tokens"`
	AvgLatencyMillis      int                     `json:"avg_latency_ms"`
	MaxLatencyMillis      int                     `json:"max_latency_ms"`
	FirstTokenMillis      *int                    `json:"first_token_ms,omitempty"`
	ErrorMessage          string                  `json:"error_message,omitempty"`
	Summary               string                  `json:"summary,omitempty"`
	Samples               []APIKeyProbeSample     `json:"samples,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
	StartedAt             *time.Time              `json:"started_at,omitempty"`
	FinishedAt            *time.Time              `json:"finished_at,omitempty"`
}

type APIKeyProbeSample struct {
	ID               int64     `json:"id"`
	RunID            int64     `json:"run_id"`
	RequestIndex     int       `json:"request_index"`
	Type             string    `json:"type"`
	Label            string    `json:"label"`
	RequestID        string    `json:"request_id"`
	Status           string    `json:"status"`
	UsageLogID       int64     `json:"usage_log_id,omitempty"`
	UsageLogStatus   string    `json:"usage_log_status"`
	UsageLogMessage  string    `json:"usage_log_message,omitempty"`
	AccountID        int64     `json:"account_id,omitempty"`
	AccountName      string    `json:"account_name,omitempty"`
	UpstreamEndpoint string    `json:"upstream_endpoint,omitempty"`
	DurationMillis   int       `json:"latency_ms"`
	FirstTokenMillis *int      `json:"first_token_ms,omitempty"`
	InputTokens      int       `json:"input_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	TotalTokens      int       `json:"tokens"`
	TotalCost        float64   `json:"total_cost"`
	ActualCost       float64   `json:"actual_cost"`
	ErrorCode        string    `json:"error_code,omitempty"`
	ErrorMessage     string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type APIKeyProbeSampleRequest struct {
	APIKey    *APIKey
	BaseURL   string
	Model     string
	RequestID string
	Index     int
	Sample    APIKeyProbePlannedSample
}

type APIKeyProbeSampleResult struct {
	Duration         time.Duration
	FirstTokenMs     *int
	InputTokens      int
	OutputTokens     int
	TotalTokens      int
	TotalCost        float64
	ActualCost       float64
	AccountID        int64
	AccountName      string
	UpstreamEndpoint string
	UsageLogID       int64
	UsageLogStatus   string
	UsageLogMessage  string
	ErrorCode        string
	ErrorMessage     string
}

type APIKeyProbeHistoryFilter struct {
	UserID   int64
	APIKeyID int64
	Limit    int
}

type APIKeyProbeDependencies struct {
	APIKeys APIKeyProbeAPIKeyReader
	Runs    APIKeyProbeRunStore
	Samples APIKeyProbeSampleStore
	Runner  APIKeyProbeSampleRunner
}

type APIKeyProbeAPIKeyReader interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

type APIKeyProbeRunStore interface {
	CreateProbeRun(ctx context.Context, run *APIKeyProbeResult) error
	UpdateProbeRun(ctx context.Context, run *APIKeyProbeResult) error
	ListProbeRuns(ctx context.Context, filter APIKeyProbeHistoryFilter) ([]APIKeyProbeResult, error)
	GetProbeRun(ctx context.Context, userID, apiKeyID, runID int64) (*APIKeyProbeResult, error)
}

type APIKeyProbeSampleStore interface {
	SaveSample(ctx context.Context, sample APIKeyProbeSample) error
	ListProbeSamples(ctx context.Context, runID int64) ([]APIKeyProbeSample, error)
}

type APIKeyProbeSampleRunner interface {
	RunSample(ctx context.Context, req APIKeyProbeSampleRequest) (APIKeyProbeSampleResult, error)
}

type APIKeyProbeHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type APIKeyProbeUsageFinder interface {
	FindUsageByRequestID(ctx context.Context, apiKeyID int64, requestID string) (*UsageLog, error)
}

type APIKeyProbeRepository interface {
	APIKeyProbeRunStore
	APIKeyProbeSampleStore
	APIKeyProbeUsageFinder
}

type APIKeyProbeService struct {
	apiKeys APIKeyProbeAPIKeyReader
	runs    APIKeyProbeRunStore
	samples APIKeyProbeSampleStore
	runner  APIKeyProbeSampleRunner
}

func NewAPIKeyProbeService(deps APIKeyProbeDependencies) *APIKeyProbeService {
	return &APIKeyProbeService{
		apiKeys: deps.APIKeys,
		runs:    deps.Runs,
		samples: deps.Samples,
		runner:  deps.Runner,
	}
}

func ProvideAPIKeyProbeService(apiKeyRepo APIKeyRepository, probeRepo APIKeyProbeRepository) *APIKeyProbeService {
	runner := NewHTTPAPIKeyProbeRunner(&http.Client{}, probeRepo)
	return NewAPIKeyProbeService(APIKeyProbeDependencies{
		APIKeys: apiKeyRepo,
		Runs:    probeRepo,
		Samples: probeRepo,
		Runner:  runner,
	})
}

func (s *APIKeyProbeService) Plan(ctx context.Context, req APIKeyProbeRunRequest) (APIKeyProbePlan, error) {
	profile := normalizeAPIKeyProbeProfile(req.Profile)
	samples := make([]APIKeyProbePlannedSample, 0, 9)
	baseCount := 3
	if profile == APIKeyProbeProfileQuick {
		baseCount = 1
	}
	for i := 0; i < baseCount; i++ {
		samples = append(samples, shortProbeSample("基础测速"))
	}
	if req.IncludeCodexStability {
		for i := 0; i < 5; i++ {
			samples = append(samples, shortProbeSample("Codex 稳定性小测"))
		}
	}
	if req.IncludeLongContext {
		samples = append(samples, longContextProbeSample())
	}
	plan := APIKeyProbePlan{Profile: profile, Samples: samples, EstimatedRequests: len(samples)}
	for _, sample := range samples {
		plan.EstimatedInputTokensMin += sample.InputTokenMin
		plan.EstimatedInputTokensMax += sample.InputTokenMax
		plan.EstimatedOutputTokensMin += sample.OutputTokenMin
		plan.EstimatedOutputTokensMax += sample.OutputTokenMax
	}
	return plan, nil
}

func (s *APIKeyProbeService) Run(ctx context.Context, req APIKeyProbeRunRequest) (APIKeyProbeResult, error) {
	if s.apiKeys == nil {
		return APIKeyProbeResult{}, fmt.Errorf("api key probe api key reader is nil")
	}
	key, err := s.apiKeys.GetByID(ctx, req.APIKeyID)
	if err != nil {
		return APIKeyProbeResult{}, err
	}
	if key.UserID != req.UserID {
		return APIKeyProbeResult{}, ErrInsufficientPerms
	}
	if key.Status != "" && !key.IsActive() {
		return APIKeyProbeResult{}, ErrAPIKeyNotFound
	}
	if key.IsExpired() {
		return APIKeyProbeResult{}, ErrAPIKeyExpired
	}
	if key.IsQuotaExhausted() {
		return APIKeyProbeResult{}, ErrAPIKeyQuotaExhausted
	}

	plan, err := s.Plan(ctx, req)
	if err != nil {
		return APIKeyProbeResult{}, err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "gpt-5-mini"
	}
	now := time.Now()
	run := APIKeyProbeResult{
		UserID:                req.UserID,
		APIKeyID:              req.APIKeyID,
		Profile:               plan.Profile,
		Status:                APIKeyProbeStatusRunning,
		Model:                 model,
		IncludeCodexStability: req.IncludeCodexStability,
		IncludeLongContext:    req.IncludeLongContext,
		Estimate:              estimateFromPlan(plan),
		RequestCount:          plan.EstimatedRequests,
		CreatedAt:             now,
		StartedAt:             &now,
	}
	if s.runs != nil {
		if err := s.runs.CreateProbeRun(ctx, &run); err != nil {
			return APIKeyProbeResult{}, err
		}
	}

	for idx, planned := range plan.Samples {
		requestID := fmt.Sprintf("client:api-key-probe-%s-%02d", uuid.NewString(), idx+1)
		sampleReq := APIKeyProbeSampleRequest{
			APIKey:    key,
			BaseURL:   req.BaseURL,
			Model:     model,
			RequestID: requestID,
			Index:     idx + 1,
			Sample:    planned,
		}
		res, sampleErr := s.runSample(ctx, sampleReq)
		sample := sampleFromResult(run.ID, idx+1, planned, requestID, res, sampleErr)
		if sampleErr != nil {
			sample.Status = APIKeyProbeSampleFailed
			sample.ErrorCode = "probe_request_failed"
			sample.ErrorMessage = sampleErr.Error()
		}
		if s.samples != nil {
			if err := s.samples.SaveSample(ctx, sample); err != nil {
				return APIKeyProbeResult{}, err
			}
		}
		run.Samples = append(run.Samples, sample)
	}
	finalizeAPIKeyProbeResult(&run)
	if s.runs != nil {
		if err := s.runs.UpdateProbeRun(ctx, &run); err != nil {
			return APIKeyProbeResult{}, err
		}
	}
	return run, nil
}

func (s *APIKeyProbeService) List(ctx context.Context, filter APIKeyProbeHistoryFilter) ([]APIKeyProbeResult, error) {
	if s.runs == nil {
		return nil, fmt.Errorf("api key probe run store is nil")
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	runs, err := s.runs.ListProbeRuns(ctx, filter)
	if err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *APIKeyProbeService) Get(ctx context.Context, userID, apiKeyID, runID int64) (*APIKeyProbeResult, error) {
	if s.runs == nil {
		return nil, fmt.Errorf("api key probe run store is nil")
	}
	run, err := s.runs.GetProbeRun(ctx, userID, apiKeyID, runID)
	if err != nil {
		return nil, err
	}
	if s.samples != nil {
		samples, err := s.samples.ListProbeSamples(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		run.Samples = samples
	}
	return run, nil
}

func (s *APIKeyProbeService) runSample(ctx context.Context, req APIKeyProbeSampleRequest) (APIKeyProbeSampleResult, error) {
	if s.runner == nil {
		return APIKeyProbeSampleResult{}, fmt.Errorf("api key probe runner is nil")
	}
	return s.runner.RunSample(ctx, req)
}

type HTTPAPIKeyProbeRunner struct {
	client      APIKeyProbeHTTPClient
	usage       APIKeyProbeUsageFinder
	pollTimeout time.Duration
}

func NewHTTPAPIKeyProbeRunner(client APIKeyProbeHTTPClient, usage APIKeyProbeUsageFinder) *HTTPAPIKeyProbeRunner {
	if client == nil {
		client = &http.Client{}
	}
	return &HTTPAPIKeyProbeRunner{client: client, usage: usage, pollTimeout: 5 * time.Second}
}

func (r *HTTPAPIKeyProbeRunner) RunSample(ctx context.Context, req APIKeyProbeSampleRequest) (APIKeyProbeSampleResult, error) {
	if req.APIKey == nil {
		return APIKeyProbeSampleResult{}, fmt.Errorf("api key is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if baseURL == "" {
		return APIKeyProbeSampleResult{}, fmt.Errorf("base url is required")
	}
	timeout := req.Sample.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	sampleCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body := buildOpenAIResponsesProbePayload(req.Model, req.Sample.Prompt, false, req.Sample.MaxOutputTokens)
	payload, err := json.Marshal(body)
	if err != nil {
		return APIKeyProbeSampleResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(sampleCtx, http.MethodPost, baseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return APIKeyProbeSampleResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey.Key)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Client-Request-ID", strings.TrimPrefix(req.RequestID, "client:"))
	httpReq.Header.Set("X-Request-ID", strings.TrimPrefix(req.RequestID, "client:"))

	start := time.Now()
	resp, err := r.client.Do(httpReq)
	duration := time.Since(start)
	result := APIKeyProbeSampleResult{Duration: duration}
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = strings.TrimSpace(string(data))
		if result.ErrorMessage == "" {
			result.ErrorMessage = resp.Status
		}
		return result, fmt.Errorf("probe request returned %s", resp.Status)
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if r.usage == nil {
		result.UsageLogStatus = APIKeyProbeUsageLogMissing
		result.UsageLogMessage = "usage log lookup is not configured"
		return result, nil
	}
	usage, err := r.waitUsageLog(ctx, req.APIKey.ID, req.RequestID)
	if err != nil {
		result.UsageLogStatus = APIKeyProbeUsageLogMissing
		result.UsageLogMessage = err.Error()
		return result, nil
	}
	result.UsageLogID = usage.ID
	result.UsageLogStatus = APIKeyProbeUsageLogPersisted
	result.InputTokens = usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
	result.OutputTokens = usage.OutputTokens
	result.TotalTokens = usage.TotalTokens()
	result.TotalCost = usage.TotalCost
	result.ActualCost = usage.ActualCost
	result.AccountID = usage.AccountID
	if usage.Account != nil {
		result.AccountName = usage.Account.Name
	}
	if usage.DurationMs != nil {
		result.Duration = time.Duration(*usage.DurationMs) * time.Millisecond
	}
	result.FirstTokenMs = usage.FirstTokenMs
	if usage.UpstreamEndpoint != nil {
		result.UpstreamEndpoint = *usage.UpstreamEndpoint
	}
	return result, nil
}

func (r *HTTPAPIKeyProbeRunner) waitUsageLog(ctx context.Context, apiKeyID int64, requestID string) (*UsageLog, error) {
	deadline := time.Now().Add(r.pollTimeout)
	for {
		usage, err := r.usage.FindUsageByRequestID(ctx, apiKeyID, requestID)
		if err == nil && usage != nil {
			return usage, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("usage log was not persisted before probe collection timeout")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func normalizeAPIKeyProbeProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case APIKeyProbeProfileQuick:
		return APIKeyProbeProfileQuick
	default:
		return APIKeyProbeProfileStandard
	}
}

func shortProbeSample(label string) APIKeyProbePlannedSample {
	return APIKeyProbePlannedSample{
		Type:            "short",
		Label:           label,
		Prompt:          "只回复 ok，用于 API Key 测速体检。",
		Timeout:         60 * time.Second,
		MaxOutputTokens: 20,
		InputTokenMin:   40,
		InputTokenMax:   120,
		OutputTokenMin:  10,
		OutputTokenMax:  20,
	}
}

func longContextProbeSample() APIKeyProbePlannedSample {
	paragraph := "这是一段用于长上下文小测的本地体检文本，请保持理解但只在最后回复 ok。"
	return APIKeyProbePlannedSample{
		Type:            "long_context",
		Label:           "长上下文小测",
		Prompt:          strings.Repeat(paragraph, 36),
		Timeout:         180 * time.Second,
		MaxOutputTokens: 40,
		InputTokenMin:   1000,
		InputTokenMax:   2000,
		OutputTokenMin:  20,
		OutputTokenMax:  40,
	}
}

func estimateFromPlan(plan APIKeyProbePlan) APIKeyProbeEstimate {
	return APIKeyProbeEstimate{
		Requests:        plan.EstimatedRequests,
		InputTokensMin:  plan.EstimatedInputTokensMin,
		InputTokensMax:  plan.EstimatedInputTokensMax,
		OutputTokensMin: plan.EstimatedOutputTokensMin,
		OutputTokensMax: plan.EstimatedOutputTokensMax,
		TotalTokensMin:  plan.EstimatedInputTokensMin + plan.EstimatedOutputTokensMin,
		TotalTokensMax:  plan.EstimatedInputTokensMax + plan.EstimatedOutputTokensMax,
	}
}

func sampleFromResult(runID int64, index int, planned APIKeyProbePlannedSample, requestID string, result APIKeyProbeSampleResult, err error) APIKeyProbeSample {
	status := APIKeyProbeSampleSuccess
	usageStatus := result.UsageLogStatus
	if usageStatus == "" {
		usageStatus = APIKeyProbeUsageLogPersisted
	}
	if usageStatus == APIKeyProbeUsageLogMissing {
		status = APIKeyProbeSampleWarning
	}
	if err != nil {
		status = APIKeyProbeSampleFailed
	}
	totalTokens := result.TotalTokens
	if totalTokens == 0 {
		totalTokens = result.InputTokens + result.OutputTokens
	}
	return APIKeyProbeSample{
		RunID:            runID,
		RequestIndex:     index,
		Type:             planned.Type,
		Label:            planned.Label,
		RequestID:        requestID,
		Status:           status,
		UsageLogID:       result.UsageLogID,
		UsageLogStatus:   usageStatus,
		UsageLogMessage:  result.UsageLogMessage,
		AccountID:        result.AccountID,
		AccountName:      result.AccountName,
		UpstreamEndpoint: result.UpstreamEndpoint,
		DurationMillis:   int(math.Round(float64(result.Duration / time.Millisecond))),
		FirstTokenMillis: result.FirstTokenMs,
		InputTokens:      result.InputTokens,
		OutputTokens:     result.OutputTokens,
		TotalTokens:      totalTokens,
		TotalCost:        result.TotalCost,
		ActualCost:       result.ActualCost,
		ErrorCode:        result.ErrorCode,
		ErrorMessage:     result.ErrorMessage,
		CreatedAt:        time.Now(),
	}
}

func finalizeAPIKeyProbeResult(run *APIKeyProbeResult) {
	if run == nil {
		return
	}
	durations := make([]int, 0, len(run.Samples))
	var firstTokens []int
	for _, sample := range run.Samples {
		if sample.Status == APIKeyProbeSampleFailed {
			run.FailureCount++
		} else {
			run.SuccessCount++
		}
		if sample.DurationMillis > 0 {
			durations = append(durations, sample.DurationMillis)
		}
		if sample.FirstTokenMillis != nil {
			firstTokens = append(firstTokens, *sample.FirstTokenMillis)
		}
		run.Usage.InputTokens += sample.InputTokens
		run.Usage.OutputTokens += sample.OutputTokens
		run.Usage.TotalTokens += sample.TotalTokens
		run.Usage.TotalCost += sample.TotalCost
		run.Usage.ActualCost += sample.ActualCost
	}
	run.TotalTokens = run.Usage.TotalTokens
	run.Latency = latencyStats(durations)
	run.AvgLatencyMillis = run.Latency.AvgMillis
	run.MaxLatencyMillis = run.Latency.MaxMillis
	if len(firstTokens) > 0 {
		v := latencyStats(firstTokens).P50Millis
		run.FirstTokenMillis = &v
	}
	switch {
	case run.FailureCount == 0:
		run.Status = APIKeyProbeStatusSuccess
	case run.SuccessCount == 0:
		run.Status = APIKeyProbeStatusFailed
	default:
		run.Status = APIKeyProbeStatusPartial
	}
	finished := time.Now()
	run.FinishedAt = &finished
	run.Summary = fmt.Sprintf("完成 %d/%d 次请求，平均延迟 %d ms，消耗 %d tokens", run.SuccessCount, len(run.Samples), run.Latency.AvgMillis, run.TotalTokens)
}

func latencyStats(values []int) APIKeyProbeLatencyStats {
	if len(values) == 0 {
		return APIKeyProbeLatencyStats{}
	}
	sort.Ints(values)
	sum := 0
	for _, v := range values {
		sum += v
	}
	return APIKeyProbeLatencyStats{
		P50Millis: percentileNearestRank(values, 0.50),
		P95Millis: percentileNearestRank(values, 0.95),
		AvgMillis: sum / len(values),
		MaxMillis: values[len(values)-1],
	}
}

func percentileNearestRank(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func ProbeClientRequestIDContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxkey.ClientRequestID, strings.TrimPrefix(requestID, "client:"))
}
