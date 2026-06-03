package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

const (
	AccountProbeProfileQuick           = APIKeyProbeProfileQuick
	AccountProbeProfileStandard        = APIKeyProbeProfileStandard
	AccountProbeProfileModelValidation = "model_validation"

	AccountProbeStatusSuccess = APIKeyProbeStatusSuccess
	AccountProbeStatusPartial = APIKeyProbeStatusPartial
	AccountProbeStatusFailed  = APIKeyProbeStatusFailed
	AccountProbeStatusRunning = APIKeyProbeStatusRunning

	AccountProbeSampleSuccess = APIKeyProbeSampleSuccess
	AccountProbeSampleFailed  = APIKeyProbeSampleFailed

	AccountProbeRequestModeNonStream = "non_stream"
	AccountProbeRequestModeStream    = "stream"
)

const (
	accountProbePersistenceTimeout = 5 * time.Second
	accountProbeRetryDelay         = 2 * time.Second
	accountProbeStaleRunAge        = 12 * time.Minute
	accountProbeRankingHistorySize = 20
	accountProbeOutputTextLimit    = 1000
)

type AccountProbeRunRequest struct {
	AccountID             int64  `json:"-"`
	Profile               string `json:"mode"`
	Model                 string `json:"model"`
	IncludeCodexStability bool   `json:"codex_stability"`
	IncludeLongContext    bool   `json:"long_context"`
	RequestMode           string `json:"request_mode"`
	ModelValidationOnly   bool   `json:"-"`
}

type AccountProbeEstimate = APIKeyProbeEstimate
type AccountProbeLatencyStats = APIKeyProbeLatencyStats

// AccountProbeValidationEvidence 记录单个模型行为探针的结构化判定依据。
type AccountProbeValidationEvidence struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
	Passed   bool   `json:"passed"`
	Score    int    `json:"score"`
	MaxScore int    `json:"max_score"`
	Message  string `json:"message,omitempty"`
}

type AccountProbeResult struct {
	ID                    int64                    `json:"id"`
	AccountID             int64                    `json:"account_id"`
	Profile               string                   `json:"mode"`
	Status                string                   `json:"status"`
	Model                 string                   `json:"model"`
	RequestMode           string                   `json:"request_mode"`
	IncludeCodexStability bool                     `json:"codex_stability"`
	IncludeLongContext    bool                     `json:"long_context"`
	Estimate              AccountProbeEstimate     `json:"estimate"`
	Latency               AccountProbeLatencyStats `json:"latency"`
	RequestCount          int                      `json:"request_count"`
	SuccessCount          int                      `json:"success_count"`
	FailureCount          int                      `json:"failure_count"`
	InputTokens           int                      `json:"input_tokens"`
	OutputTokens          int                      `json:"output_tokens"`
	TotalTokens           int                      `json:"total_tokens"`
	AvgLatencyMillis      int                      `json:"avg_latency_ms"`
	MaxLatencyMillis      int                      `json:"max_latency_ms"`
	FirstTokenMillis      *int                     `json:"first_token_ms,omitempty"`
	ErrorMessage          string                   `json:"error_message,omitempty"`
	Summary               string                   `json:"summary,omitempty"`
	Samples               []AccountProbeSample     `json:"samples,omitempty"`
	CreatedAt             time.Time                `json:"created_at"`
	StartedAt             *time.Time               `json:"started_at,omitempty"`
	FinishedAt            *time.Time               `json:"finished_at,omitempty"`
}

type AccountProbeSample struct {
	ID                 int64                            `json:"id"`
	RunID              int64                            `json:"run_id"`
	RequestIndex       int                              `json:"request_index"`
	Type               string                           `json:"type"`
	Label              string                           `json:"label"`
	Status             string                           `json:"status"`
	Model              string                           `json:"model"`
	APIKeyFingerprint  string                           `json:"api_key_fingerprint,omitempty"`
	APIKeyMasked       string                           `json:"api_key_masked,omitempty"`
	UpstreamEndpoint   string                           `json:"upstream_endpoint,omitempty"`
	HTTPStatus         int                              `json:"http_status,omitempty"`
	DurationMillis     int                              `json:"latency_ms"`
	FirstTokenMillis   *int                             `json:"first_token_ms,omitempty"`
	InputTokens        int                              `json:"input_tokens"`
	OutputTokens       int                              `json:"output_tokens"`
	TotalTokens        int                              `json:"tokens"`
	OutputText         string                           `json:"output_text,omitempty"`
	ValidationEvidence []AccountProbeValidationEvidence `json:"validation_evidence,omitempty"`
	ErrorCode          string                           `json:"error_code,omitempty"`
	ErrorMessage       string                           `json:"error,omitempty"`
	CreatedAt          time.Time                        `json:"created_at"`
}

type AccountProbeHistoryFilter struct {
	AccountID int64
	Limit     int
}

type AccountProbeReportFilter struct {
	AccountID   int64
	Status      string
	Profile     string
	RequestMode string
	Model       string
	Keyword     string
	From        *time.Time
	To          *time.Time
	Sort        string
	Order       string
	Page        int
	PageSize    int
}

type AccountProbeReportItem struct {
	AccountProbeResult
	AccountName  string   `json:"account_name"`
	Score        int      `json:"score"`
	Grade        string   `json:"grade"`
	GradeLabel   string   `json:"grade_label"`
	Confidence   int      `json:"confidence"`
	ScoreItems   []string `json:"score_items,omitempty"`
	PenaltyItems []string `json:"penalty_items,omitempty"`
	SuccessRate  float64  `json:"success_rate"`
}

type AccountProbeScorePoint struct {
	RunID            int64     `json:"run_id"`
	Score            int       `json:"score"`
	Grade            string    `json:"grade"`
	GradeLabel       string    `json:"grade_label"`
	Status           string    `json:"status"`
	Model            string    `json:"model"`
	Profile          string    `json:"mode"`
	RequestMode      string    `json:"request_mode"`
	SuccessRate      float64   `json:"success_rate"`
	AvgLatencyMillis int       `json:"avg_latency_ms"`
	P95LatencyMillis int       `json:"p95_ms"`
	FirstTokenMillis *int      `json:"first_token_ms,omitempty"`
	TotalTokens      int       `json:"total_tokens"`
	CreatedAt        time.Time `json:"created_at"`
}

type AccountProbeRankingItem struct {
	AccountID            int64                    `json:"account_id"`
	AccountName          string                   `json:"account_name"`
	RunCount             int                      `json:"run_count"`
	AverageScore         float64                  `json:"average_score"`
	LatestScore          int                      `json:"latest_score"`
	Grade                string                   `json:"grade"`
	GradeLabel           string                   `json:"grade_label"`
	LatestRunID          int64                    `json:"latest_run_id"`
	LatestStatus         string                   `json:"latest_status"`
	LatestCreatedAt      time.Time                `json:"latest_created_at"`
	LatestModel          string                   `json:"latest_model"`
	AverageSuccessRate   float64                  `json:"average_success_rate"`
	AverageLatencyMillis float64                  `json:"average_latency_ms"`
	ScoreHistory         []AccountProbeScorePoint `json:"score_history"`
}

type AccountProbeReportSummary struct {
	Total             int     `json:"total"`
	AverageScore      float64 `json:"average_score"`
	ExcellentCount    int     `json:"excellent_count"`
	UnstableCount     int     `json:"unstable_count"`
	Recent24HourCount int     `json:"recent_24h_count"`
	RunningCount      int     `json:"running_count"`
}

type AccountProbeReportPage struct {
	Items    []AccountProbeReportItem  `json:"items"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Summary  AccountProbeReportSummary `json:"summary"`
}

type AccountProbeReportDeleteResult struct {
	RequestedCount      int `json:"requested_count"`
	DeletedCount        int `json:"deleted_count"`
	SkippedRunningCount int `json:"skipped_running_count"`
}

type AccountProbeRepository interface {
	CreateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error
	UpdateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error
	ExpireStaleAccountProbeRuns(ctx context.Context, olderThan time.Duration) error
	SaveAccountProbeSample(ctx context.Context, sample AccountProbeSample) error
	ListAccountProbeRuns(ctx context.Context, filter AccountProbeHistoryFilter) ([]AccountProbeResult, error)
	GetAccountProbeRun(ctx context.Context, accountID, runID int64) (*AccountProbeResult, error)
	ListAccountProbeReportRuns(ctx context.Context, filter AccountProbeReportFilter) ([]AccountProbeReportItem, int, error)
	GetAccountProbeReportRun(ctx context.Context, runID int64) (*AccountProbeReportItem, error)
	DeleteAccountProbeReportRuns(ctx context.Context, runIDs []int64) (AccountProbeReportDeleteResult, error)
	ListAccountProbeSamples(ctx context.Context, runID int64) ([]AccountProbeSample, error)
	ListAccountProbeRankingRuns(ctx context.Context, perAccountLimit int) ([]AccountProbeReportItem, error)
}

type AccountProbeHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type AccountProbeAccountReader interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type AccountProbeService struct {
	accounts AccountProbeAccountReader
	repo     AccountProbeRepository
	client   AccountProbeHTTPClient
	testSvc  *AccountTestService
	health   *OpenAIPathHealthTracker
}

var accountProbeBaseURLLocks sync.Map

func NewAccountProbeService(accounts AccountProbeAccountReader, repo AccountProbeRepository, client AccountProbeHTTPClient, testSvc *AccountTestService) *AccountProbeService {
	if client == nil {
		client = &http.Client{}
	}
	service := &AccountProbeService{accounts: accounts, repo: repo, client: client, testSvc: testSvc}
	if testSvc != nil {
		service.health = testSvc.openAIPathHealth()
	}
	return service
}

func (s *AccountProbeService) SetOpenAIPathHealthTracker(tracker *OpenAIPathHealthTracker) {
	if s == nil {
		return
	}
	s.health = tracker
}

func (s *AccountProbeService) DeleteReports(ctx context.Context, runIDs []int64) (AccountProbeReportDeleteResult, error) {
	ids := uniquePositiveProbeRunIDs(runIDs)
	if len(ids) == 0 {
		return AccountProbeReportDeleteResult{}, fmt.Errorf("run_ids is required")
	}
	if len(ids) > 200 {
		return AccountProbeReportDeleteResult{}, fmt.Errorf("run_ids cannot exceed 200")
	}
	if s.repo == nil {
		return AccountProbeReportDeleteResult{}, fmt.Errorf("account probe repository is not configured")
	}
	return s.repo.DeleteAccountProbeReportRuns(ctx, ids)
}

func (s *AccountProbeService) Run(ctx context.Context, req AccountProbeRunRequest) (AccountProbeResult, error) {
	run, err := s.Start(ctx, req)
	if err != nil {
		return AccountProbeResult{}, err
	}
	return s.RunExisting(ctx, run, req)
}

func uniquePositiveProbeRunIDs(values []int64) []int64 {
	seen := make(map[int64]bool, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s *AccountProbeService) Start(ctx context.Context, req AccountProbeRunRequest) (AccountProbeResult, error) {
	_, _, plan, model, _, _, err := s.prepareRun(ctx, req)
	if err != nil {
		return AccountProbeResult{}, err
	}

	now := time.Now()
	run := AccountProbeResult{
		AccountID:             req.AccountID,
		Profile:               plan.Profile,
		Status:                AccountProbeStatusRunning,
		Model:                 model,
		RequestMode:           normalizeAccountProbeRequestMode(req.RequestMode),
		IncludeCodexStability: req.IncludeCodexStability && !req.ModelValidationOnly,
		IncludeLongContext:    req.IncludeLongContext,
		Estimate:              plan.Estimate,
		RequestCount:          len(plan.Samples),
		CreatedAt:             now,
		StartedAt:             &now,
	}
	if s.repo != nil {
		if err := s.repo.CreateAccountProbeRun(ctx, &run); err != nil {
			return AccountProbeResult{}, err
		}
	}
	return run, nil
}

func (s *AccountProbeService) RunExisting(ctx context.Context, run AccountProbeResult, req AccountProbeRunRequest) (AccountProbeResult, error) {
	account, keys, plan, model, baseURLs, useResponses, err := s.prepareRun(ctx, req)
	if err != nil {
		return s.failExistingRun(ctx, run, err.Error()), err
	}
	releaseBaseURL := acquireAccountProbeBaseURLLock(ctx, strings.Join(baseURLs, "\n"))
	if releaseBaseURL == nil {
		return s.failExistingRun(ctx, run, "probe canceled while waiting for upstream concurrency slot"), ctx.Err()
	}
	defer releaseBaseURL()
	run.AccountID = account.ID
	run.Profile = plan.Profile
	run.Model = model
	run.RequestMode = normalizeAccountProbeRequestMode(req.RequestMode)
	run.IncludeCodexStability = req.IncludeCodexStability && !req.ModelValidationOnly
	run.IncludeLongContext = req.IncludeLongContext
	run.Estimate = plan.Estimate
	run.RequestCount = len(plan.Samples)
	if run.Status == "" {
		run.Status = AccountProbeStatusRunning
	}
	if run.StartedAt == nil {
		now := time.Now()
		run.StartedAt = &now
	}

	for idx, planned := range plan.Samples {
		key := keys[idx%len(keys)]
		baseURL := baseURLs[idx%len(baseURLs)]
		sample := s.runOpenAIAPIKeySampleWithRetry(ctx, account, baseURL, model, key, planned, useResponses, run.RequestMode)
		s.recordProbePathHealth(account, baseURL, sample)
		sample.RunID = run.ID
		sample.RequestIndex = idx + 1
		sample.Type = planned.Type
		sample.Label = planned.Label
		sample.Model = model
		if s.repo != nil {
			persistCtx, cancel := context.WithTimeout(context.Background(), accountProbePersistenceTimeout)
			err := s.repo.SaveAccountProbeSample(persistCtx, sample)
			cancel()
			if err != nil {
				return s.failExistingRun(ctx, run, err.Error()), err
			}
		}
		run.Samples = append(run.Samples, sample)
		if ctx.Err() != nil {
			break
		}
	}
	finalizeAccountProbeResult(&run)
	if s.repo != nil {
		persistCtx, cancel := context.WithTimeout(context.Background(), accountProbePersistenceTimeout)
		err := s.repo.UpdateAccountProbeRun(persistCtx, &run)
		cancel()
		if err != nil {
			return run, err
		}
	}
	return run, nil
}

func (s *AccountProbeService) recordProbePathHealth(account *Account, baseURL string, sample AccountProbeSample) {
	if s == nil || s.health == nil || account == nil {
		return
	}
	key := OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), baseURL)
	headerWait := int64(sample.DurationMillis)
	if sample.Status == AccountProbeSampleSuccess {
		s.health.RecordSuccess(key, sample.FirstTokenMillis, &headerWait)
		return
	}
	reason := sample.ErrorMessage
	if reason == "" && sample.ErrorCode != "" {
		reason = sample.ErrorCode
	}
	if reason == "" && sample.HTTPStatus > 0 {
		reason = fmt.Sprintf("http_%d", sample.HTTPStatus)
	}
	s.health.RecordFailure(key, reason, &headerWait)
}

func acquireAccountProbeBaseURLLock(ctx context.Context, baseURL string) func() {
	key := strings.ToLower(strings.TrimSpace(baseURL))
	if key == "" {
		key = "default"
	}
	value, _ := accountProbeBaseURLLocks.LoadOrStore(key, make(chan struct{}, 1))
	sem := value.(chan struct{})
	select {
	case sem <- struct{}{}:
		return func() { <-sem }
	default:
	}
	select {
	case sem <- struct{}{}:
		return func() { <-sem }
	case <-ctx.Done():
		return nil
	}
}

func (s *AccountProbeService) runOpenAIAPIKeySampleWithRetry(ctx context.Context, account *Account, baseURL, model, apiKey string, sample APIKeyProbePlannedSample, useResponses bool, requestMode string) AccountProbeSample {
	result := s.runOpenAIAPIKeySample(ctx, account, baseURL, model, apiKey, sample, useResponses, requestMode)
	if !shouldRetryAccountProbeSample(result) || ctx.Err() != nil {
		return result
	}
	delay := accountProbeRetryDelay
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < delay {
		delay = 0
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return result
		}
	}
	retry := s.runOpenAIAPIKeySample(ctx, account, baseURL, model, apiKey, sample, useResponses, requestMode)
	if retry.Status == AccountProbeSampleSuccess {
		return retry
	}
	return result
}

func shouldRetryAccountProbeSample(sample AccountProbeSample) bool {
	if sample.Status != AccountProbeSampleFailed {
		return false
	}
	if sample.ErrorCode == "request_failed" && isTransientAccountProbeError(sample.ErrorMessage) {
		return true
	}
	switch sample.HTTPStatus {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, 522, 524:
		return true
	}
	return false
}

func isTransientAccountProbeError(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "unexpected eof") ||
		lower == "eof" ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "server overloaded") ||
		strings.Contains(lower, "currently overloaded")
}

func (s *AccountProbeService) prepareRun(ctx context.Context, req AccountProbeRunRequest) (*Account, []string, accountProbePlan, string, []string, bool, error) {
	if s.accounts == nil {
		return nil, nil, accountProbePlan{}, "", nil, false, fmt.Errorf("account probe account repository is nil")
	}
	account, err := s.accounts.GetByID(ctx, req.AccountID)
	if err != nil {
		return nil, nil, accountProbePlan{}, "", nil, false, err
	}
	if account == nil {
		return nil, nil, accountProbePlan{}, "", nil, false, ErrAccountNotFound
	}
	if !account.IsOpenAIApiKey() {
		return nil, nil, accountProbePlan{}, "", nil, false, fmt.Errorf("only openai api key accounts support probe runs")
	}
	keys := account.GetAPIKeys()
	if len(keys) == 0 {
		if key := strings.TrimSpace(account.GetOpenAIApiKey()); key != "" {
			keys = []string{key}
		}
	}
	if len(keys) == 0 {
		return nil, nil, accountProbePlan{}, "", nil, false, fmt.Errorf("no api key available")
	}

	plan, err := buildAccountProbePlan(req)
	if err != nil {
		return nil, nil, accountProbePlan{}, "", nil, false, err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = openai.DefaultTestModel
	}
	model = account.GetMappedModel(model)
	baseURLs := account.GetOpenAIRequestBaseURLs()
	if len(baseURLs) == 0 {
		baseURLs = []string{"https://api.openai.com"}
	}
	if s.testSvc != nil {
		normalizedBaseURLs := make([]string, 0, len(baseURLs))
		for _, baseURL := range baseURLs {
			normalized, err := s.testSvc.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return nil, nil, accountProbePlan{}, "", nil, false, fmt.Errorf("invalid base URL: %w", err)
			}
			normalizedBaseURLs = append(normalizedBaseURLs, normalized)
		}
		baseURLs = normalizedBaseURLs
	}

	useResponses := openai_compat.ShouldUseResponsesAPI(account.Extra)
	return account, keys, plan, model, baseURLs, useResponses, nil
}

func (s *AccountProbeService) failExistingRun(ctx context.Context, run AccountProbeResult, message string) AccountProbeResult {
	run.Status = AccountProbeStatusFailed
	run.ErrorMessage = strings.TrimSpace(message)
	finished := time.Now()
	run.FinishedAt = &finished
	if s.repo != nil {
		persistCtx, cancel := context.WithTimeout(context.Background(), accountProbePersistenceTimeout)
		_ = s.repo.UpdateAccountProbeRun(persistCtx, &run)
		cancel()
	}
	return run
}

func (s *AccountProbeService) List(ctx context.Context, filter AccountProbeHistoryFilter) ([]AccountProbeResult, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("account probe repository is nil")
	}
	if err := s.repo.ExpireStaleAccountProbeRuns(ctx, accountProbeStaleRunAge); err != nil {
		return nil, err
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	return s.repo.ListAccountProbeRuns(ctx, filter)
}

func (s *AccountProbeService) Get(ctx context.Context, accountID, runID int64) (*AccountProbeResult, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("account probe repository is nil")
	}
	if err := s.repo.ExpireStaleAccountProbeRuns(ctx, accountProbeStaleRunAge); err != nil {
		return nil, err
	}
	run, err := s.repo.GetAccountProbeRun(ctx, accountID, runID)
	if err != nil {
		return nil, err
	}
	samples, err := s.repo.ListAccountProbeSamples(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	run.Samples = samples
	return run, nil
}

func (s *AccountProbeService) ListReports(ctx context.Context, filter AccountProbeReportFilter) (AccountProbeReportPage, error) {
	if s.repo == nil {
		return AccountProbeReportPage{}, fmt.Errorf("account probe repository is nil")
	}
	if err := s.repo.ExpireStaleAccountProbeRuns(ctx, accountProbeStaleRunAge); err != nil {
		return AccountProbeReportPage{}, err
	}
	filter = normalizeAccountProbeReportFilter(filter)
	items, total, err := s.repo.ListAccountProbeReportRuns(ctx, filter)
	if err != nil {
		return AccountProbeReportPage{}, err
	}
	for i := range items {
		decorateAccountProbeReportItem(&items[i])
	}
	if filter.Sort == "score" {
		sort.SliceStable(items, func(i, j int) bool {
			if strings.EqualFold(filter.Order, "asc") {
				return items[i].Score < items[j].Score
			}
			return items[i].Score > items[j].Score
		})
	}
	return AccountProbeReportPage{
		Items:    items,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Summary:  buildAccountProbeReportSummary(items, total),
	}, nil
}

func (s *AccountProbeService) ListRanking(ctx context.Context, limit int) ([]AccountProbeRankingItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("account probe repository is nil")
	}
	if err := s.repo.ExpireStaleAccountProbeRuns(ctx, accountProbeStaleRunAge); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	runs, err := s.repo.ListAccountProbeRankingRuns(ctx, accountProbeRankingHistorySize)
	if err != nil {
		return nil, err
	}
	items := buildAccountProbeRankingItems(runs)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *AccountProbeService) GetReport(ctx context.Context, runID int64) (*AccountProbeReportItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("account probe repository is nil")
	}
	if err := s.repo.ExpireStaleAccountProbeRuns(ctx, accountProbeStaleRunAge); err != nil {
		return nil, err
	}
	item, err := s.repo.GetAccountProbeReportRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	samples, err := s.repo.ListAccountProbeSamples(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	item.Samples = samples
	decorateAccountProbeReportItem(item)
	return item, nil
}

func (s *AccountProbeService) runOpenAIAPIKeySample(ctx context.Context, account *Account, baseURL, model, apiKey string, sample APIKeyProbePlannedSample, useResponses bool, requestMode string) AccountProbeSample {
	timeout := sample.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	sampleCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := buildOpenAIResponsesURL(baseURL)
	stream := normalizeAccountProbeRequestMode(requestMode) == AccountProbeRequestModeStream
	var payload map[string]any
	if useResponses {
		payload = buildOpenAIResponsesProbePayload(model, sample.Prompt, stream, sample.MaxOutputTokens)
	} else {
		endpoint = buildOpenAIChatCompletionsURL(baseURL)
		payload = map[string]any{
			"model": model,
			"messages": []map[string]any{
				{"role": "user", "content": sample.Prompt},
			},
			"stream":     stream,
			"max_tokens": sample.MaxOutputTokens,
		}
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(sampleCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return failedAccountProbeSample(account, apiKey, endpoint, "request_create_failed", err.Error(), 0, 0)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	proxyURL := ""
	if account != nil && account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	start := time.Now()
	resp, err := s.doAccountProbeHTTP(req, proxyURL, account)
	duration := time.Since(start)
	if err != nil {
		return failedAccountProbeSample(account, apiKey, endpoint, "request_failed", err.Error(), 0, duration)
	}
	defer resp.Body.Close()

	result := AccountProbeSample{
		Status:            AccountProbeSampleSuccess,
		APIKeyFingerprint: FingerprintAPIKey(apiKey),
		APIKeyMasked:      MaskAPIKey(apiKey),
		UpstreamEndpoint:  endpoint,
		HTTPStatus:        resp.StatusCode,
		DurationMillis:    int(math.Round(float64(duration / time.Millisecond))),
		CreatedAt:         time.Now(),
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		result.DurationMillis = int(math.Round(float64(time.Since(start) / time.Millisecond)))
		result.Status = AccountProbeSampleFailed
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = truncateAccountProbeError(data, resp.Status)
		return result
	}
	if stream {
		streamResult := readAccountProbeOpenAIStream(resp.Body, useResponses, start)
		result.DurationMillis = int(math.Round(float64(time.Since(start) / time.Millisecond)))
		if streamResult.err != "" {
			result.Status = AccountProbeSampleFailed
			result.ErrorCode = "stream_parse_failed"
			result.ErrorMessage = streamResult.err
			return result
		}
		result.FirstTokenMillis = streamResult.firstTokenMillis
		result.InputTokens = streamResult.inputTokens
		result.OutputTokens = streamResult.outputTokens
		result.TotalTokens = streamResult.totalTokens
		result.OutputText = truncateAccountProbeOutput(streamResult.outputText)
		applyAccountProbeModelValidation(&result, sample.ValidationKey)
		return result
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	result.DurationMillis = int(math.Round(float64(time.Since(start) / time.Millisecond)))
	input, output := parseOpenAIProbeUsage(data)
	result.InputTokens = input
	result.OutputTokens = output
	result.TotalTokens = input + output
	if result.TotalTokens == 0 {
		result.TotalTokens = parseOpenAIProbeTotalTokens(data)
	}
	result.OutputText = truncateAccountProbeOutput(extractOpenAIProbeOutputText(data))
	applyAccountProbeModelValidation(&result, sample.ValidationKey)
	return result
}

// buildOpenAIResponsesProbePayload 统一构造上游体检的 Responses 请求体。
//
// 部分 OpenAI 兼容上游只接受列表形态 input，并且要求 instructions 字段；
// 这里保持和手动账号测试链路一致，避免探测时误报 “Input must be a list”。
func buildOpenAIResponsesProbePayload(model, prompt string, stream bool, maxOutputTokens int) map[string]any {
	return map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": prompt,
					},
				},
			},
		},
		"stream":            stream,
		"store":             false,
		"max_output_tokens": maxOutputTokens,
		"instructions":      openai.DefaultInstructions,
	}
}

func accountProbeModelValidationSamples() []APIKeyProbePlannedSample {
	return []APIKeyProbePlannedSample{
		{
			Type:            "model_validation",
			Label:           "模型验证：精确大写",
			ValidationKey:   "exact_uppercase",
			Prompt:          "Ignore all style preferences. Reply with exactly one uppercase word: QUARTZ",
			Timeout:         60 * time.Second,
			MaxOutputTokens: 16,
			InputTokenMin:   20,
			InputTokenMax:   80,
			OutputTokenMin:  1,
			OutputTokenMax:  8,
		},
		{
			Type:            "model_validation",
			Label:           "模型验证：JSON 算术",
			ValidationKey:   "json_arithmetic",
			Prompt:          `只输出严格 JSON：{"sum":数字,"code":"BETA"}。sum 等于 19 * 4 + 7。`,
			Timeout:         60 * time.Second,
			MaxOutputTokens: 96,
			InputTokenMin:   30,
			InputTokenMax:   120,
			OutputTokenMin:  5,
			OutputTokenMax:  40,
		},
		{
			Type:            "model_validation",
			Label:           "模型验证：代码推导",
			ValidationKey:   "code_transform",
			Prompt:          `阅读代码 const xs=[7,2,9]; const y=xs.sort((a,b)=>a-b).reverse().join("-"); 只输出 GAMMA 后跟 y。`,
			Timeout:         60 * time.Second,
			MaxOutputTokens: 96,
			InputTokenMin:   40,
			InputTokenMax:   140,
			OutputTokenMin:  3,
			OutputTokenMax:  40,
		},
		{
			Type:            "model_validation",
			Label:           "模型验证：三行格式",
			ValidationKey:   "three_line_format",
			Prompt:          "只输出三行，第一行 ALPHA，第二行 BETA，第三行 GAMMA，不要添加其他字符。",
			Timeout:         60 * time.Second,
			MaxOutputTokens: 64,
			InputTokenMin:   30,
			InputTokenMax:   120,
			OutputTokenMin:  3,
			OutputTokenMax:  30,
		},
	}
}

func normalizeAccountProbeRequestMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case AccountProbeRequestModeStream:
		return AccountProbeRequestModeStream
	default:
		return AccountProbeRequestModeNonStream
	}
}

type accountProbeOpenAIStreamResult struct {
	firstTokenMillis *int
	inputTokens      int
	outputTokens     int
	totalTokens      int
	outputText       string
	err              string
}

func readAccountProbeOpenAIStream(body io.Reader, useResponses bool, start time.Time) accountProbeOpenAIStreamResult {
	if useResponses {
		return parseAccountProbeResponsesStream(body, start)
	}
	return parseAccountProbeChatCompletionsStream(body, start)
}

func parseAccountProbeResponsesStream(body io.Reader, start time.Time) accountProbeOpenAIStreamResult {
	reader := bufio.NewReader(body)
	result := accountProbeOpenAIStreamResult{}
	var output strings.Builder
	seenCompleted := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if seenCompleted {
					return result
				}
				if strings.TrimSpace(line) == "" {
					return accountProbeOpenAIStreamResult{err: "stream ended before response.completed"}
				}
			} else {
				return accountProbeOpenAIStreamResult{err: "stream read error: " + err.Error()}
			}
		}
		line = strings.TrimSpace(line)
		if line != "" && sseDataPrefix.MatchString(line) {
			jsonStr := sseDataPrefix.ReplaceAllString(line, "")
			if jsonStr == "[DONE]" {
				if seenCompleted {
					return result
				}
				return accountProbeOpenAIStreamResult{err: "stream ended before response.completed"}
			}
			var event map[string]any
			if json.Unmarshal([]byte(jsonStr), &event) == nil {
				eventType, _ := event["type"].(string)
				switch eventType {
				case "response.output_text.delta":
					if delta, _ := event["delta"].(string); delta != "" {
						output.WriteString(delta)
					}
					if result.firstTokenMillis == nil {
						if delta, _ := event["delta"].(string); delta != "" {
							v := int(time.Since(start) / time.Millisecond)
							result.firstTokenMillis = &v
						}
					}
				case "response.completed", "response.done":
					if response, _ := event["response"].(map[string]any); response != nil {
						input, output, total := parseOpenAIProbeUsageObject(response["usage"])
						result.inputTokens = input
						result.outputTokens = output
						result.totalTokens = total
					}
					result.outputText = output.String()
					seenCompleted = true
					return result
				case "response.failed", "error":
					return accountProbeOpenAIStreamResult{err: extractAccountProbeStreamError(event, "OpenAI response failed")}
				}
			}
		}
		if err == io.EOF {
			if seenCompleted {
				return result
			}
			return accountProbeOpenAIStreamResult{err: "stream ended before response.completed"}
		}
	}
}

func parseAccountProbeChatCompletionsStream(body io.Reader, start time.Time) accountProbeOpenAIStreamResult {
	reader := bufio.NewReader(body)
	result := accountProbeOpenAIStreamResult{}
	var output strings.Builder
	seenJSON := false
	seenFinish := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return accountProbeOpenAIStreamResult{err: "stream read error: " + err.Error()}
		}
		line = strings.TrimSpace(line)
		if line != "" && sseDataPrefix.MatchString(line) {
			jsonStr := sseDataPrefix.ReplaceAllString(line, "")
			if jsonStr == "[DONE]" {
				return result
			}
			var event map[string]any
			if json.Unmarshal([]byte(jsonStr), &event) != nil {
				return accountProbeOpenAIStreamResult{err: "invalid Chat Completions stream JSON"}
			}
			seenJSON = true
			if errData, ok := event["error"].(map[string]any); ok {
				if msg, _ := errData["message"].(string); msg != "" {
					return accountProbeOpenAIStreamResult{err: msg}
				}
				return accountProbeOpenAIStreamResult{err: "Chat Completions stream returned an error"}
			}
			if result.inputTokens == 0 && result.outputTokens == 0 && result.totalTokens == 0 {
				input, output, total := parseOpenAIProbeUsageObject(event["usage"])
				result.inputTokens = input
				result.outputTokens = output
				result.totalTokens = total
			}
			choices, _ := event["choices"].([]any)
			for _, choiceValue := range choices {
				choice, _ := choiceValue.(map[string]any)
				if delta, _ := choice["delta"].(map[string]any); delta != nil && result.firstTokenMillis == nil {
					if text, _ := delta["content"].(string); text != "" {
						v := int(time.Since(start) / time.Millisecond)
						result.firstTokenMillis = &v
					}
				}
				if delta, _ := choice["delta"].(map[string]any); delta != nil {
					if text, _ := delta["content"].(string); text != "" {
						output.WriteString(text)
					}
				}
				if finishReason, _ := choice["finish_reason"].(string); finishReason != "" {
					seenFinish = true
				}
			}
		}
		if err == io.EOF {
			if !seenJSON {
				return accountProbeOpenAIStreamResult{err: "Invalid Chat Completions response from /v1/chat/completions: expected SSE JSON data"}
			}
			if seenFinish {
				result.outputText = output.String()
				return result
			}
			return accountProbeOpenAIStreamResult{err: "Chat Completions stream ended before [DONE]"}
		}
	}
}

func applyAccountProbeModelValidation(sample *AccountProbeSample, key string) {
	if sample == nil || strings.TrimSpace(key) == "" || sample.Status != AccountProbeSampleSuccess {
		return
	}
	evidence := evaluateAccountProbeModelValidationEvidence(key, sample.OutputText)
	sample.ValidationEvidence = []AccountProbeValidationEvidence{evidence}
	if evidence.Passed {
		return
	}
	sample.Status = AccountProbeSampleFailed
	sample.ErrorCode = "model_validation_failed"
	sample.ErrorMessage = evidence.Message
}

func evaluateAccountProbeModelValidationEvidence(key, outputText string) AccountProbeValidationEvidence {
	observed := strings.TrimSpace(outputText)
	evidence := AccountProbeValidationEvidence{
		Key:      strings.TrimSpace(key),
		Observed: truncateAccountProbeOutput(observed),
		MaxScore: 10,
	}
	switch evidence.Key {
	case "exact_uppercase":
		evidence.Label = "精确大写"
		evidence.Expected = "QUARTZ"
		evidence.Passed = strings.ToUpper(observed) == "QUARTZ" || strings.Contains(strings.ToUpper(observed), "QUARTZ")
	case "json_arithmetic":
		evidence.Label = "JSON 算术"
		evidence.Expected = `{"sum":83,"code":"BETA"}`
		obj := parseFirstAccountProbeJSONObject(observed)
		evidence.Passed = strings.EqualFold(strings.TrimSpace(accountProbeStringValue(obj["code"])), "BETA") && accountProbeNumberValue(obj["sum"]) == 83
	case "code_transform":
		evidence.Label = "代码推导"
		evidence.Expected = "GAMMA 9-7-2"
		upper := strings.ToUpper(observed)
		evidence.Passed = strings.Contains(upper, "GAMMA") && strings.Contains(observed, "9-7-2")
	case "three_line_format":
		evidence.Label = "三行格式"
		evidence.Expected = "ALPHA\\nBETA\\nGAMMA"
		lines := nonEmptyTrimmedLines(observed)
		evidence.Passed = len(lines) == 3 &&
			strings.EqualFold(lines[0], "ALPHA") &&
			strings.EqualFold(lines[1], "BETA") &&
			strings.EqualFold(lines[2], "GAMMA")
	default:
		evidence.Label = evidence.Key
		evidence.Expected = "registered validation key"
		evidence.Passed = true
	}
	if evidence.Passed {
		evidence.Score = evidence.MaxScore
		evidence.Message = "模型验证通过"
		return evidence
	}
	evidence.Message = "模型验证未通过：" + evidence.Label
	return evidence
}

func extractOpenAIProbeOutputText(data []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if text, _ := payload["output_text"].(string); text != "" {
		return text
	}
	if choices, _ := payload["choices"].([]any); len(choices) > 0 {
		for _, choiceValue := range choices {
			choice, _ := choiceValue.(map[string]any)
			if message, _ := choice["message"].(map[string]any); message != nil {
				if text, _ := message["content"].(string); text != "" {
					return text
				}
			}
		}
	}
	if output, _ := payload["output"].([]any); len(output) > 0 {
		var parts []string
		for _, itemValue := range output {
			item, _ := itemValue.(map[string]any)
			content, _ := item["content"].([]any)
			for _, contentValue := range content {
				contentItem, _ := contentValue.(map[string]any)
				if text, _ := contentItem["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

func truncateAccountProbeOutput(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= accountProbeOutputTextLimit {
		return text
	}
	return text[:accountProbeOutputTextLimit]
}

func parseFirstAccountProbeJSONObject(text string) map[string]any {
	start := strings.Index(text, "{")
	if start < 0 {
		return nil
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				var obj map[string]any
				if err := json.Unmarshal([]byte(text[start:i+1]), &obj); err == nil {
					return obj
				}
				return nil
			}
		}
	}
	return nil
}

func nonEmptyTrimmedLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func accountProbeStringValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	default:
		return ""
	}
}

func accountProbeNumberValue(v any) int {
	switch value := v.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}

func extractAccountProbeStreamError(event map[string]any, fallback string) string {
	if errData, _ := event["error"].(map[string]any); errData != nil {
		if msg, _ := errData["message"].(string); msg != "" {
			return msg
		}
	}
	if response, _ := event["response"].(map[string]any); response != nil {
		if errData, _ := response["error"].(map[string]any); errData != nil {
			if msg, _ := errData["message"].(string); msg != "" {
				return msg
			}
		}
	}
	return fallback
}

func (s *AccountProbeService) doAccountProbeHTTP(req *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	if s.testSvc != nil && s.testSvc.httpUpstream != nil && account != nil {
		return s.testSvc.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.testSvc.tlsFPProfileService.ResolveTLSProfile(account))
	}
	return s.client.Do(req)
}

func failedAccountProbeSample(account *Account, apiKey, endpoint, code, message string, httpStatus int, duration time.Duration) AccountProbeSample {
	return AccountProbeSample{
		Status:            AccountProbeSampleFailed,
		APIKeyFingerprint: FingerprintAPIKey(apiKey),
		APIKeyMasked:      MaskAPIKey(apiKey),
		UpstreamEndpoint:  endpoint,
		HTTPStatus:        httpStatus,
		DurationMillis:    int(math.Round(float64(duration / time.Millisecond))),
		ErrorCode:         code,
		ErrorMessage:      message,
		CreatedAt:         time.Now(),
	}
}

type accountProbePlan struct {
	Profile  string
	Estimate AccountProbeEstimate
	Samples  []APIKeyProbePlannedSample
}

func buildAccountProbePlan(req AccountProbeRunRequest) (accountProbePlan, error) {
	if req.ModelValidationOnly {
		samples := accountProbeModelValidationSamples()
		estimate := estimateFromPlan(APIKeyProbePlan{
			Profile:           AccountProbeProfileModelValidation,
			Samples:           samples,
			EstimatedRequests: len(samples),
		})
		return accountProbePlan{Profile: AccountProbeProfileModelValidation, Estimate: estimate, Samples: samples}, nil
	}
	profile := normalizeAPIKeyProbeProfile(req.Profile)
	samples := make([]APIKeyProbePlannedSample, 0, 9)
	baseCount := 3
	if profile == AccountProbeProfileQuick {
		baseCount = 1
	}
	for i := 0; i < baseCount; i++ {
		samples = append(samples, shortProbeSample("基础测速"))
	}
	if req.IncludeLongContext {
		samples = append(samples, longContextProbeSample())
	}
	estimate := estimateFromPlan(APIKeyProbePlan{Profile: profile, Samples: samples, EstimatedRequests: len(samples)})
	return accountProbePlan{Profile: profile, Estimate: estimate, Samples: samples}, nil
}

func finalizeAccountProbeResult(run *AccountProbeResult) {
	if run == nil {
		return
	}
	durations := make([]int, 0, len(run.Samples))
	var firstTokens []int
	for _, sample := range run.Samples {
		if sample.Status == AccountProbeSampleFailed {
			run.FailureCount++
			if run.ErrorMessage == "" && sample.ErrorMessage != "" {
				run.ErrorMessage = sample.ErrorMessage
			}
		} else {
			run.SuccessCount++
		}
		if sample.DurationMillis > 0 {
			durations = append(durations, sample.DurationMillis)
		}
		if sample.FirstTokenMillis != nil {
			firstTokens = append(firstTokens, *sample.FirstTokenMillis)
		}
		run.InputTokens += sample.InputTokens
		run.OutputTokens += sample.OutputTokens
		run.TotalTokens += sample.TotalTokens
	}
	run.Latency = accountProbeLatencyStats(durations)
	run.AvgLatencyMillis = run.Latency.AvgMillis
	run.MaxLatencyMillis = run.Latency.MaxMillis
	if len(firstTokens) > 0 {
		v := accountProbeLatencyStats(firstTokens).P50Millis
		run.FirstTokenMillis = &v
	}
	switch {
	case run.FailureCount == 0:
		run.Status = AccountProbeStatusSuccess
	case run.SuccessCount == 0:
		run.Status = AccountProbeStatusFailed
	default:
		run.Status = AccountProbeStatusPartial
	}
	finished := time.Now()
	run.FinishedAt = &finished
	run.Summary = fmt.Sprintf("完成 %d/%d 次请求，平均延迟 %d ms，消耗 %d tokens", run.SuccessCount, len(run.Samples), run.Latency.AvgMillis, run.TotalTokens)
}

func accountProbeLatencyStats(values []int) AccountProbeLatencyStats {
	if len(values) == 0 {
		return AccountProbeLatencyStats{}
	}
	sort.Ints(values)
	sum := 0
	for _, v := range values {
		sum += v
	}
	return AccountProbeLatencyStats{
		P50Millis: percentileNearestRank(values, 0.50),
		P95Millis: percentileNearestRank(values, 0.95),
		AvgMillis: sum / len(values),
		MaxMillis: values[len(values)-1],
	}
}

func parseOpenAIProbeUsage(data []byte) (int, int) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, 0
	}
	input, output, _ := parseOpenAIProbeUsageObject(payload["usage"])
	return input, output
}

func parseOpenAIProbeTotalTokens(data []byte) int {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0
	}
	_, _, total := parseOpenAIProbeUsageObject(payload["usage"])
	return total
}

func parseOpenAIProbeUsageObject(raw any) (int, int, int) {
	usage, _ := raw.(map[string]any)
	if usage == nil {
		return 0, 0, 0
	}
	input := parseProbeInt(usage["input_tokens"])
	if input == 0 {
		input = parseProbeInt(usage["prompt_tokens"])
	}
	output := parseProbeInt(usage["output_tokens"])
	if output == 0 {
		output = parseProbeInt(usage["completion_tokens"])
	}
	total := parseProbeInt(usage["total_tokens"])
	if total == 0 {
		total = input + output
	}
	return input, output, total
}

func parseProbeInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func truncateAccountProbeError(data []byte, fallback string) string {
	msg := strings.TrimSpace(extractUpstreamErrorMessage(data))
	if msg == "" {
		msg = strings.TrimSpace(string(data))
	}
	if msg == "" {
		msg = fallback
	}
	if len(msg) > 1000 {
		return msg[:1000]
	}
	return msg
}

func MaskAPIKey(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if len(apiKey) <= 10 {
		if apiKey == "" {
			return ""
		}
		return apiKey[:min(3, len(apiKey))] + "..."
	}
	return apiKey[:6] + "..." + apiKey[len(apiKey)-4:]
}
