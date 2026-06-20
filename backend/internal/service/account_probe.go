package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

const (
	AccountProbeProfileQuick           = APIKeyProbeProfileQuick
	AccountProbeProfileStandard        = APIKeyProbeProfileStandard
	AccountProbeProfileModelValidation = "model_validation"

	AccountProbeSourceSelfValidation = "self_validation"
	AccountProbeSourceBazaarLinkAPI  = "bazaarlink_api"

	AccountProbeStatusSuccess = APIKeyProbeStatusSuccess
	AccountProbeStatusPartial = APIKeyProbeStatusPartial
	AccountProbeStatusFailed  = APIKeyProbeStatusFailed
	AccountProbeStatusRunning = APIKeyProbeStatusRunning

	AccountProbeSampleSuccess = APIKeyProbeSampleSuccess
	AccountProbeSampleFailed  = APIKeyProbeSampleFailed
	AccountProbeSampleWarning = APIKeyProbeSampleWarning

	AccountProbeRequestModeNonStream = "non_stream"
	AccountProbeRequestModeStream    = "stream"
)

const (
	accountProbePersistenceTimeout  = 5 * time.Second
	accountProbeRetryDelay          = 2 * time.Second
	accountProbeModelRetryDelay     = 300 * time.Millisecond
	accountProbeModelMaxAttempts    = 3
	accountProbeDistributionRuns    = 5
	accountProbeStaleRunAge         = 12 * time.Minute
	accountProbeRankingHistorySize  = 20
	accountProbeOutputTextLimit     = 1000
	accountProbeTranscriptTextLimit = 20000
)

type AccountProbeRunRequest struct {
	AccountID             int64  `json:"-"`
	Profile               string `json:"mode"`
	Model                 string `json:"model"`
	IncludeCodexStability bool   `json:"codex_stability"`
	IncludeLongContext    bool   `json:"long_context"`
	RequestMode           string `json:"request_mode"`
	TrustedComparisonID   int64  `json:"trusted_comparison_account_id,omitempty"`
	ModelValidationOnly   bool   `json:"-"`
	// RepairSchedulingPoolState 只给调度池人工测验使用；普通上游体检成功不能修复账号调度或池内健康状态。
	RepairSchedulingPoolState bool `json:"-"`
}

type BazaarLinkProbeMode string

const (
	BazaarLinkProbeModeQuick BazaarLinkProbeMode = "quick"
	BazaarLinkProbeModeFull  BazaarLinkProbeMode = "full"
)

type BazaarLinkProbeRunRequest struct {
	AccountID int64               `json:"account_id"`
	Model     string              `json:"model"`
	Mode      BazaarLinkProbeMode `json:"mode"`
}

type AccountProbeEstimate = APIKeyProbeEstimate
type AccountProbeLatencyStats = APIKeyProbeLatencyStats

// AccountProbeValidationEvidence 记录单个模型行为探针的结构化判定依据。
type AccountProbeValidationEvidence struct {
	Key                string   `json:"key"`
	Label              string   `json:"label"`
	Expected           string   `json:"expected"`
	Observed           string   `json:"observed"`
	Passed             bool     `json:"passed"`
	Score              int      `json:"score"`
	DisplayScore       *float64 `json:"display_score,omitempty"`
	MaxScore           int      `json:"max_score"`
	Message            string   `json:"message,omitempty"`
	Category           string   `json:"category,omitempty"`
	Severity           string   `json:"severity,omitempty"`
	AttemptCount       int      `json:"attempt_count,omitempty"`
	RetryAttemptCount  int      `json:"retry_attempt_count,omitempty"`
	AttemptStatusCodes []int    `json:"attempt_status_codes,omitempty"`
	ResponseModel      string   `json:"response_model,omitempty"`
	ExpectedModel      string   `json:"expected_model,omitempty"`
	TrustedAccountID   int64    `json:"trusted_account_id,omitempty"`
	SimilarityPercent  int      `json:"similarity_percent,omitempty"`
	PairCoverage       int      `json:"pair_coverage_percent,omitempty"`
	TargetPassRate     int      `json:"target_pass_rate_percent,omitempty"`
	TrustedPassRate    int      `json:"trusted_pass_rate_percent,omitempty"`
}

type AccountProbeResult struct {
	ID                    int64                    `json:"id"`
	AccountID             int64                    `json:"account_id"`
	Profile               string                   `json:"mode"`
	ProbeSource           string                   `json:"probe_source"`
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
	// RequestPrompt 保存发送给上游模型的原始 prompt，便于在体检报告里复盘输入。
	RequestPrompt string `json:"request_prompt,omitempty"`
	// RequestBody 保存不含 Authorization 的上游请求 JSON 或 GET 请求摘要。
	RequestBody string `json:"request_body,omitempty"`
	// ResponseBody 保存上游返回体或流式 completed response，用于详情弹窗核对输出。
	ResponseBody string    `json:"response_body,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
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
	DisplayScore *float64 `json:"display_score,omitempty"`
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
	accounts         AccountProbeAccountReader
	repo             AccountProbeRepository
	client           AccountProbeHTTPClient
	testSvc          *AccountTestService
	health           *OpenAIPathHealthTracker
	rateLimitService *RateLimitService
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

func (s *AccountProbeService) SetRateLimitService(rateLimitService *RateLimitService) {
	if s == nil {
		return
	}
	s.rateLimitService = rateLimitService
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
		ProbeSource:           AccountProbeSourceSelfValidation,
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
	if strings.TrimSpace(run.ProbeSource) == "" {
		run.ProbeSource = AccountProbeSourceSelfValidation
	}
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
		if !req.ModelValidationOnly && !strings.EqualFold(plan.Profile, AccountProbeProfileModelValidation) {
			s.recordProbePathHealth(account, baseURL, sample)
		}
		sample.RunID = run.ID
		sample.RequestIndex = idx + 1
		sample.Type = planned.Type
		sample.Label = planned.Label
		sample.Model = accountProbeSampleRequestModel(account, model, planned)
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
	if ctx.Err() == nil && req.ModelValidationOnly && req.TrustedComparisonID > 0 {
		trustedSamples, err := s.runAccountProbeTrustedComparison(ctx, run.ID, len(run.Samples)+1, account, keys, baseURLs, model, useResponses, run.RequestMode, req.TrustedComparisonID, run.Samples)
		if err != nil {
			trustedSamples = []AccountProbeSample{failedAccountProbeTrustedComparisonSample(run.ID, len(run.Samples)+1, model, req.TrustedComparisonID, err)}
		}
		for _, sample := range trustedSamples {
			if s.repo != nil {
				persistCtx, cancel := context.WithTimeout(context.Background(), accountProbePersistenceTimeout)
				err := s.repo.SaveAccountProbeSample(persistCtx, sample)
				cancel()
				if err != nil {
					return s.failExistingRun(ctx, run, err.Error()), err
				}
			}
			run.Samples = append(run.Samples, sample)
		}
	}
	if len(run.Samples) > run.RequestCount {
		run.RequestCount = len(run.Samples)
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
	s.recordAccountProbeOutcome(account, run, req.RepairSchedulingPoolState)
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

func (s *AccountProbeService) recordAccountProbeOutcome(account *Account, run AccountProbeResult, repairSchedulingPoolState bool) {
	if s == nil || s.rateLimitService == nil || account == nil {
		return
	}
	outcome := accountProbeOutcomeFromRun(account, run, repairSchedulingPoolState)
	persistCtx, cancel := context.WithTimeout(context.Background(), accountProbePersistenceTimeout)
	defer cancel()
	if _, err := s.rateLimitService.RecordAccountProbeOutcome(persistCtx, outcome); err != nil {
		slog.Warn("account_probe_outcome_record_failed", "account_id", account.ID, "run_id", run.ID, "status", run.Status, "error", err)
	}
}

func accountProbeOutcomeFromRun(account *Account, run AccountProbeResult, repairSchedulingPoolState bool) AccountProbeOutcome {
	success := run.Status == AccountProbeStatusSuccess
	latencyMs := run.AvgLatencyMillis
	if latencyMs <= 0 {
		latencyMs = run.Latency.AvgMillis
	}
	var latencyPtr *int
	if latencyMs >= 0 {
		latencyPtr = &latencyMs
	}
	httpStatus, reason, errorMessage, keyFingerprint := accountProbeFailureSummary(run)
	observedAt := time.Now()
	if run.FinishedAt != nil {
		observedAt = *run.FinishedAt
	}
	source := AccountProbeOutcomeSourceAccountProbe
	if repairSchedulingPoolState {
		source = AccountProbeOutcomeSourceManualTest
	}
	return AccountProbeOutcome{
		AccountID:      account.ID,
		Account:        account,
		Source:         source,
		Success:        success,
		ErrorMessage:   errorMessage,
		HTTPStatus:     httpStatus,
		Reason:         reason,
		LatencyMs:      latencyPtr,
		FirstTokenMs:   run.FirstTokenMillis,
		KeyFingerprint: keyFingerprint,
		ObservedAt:     observedAt,
	}
}

func accountProbeFailureSummary(run AccountProbeResult) (int, string, string, string) {
	if run.Status == AccountProbeStatusSuccess {
		return 0, "probe_success", "", ""
	}
	for _, sample := range run.Samples {
		if sample.Status != AccountProbeSampleFailed {
			continue
		}
		reason := strings.TrimSpace(sample.ErrorCode)
		if reason == "" && sample.HTTPStatus > 0 {
			reason = fmt.Sprintf("http_%d", sample.HTTPStatus)
		}
		return sample.HTTPStatus, reason, firstNonEmptyString(sample.ErrorMessage, run.ErrorMessage, reason), sample.APIKeyFingerprint
	}
	return 0, strings.TrimSpace(run.Status), strings.TrimSpace(run.ErrorMessage), ""
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
	maxAttempts := 2
	if isAccountProbeModelValidationSample(sample) {
		maxAttempts = accountProbeModelMaxAttempts
		useResponses = true
	}
	var attempts []AccountProbeSample
	var result AccountProbeSample
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result = s.runOpenAIAPIKeySample(ctx, account, baseURL, model, apiKey, sample, useResponses, requestMode)
		attempts = append(attempts, result)
		if !shouldRetryAccountProbeSample(result, sample) || ctx.Err() != nil || attempt >= maxAttempts {
			break
		}
		if !waitAccountProbeRetryDelay(ctx, sample, attempt) {
			break
		}
	}
	if isAccountProbeModelValidationSample(sample) {
		attachAccountProbeRetryEvidence(&result, sample, attempts)
	}
	return result
}

type accountProbeDistributionPair struct {
	key     string
	target  AccountProbeSample
	trusted AccountProbeSample
}

type accountProbeDistributionPairScore struct {
	key                     string
	successful              bool
	targetConstraintPassed  bool
	trustedConstraintPassed bool
	similarity              float64
	lengthRatio             float64
	usageRatio              *float64
}

func (s *AccountProbeService) runAccountProbeTrustedComparison(ctx context.Context, runID int64, startIndex int, target *Account, targetKeys, targetBaseURLs []string, targetModel string, targetUseResponses bool, requestMode string, trustedAccountID int64, targetCoreSamples []AccountProbeSample) ([]AccountProbeSample, error) {
	trusted, trustedKeys, trustedModel, trustedBaseURLs, trustedUseResponses, err := s.prepareTrustedComparisonTarget(ctx, trustedAccountID, targetModel)
	if err != nil {
		return nil, err
	}
	trustedCorePlans := accountProbeTrustedCoreSamplesForModel(trustedModel)
	targetPlans := accountProbeTrustedDistributionSamplesForModel(targetModel, "target")
	trustedPlans := accountProbeTrustedDistributionSamplesForModel(trustedModel, "trusted")
	samples := make([]AccountProbeSample, 0, len(trustedCorePlans)+len(targetPlans)+len(trustedPlans)+2)
	trustedCoreSamples := make([]AccountProbeSample, 0, len(trustedCorePlans))
	pairs := make([]accountProbeDistributionPair, 0, len(targetPlans))
	index := startIndex
	for i, trustedPlan := range trustedCorePlans {
		if ctx.Err() != nil {
			break
		}
		trustedSample := s.runOpenAIAPIKeySampleWithRetry(ctx, trusted, trustedBaseURLs[i%len(trustedBaseURLs)], trustedModel, trustedKeys[i%len(trustedKeys)], trustedPlan, trustedUseResponses, requestMode)
		trustedSample.RunID = runID
		trustedSample.RequestIndex = index
		trustedSample.Type = trustedPlan.Type
		trustedSample.Label = trustedPlan.Label
		trustedSample.Model = accountProbeSampleRequestModel(trusted, trustedModel, trustedPlan)
		samples = append(samples, trustedSample)
		trustedCoreSamples = append(trustedCoreSamples, trustedSample)
		index++
	}
	coreSummary := accountProbeTrustedCoreComparisonSample(runID, index, targetModel, trusted.ID, targetCoreSamples, trustedCoreSamples)
	samples = append(samples, coreSummary)
	index++
	if ctx.Err() != nil || !accountProbeTrustedCoreComparable(targetCoreSamples, trustedCoreSamples) {
		return samples, nil
	}
	for i := range targetPlans {
		if ctx.Err() != nil {
			break
		}
		targetPlan := targetPlans[i]
		targetSample := s.runOpenAIAPIKeySampleWithRetry(ctx, target, targetBaseURLs[i%len(targetBaseURLs)], targetModel, targetKeys[i%len(targetKeys)], targetPlan, targetUseResponses, requestMode)
		targetSample.RunID = runID
		targetSample.RequestIndex = index
		targetSample.Type = targetPlan.Type
		targetSample.Label = targetPlan.Label
		targetSample.Model = accountProbeSampleRequestModel(target, targetModel, targetPlan)
		samples = append(samples, targetSample)
		index++

		trustedPlan := trustedPlans[i]
		trustedSample := s.runOpenAIAPIKeySampleWithRetry(ctx, trusted, trustedBaseURLs[i%len(trustedBaseURLs)], trustedModel, trustedKeys[i%len(trustedKeys)], trustedPlan, trustedUseResponses, requestMode)
		trustedSample.RunID = runID
		trustedSample.RequestIndex = index
		trustedSample.Type = trustedPlan.Type
		trustedSample.Label = trustedPlan.Label
		trustedSample.Model = accountProbeSampleRequestModel(trusted, trustedModel, trustedPlan)
		samples = append(samples, trustedSample)
		index++

		pairs = append(pairs, accountProbeDistributionPair{
			key:     targetPlan.DistributionKey,
			target:  targetSample,
			trusted: trustedSample,
		})
	}
	summary := accountProbeTrustedComparisonSummarySample(runID, index, targetModel, trusted.ID, pairs)
	samples = append(samples, summary)
	return samples, nil
}

func failedAccountProbeTrustedComparisonSample(runID int64, index int, model string, trustedAccountID int64, err error) AccountProbeSample {
	message := "可信对比账号不可用"
	if err != nil {
		message = err.Error()
	}
	evidence := AccountProbeValidationEvidence{
		Key:              "trusted_distribution_similarity",
		Label:            "可信对比分布相似度",
		Expected:         "可信账号可用并完成分布对照",
		Observed:         message,
		Passed:           false,
		Score:            0,
		MaxScore:         15,
		Message:          message,
		Category:         "distribution_similarity",
		Severity:         "critical",
		ExpectedModel:    model,
		TrustedAccountID: trustedAccountID,
	}
	return AccountProbeSample{
		RunID:              runID,
		RequestIndex:       index,
		Type:               "trusted_comparison",
		Label:              "可信对比：分布相似度",
		Status:             AccountProbeSampleFailed,
		Model:              model,
		OutputText:         message,
		ValidationEvidence: []AccountProbeValidationEvidence{evidence},
		ErrorCode:          "trusted_comparison_unavailable",
		ErrorMessage:       message,
		CreatedAt:          time.Now(),
	}
}

func accountProbeTrustedCoreComparisonSample(runID int64, index int, model string, trustedAccountID int64, targetSamples, trustedSamples []AccountProbeSample) AccountProbeSample {
	evidence := evaluateAccountProbeTrustedCoreComparison(model, trustedAccountID, targetSamples, trustedSamples)
	status := AccountProbeSampleFailed
	if evidence.Passed {
		status = AccountProbeSampleSuccess
	} else if evidence.Severity == "warning" {
		status = AccountProbeSampleWarning
	}
	message := evidence.Message
	if message == "" {
		message = evidence.Observed
	}
	sample := AccountProbeSample{
		RunID:              runID,
		RequestIndex:       index,
		Type:               "trusted_comparison",
		Label:              "可信对比：核心探针可比性",
		Status:             status,
		Model:              model,
		OutputText:         message,
		ValidationEvidence: []AccountProbeValidationEvidence{evidence},
		CreatedAt:          time.Now(),
	}
	if status == AccountProbeSampleFailed {
		sample.ErrorCode = "trusted_comparison_core_failed"
		sample.ErrorMessage = message
	}
	for _, trustedSample := range trustedSamples {
		sample.DurationMillis += trustedSample.DurationMillis
		sample.InputTokens += trustedSample.InputTokens
		sample.OutputTokens += trustedSample.OutputTokens
		sample.TotalTokens += trustedSample.TotalTokens
	}
	return sample
}

func evaluateAccountProbeTrustedCoreComparison(model string, trustedAccountID int64, targetSamples, trustedSamples []AccountProbeSample) AccountProbeValidationEvidence {
	targetBasicPassed := accountProbeValidationEvidencePassed(targetSamples, "responses_basic")
	trustedBasicPassed := accountProbeValidationEvidencePassed(trustedSamples, "responses_basic")
	targetBehaviorPassed := accountProbeValidationCategoryPassed(targetSamples, "behavior_probe")
	trustedBehaviorPassed := accountProbeValidationCategoryPassed(trustedSamples, "behavior_probe")
	targetOK := targetBasicPassed && targetBehaviorPassed
	trustedOK := trustedBasicPassed && trustedBehaviorPassed
	score := 0
	passed := false
	severity := "critical"
	message := "可信对比未形成完整可比结果"
	if targetOK && trustedOK {
		score = 10
		passed = true
		severity = "info"
		message = "目标链路和可信对比链路均完成核心探针"
	} else if trustedOK {
		score = 4
		severity = "warning"
		message = "可信对比链路可用，但目标核心探针未形成完整可比结果"
	}
	observed := fmt.Sprintf("target_basic=%t, target_behavior=%t, trusted_basic=%t, trusted_behavior=%t", targetBasicPassed, targetBehaviorPassed, trustedBasicPassed, trustedBehaviorPassed)
	return AccountProbeValidationEvidence{
		Key:              "trusted_comparison_core",
		Label:            "可信对比核心探针",
		Expected:         "目标账号与可信账号均完成 Responses 基础和行为探针",
		Observed:         observed,
		Passed:           passed,
		Score:            score,
		MaxScore:         10,
		Message:          message,
		Category:         "trusted_comparison",
		Severity:         severity,
		ExpectedModel:    model,
		TrustedAccountID: trustedAccountID,
		TargetPassRate:   percentMetric(ratioFloat64(boolScore(targetBasicPassed)+boolScore(targetBehaviorPassed), 2)),
		TrustedPassRate:  percentMetric(ratioFloat64(boolScore(trustedBasicPassed)+boolScore(trustedBehaviorPassed), 2)),
	}
}

func accountProbeTrustedCoreComparable(targetSamples, trustedSamples []AccountProbeSample) bool {
	return accountProbeValidationEvidencePassed(targetSamples, "responses_basic") &&
		accountProbeValidationCategoryPassed(targetSamples, "behavior_probe") &&
		accountProbeValidationEvidencePassed(trustedSamples, "responses_basic") &&
		accountProbeValidationCategoryPassed(trustedSamples, "behavior_probe")
}

func accountProbeValidationEvidencePassed(samples []AccountProbeSample, key string) bool {
	for _, sample := range samples {
		for _, evidence := range sample.ValidationEvidence {
			if strings.EqualFold(evidence.Key, key) && evidence.Passed {
				return true
			}
		}
	}
	return false
}

func accountProbeValidationCategoryPassed(samples []AccountProbeSample, category string) bool {
	for _, sample := range samples {
		for _, evidence := range sample.ValidationEvidence {
			if strings.EqualFold(evidence.Category, category) && evidence.Passed {
				return true
			}
		}
	}
	return false
}

func accountProbeTrustedComparisonSummarySample(runID int64, index int, model string, trustedAccountID int64, pairs []accountProbeDistributionPair) AccountProbeSample {
	evidence := evaluateAccountProbeDistributionSimilarity(pairs, model, trustedAccountID)
	status := AccountProbeSampleFailed
	if evidence.Passed {
		status = AccountProbeSampleSuccess
	} else if evidence.Severity == "warning" {
		status = AccountProbeSampleWarning
	}
	message := evidence.Message
	if message == "" {
		message = evidence.Observed
	}
	sample := AccountProbeSample{
		RunID:              runID,
		RequestIndex:       index,
		Type:               "trusted_comparison",
		Label:              "可信对比：分布相似度",
		Status:             status,
		Model:              model,
		OutputText:         message,
		ValidationEvidence: []AccountProbeValidationEvidence{evidence},
		CreatedAt:          time.Now(),
	}
	if status == AccountProbeSampleFailed {
		sample.ErrorCode = "trusted_distribution_similarity_failed"
		sample.ErrorMessage = message
	}
	for _, pair := range pairs {
		sample.DurationMillis += pair.target.DurationMillis + pair.trusted.DurationMillis
		sample.InputTokens += pair.target.InputTokens + pair.trusted.InputTokens
		sample.OutputTokens += pair.target.OutputTokens + pair.trusted.OutputTokens
		sample.TotalTokens += pair.target.TotalTokens + pair.trusted.TotalTokens
	}
	return sample
}

func evaluateAccountProbeDistributionSimilarity(pairs []accountProbeDistributionPair, model string, trustedAccountID int64) AccountProbeValidationEvidence {
	pairScores := make([]accountProbeDistributionPairScore, 0, len(pairs))
	for _, pair := range pairs {
		pairScores = append(pairScores, accountProbeDistributionPairScoreForPair(pair))
	}
	successfulPairCount := 0
	targetConstraintScores := make([]float64, 0, len(pairScores))
	trustedConstraintScores := make([]float64, 0, len(pairScores))
	similarities := make([]float64, 0, len(pairScores))
	lengthRatios := make([]float64, 0, len(pairScores))
	usageRatios := make([]float64, 0, len(pairScores))
	for _, score := range pairScores {
		if score.successful {
			successfulPairCount++
		}
		targetConstraintScores = append(targetConstraintScores, boolScore(score.targetConstraintPassed))
		trustedConstraintScores = append(trustedConstraintScores, boolScore(score.trustedConstraintPassed))
		similarities = append(similarities, score.similarity)
		lengthRatios = append(lengthRatios, score.lengthRatio)
		if score.usageRatio != nil {
			usageRatios = append(usageRatios, *score.usageRatio)
		}
	}
	pairCoverage := ratioFloat64(float64(successfulPairCount), float64(len(pairScores)))
	targetConstraintRate := averageFloat64(targetConstraintScores)
	trustedConstraintRate := averageFloat64(trustedConstraintScores)
	averageSimilarity := averageFloat64(similarities)
	averageLengthRatio := averageFloat64(lengthRatios)
	averageUsageRatio := averageFloat64(usageRatios)
	similarityScore := 0.35*pairCoverage + 0.25*targetConstraintRate + 0.25*averageSimilarity + 0.1*averageLengthRatio + 0.05*averageUsageRatio
	score := clampInt(int(math.Round(similarityScore*15)), 0, 15)
	trustedLooksHealthy := trustedConstraintRate >= 0.7 && pairCoverage >= 0.7
	targetLooksDivergent := targetConstraintRate < 0.55 || averageSimilarity < 0.25 || averageLengthRatio < 0.35
	passed := trustedLooksHealthy && !targetLooksDivergent && score >= 12
	severity := "critical"
	message := "目标链路与可信对比链路的隐藏分布探针差异明显，本项扣分"
	if passed {
		severity = "info"
		message = "目标链路与可信对比链路的隐藏分布探针相似度正常"
	} else if score >= 8 {
		severity = "warning"
		message = "目标链路与可信对比链路存在轻微分布差异，建议结合多次检测观察"
	}
	observed := fmt.Sprintf("pairs=%d/%d, similarity=%.3f, target=%.3f, trusted=%.3f, length=%.3f, usage=%.3f", successfulPairCount, len(pairScores), averageSimilarity, targetConstraintRate, trustedConstraintRate, averageLengthRatio, averageUsageRatio)
	return AccountProbeValidationEvidence{
		Key:               "trusted_distribution_similarity",
		Label:             "可信对比分布相似度",
		Expected:          "目标账号与可信账号分布探针相似",
		Observed:          observed,
		Passed:            passed,
		Score:             score,
		MaxScore:          15,
		Message:           message,
		Category:          "distribution_similarity",
		Severity:          severity,
		ExpectedModel:     model,
		TrustedAccountID:  trustedAccountID,
		SimilarityPercent: percentMetric(averageSimilarity),
		PairCoverage:      percentMetric(pairCoverage),
		TargetPassRate:    percentMetric(targetConstraintRate),
		TrustedPassRate:   percentMetric(trustedConstraintRate),
	}
}

func accountProbeDistributionPairScoreForPair(pair accountProbeDistributionPair) accountProbeDistributionPairScore {
	targetText := pair.target.OutputText
	trustedText := pair.trusted.OutputText
	var usageRatio *float64
	if pair.target.TotalTokens > 0 && pair.trusted.TotalTokens > 0 {
		value := boundedRatioFloat64(float64(pair.target.TotalTokens), float64(pair.trusted.TotalTokens))
		usageRatio = &value
	}
	return accountProbeDistributionPairScore{
		key:                     pair.key,
		successful:              pair.target.Status != AccountProbeSampleFailed && pair.trusted.Status != AccountProbeSampleFailed,
		targetConstraintPassed:  pair.target.Status != AccountProbeSampleFailed && accountProbeDistributionConstraintPassed(pair.key, targetText),
		trustedConstraintPassed: pair.trusted.Status != AccountProbeSampleFailed && accountProbeDistributionConstraintPassed(pair.key, trustedText),
		similarity:              textSimilarity(targetText, trustedText),
		lengthRatio:             boundedRatioFloat64(float64(len([]rune(targetText))), float64(len([]rune(trustedText)))),
		usageRatio:              usageRatio,
	}
}

func shouldRetryAccountProbeSample(result AccountProbeSample, planned APIKeyProbePlannedSample) bool {
	if result.Status != AccountProbeSampleFailed {
		return false
	}
	if isAccountProbeModelValidationSample(planned) {
		return result.ErrorCode != "model_validation_failed"
	}
	if result.ErrorCode == "request_failed" && isTransientAccountProbeError(result.ErrorMessage) {
		return true
	}
	switch result.HTTPStatus {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, 522, 524:
		return true
	}
	return false
}

func waitAccountProbeRetryDelay(ctx context.Context, sample APIKeyProbePlannedSample, attempt int) bool {
	delay := accountProbeRetryDelay
	if isAccountProbeModelValidationSample(sample) {
		delay = time.Duration(attempt) * accountProbeModelRetryDelay
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < delay {
		delay = 0
	}
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
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
	if req.ModelValidationOnly && req.TrustedComparisonID > 0 {
		if req.TrustedComparisonID == account.ID {
			return nil, nil, accountProbePlan{}, "", nil, false, fmt.Errorf("trusted comparison account cannot be the same as target account")
		}
		if _, _, _, _, _, err := s.prepareTrustedComparisonTarget(ctx, req.TrustedComparisonID, req.Model); err != nil {
			return nil, nil, accountProbePlan{}, "", nil, false, fmt.Errorf("trusted comparison account unavailable: %w", err)
		}
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = openai.DefaultTestModel
	}
	model = account.GetMappedModel(model)
	req.Model = model
	plan, err := buildAccountProbePlan(req)
	if err != nil {
		return nil, nil, accountProbePlan{}, "", nil, false, err
	}
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

	useResponses := openai_compat.ShouldUseResponsesAPI(account.Extra) || req.ModelValidationOnly
	return account, keys, plan, model, baseURLs, useResponses, nil
}

func (s *AccountProbeService) prepareTrustedComparisonTarget(ctx context.Context, accountID int64, model string) (*Account, []string, string, []string, bool, error) {
	if s == nil || s.accounts == nil {
		return nil, nil, "", nil, false, fmt.Errorf("account probe account repository is nil")
	}
	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return nil, nil, "", nil, false, err
	}
	if account == nil {
		return nil, nil, "", nil, false, ErrAccountNotFound
	}
	if !account.IsOpenAIApiKey() {
		return nil, nil, "", nil, false, fmt.Errorf("only openai api key accounts support trusted comparison")
	}
	keys := account.GetAPIKeys()
	if len(keys) == 0 {
		if key := strings.TrimSpace(account.GetOpenAIApiKey()); key != "" {
			keys = []string{key}
		}
	}
	if len(keys) == 0 {
		return nil, nil, "", nil, false, fmt.Errorf("no api key available")
	}
	requestModel := strings.TrimSpace(model)
	if requestModel == "" {
		requestModel = openai.DefaultTestModel
	}
	requestModel = account.GetMappedModel(requestModel)
	baseURLs := account.GetOpenAIRequestBaseURLs()
	if len(baseURLs) == 0 {
		baseURLs = []string{"https://api.openai.com"}
	}
	if s.testSvc != nil {
		normalizedBaseURLs := make([]string, 0, len(baseURLs))
		for _, baseURL := range baseURLs {
			normalized, err := s.testSvc.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return nil, nil, "", nil, false, fmt.Errorf("invalid base URL: %w", err)
			}
			normalizedBaseURLs = append(normalizedBaseURLs, normalized)
		}
		baseURLs = normalizedBaseURLs
	}
	return account, keys, requestModel, baseURLs, true, nil
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
		loadedSamples := false
		if accountProbeReportItemNeedsSamples(items[i]) {
			samples, err := s.repo.ListAccountProbeSamples(ctx, items[i].ID)
			if err != nil {
				return AccountProbeReportPage{}, err
			}
			items[i].Samples = samples
			loadedSamples = true
		}
		decorateAccountProbeReportItem(&items[i])
		if loadedSamples {
			items[i].Samples = nil
		}
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

	requestModel := accountProbeSampleRequestModel(account, model, sample)
	if sample.ExpectedModel == "" || sample.PairedModel != "" {
		sample.ExpectedModel = requestModel
	}
	endpoint := buildOpenAIAccountResponsesURL(baseURL, account)
	method := http.MethodPost
	stream := normalizeAccountProbeRequestMode(requestMode) == AccountProbeRequestModeStream
	if sample.RequestMode != "" {
		stream = normalizeAccountProbeRequestMode(sample.RequestMode) == AccountProbeRequestModeStream
	}
	var payload map[string]any
	if sample.ModelCatalog {
		method = http.MethodGet
		endpoint = buildOpenAIModelsURL(baseURL)
	} else if useResponses {
		payload = buildOpenAIResponsesProbePayloadForSample(requestModel, sample, stream)
	} else {
		endpoint = buildOpenAIChatCompletionsURL(baseURL)
		payload = map[string]any{
			"model": requestModel,
			"messages": []map[string]any{
				{"role": "user", "content": sample.Prompt},
			},
			"stream":     stream,
			"max_tokens": sample.MaxOutputTokens,
		}
	}
	requestPrompt := strings.TrimSpace(sample.Prompt)
	requestBody := accountProbeRequestBodyForDisplay(method, endpoint, requestModel, stream, payload)
	var body io.Reader
	var wireBody []byte
	if payload != nil {
		data, _ := json.Marshal(payload)
		wireBody = data
		requestBody = truncateAccountProbeTranscript(string(data))
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(sampleCtx, method, endpoint, body)
	if err != nil {
		result := failedAccountProbeSample(account, apiKey, endpoint, "request_create_failed", err.Error(), 0, 0)
		result.RequestPrompt = requestPrompt
		result.RequestBody = requestBody
		return result
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	if account.IsOpenAICodexCLISimulationEnabled() && payload != nil {
		applyOpenAICodexSyntheticClientHeaders(req, wireBody, account)
	}

	proxyURL := ""
	if account != nil && account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	start := time.Now()
	resp, err := s.doAccountProbeHTTP(req, proxyURL, account)
	duration := time.Since(start)
	if err != nil {
		result := failedAccountProbeSample(account, apiKey, endpoint, "request_failed", err.Error(), 0, duration)
		result.RequestPrompt = requestPrompt
		result.RequestBody = requestBody
		return result
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
		Model:             requestModel,
		RequestPrompt:     requestPrompt,
		RequestBody:       requestBody,
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		result.DurationMillis = int(math.Round(float64(time.Since(start) / time.Millisecond)))
		result.Status = AccountProbeSampleFailed
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = truncateAccountProbeError(data, resp.Status)
		result.ResponseBody = truncateAccountProbeTranscript(string(data))
		applyFailedAccountProbeModelValidation(&result, sample)
		return result
	}
	if sample.ModelCatalog {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		result.DurationMillis = int(math.Round(float64(time.Since(start) / time.Millisecond)))
		result.OutputText = truncateAccountProbeOutput(string(data))
		result.ResponseBody = truncateAccountProbeTranscript(string(data))
		applyAccountProbeModelValidation(&result, sample, data, "")
		return result
	}
	if stream {
		streamResult := readAccountProbeOpenAIStream(resp.Body, useResponses, start)
		result.DurationMillis = int(math.Round(float64(time.Since(start) / time.Millisecond)))
		if streamResult.err != "" {
			result.Status = AccountProbeSampleFailed
			result.ErrorCode = "stream_parse_failed"
			result.ErrorMessage = streamResult.err
			if len(streamResult.responseBody) > 0 {
				result.ResponseBody = truncateAccountProbeTranscript(string(streamResult.responseBody))
			}
			return result
		}
		result.FirstTokenMillis = streamResult.firstTokenMillis
		result.InputTokens = streamResult.inputTokens
		result.OutputTokens = streamResult.outputTokens
		result.TotalTokens = streamResult.totalTokens
		result.OutputText = truncateAccountProbeOutput(streamResult.outputText)
		result.ResponseBody = truncateAccountProbeTranscript(string(streamResult.responseBody))
		applyAccountProbeModelValidation(&result, sample, streamResult.responseBody, streamResult.responseModel)
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
	result.ResponseBody = truncateAccountProbeTranscript(string(data))
	applyAccountProbeModelValidation(&result, sample, data, extractOpenAIProbeResponseModel(data))
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

func buildOpenAIResponsesProbePayloadForSample(model string, sample APIKeyProbePlannedSample, stream bool) map[string]any {
	payload := buildOpenAIResponsesProbePayload(model, sample.Prompt, stream, sample.MaxOutputTokens)
	payload["instructions"] = "You are a model capability checker. Follow the requested output exactly."
	if sample.Structured {
		payload["text"] = map[string]any{
			"format": map[string]any{
				"type": "json_schema",
				"name": "model_check_structured_output",
				"schema": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"status": map[string]any{"type": "string"},
						"value":  map[string]any{"type": "number"},
					},
					"required": []string{"status", "value"},
				},
				"strict": true,
			},
		}
	}
	if sample.ToolCalling {
		payload["tools"] = []map[string]any{
			{
				"type":        "function",
				"name":        "record_model_check",
				"description": "Record a model check marker.",
				"parameters": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"code":  map[string]any{"type": "string"},
						"count": map[string]any{"type": "number"},
					},
					"required": []string{"code", "count"},
				},
			},
		}
		payload["tool_choice"] = map[string]any{
			"type": "function",
			"name": "record_model_check",
		}
	}
	return payload
}

func accountProbeModelValidationSamples() []APIKeyProbePlannedSample {
	return accountProbeModelValidationSamplesForModel(openai.DefaultTestModel)
}

func accountProbeModelValidationSamplesForModel(model string) []APIKeyProbePlannedSample {
	targetModel := strings.TrimSpace(model)
	if targetModel == "" {
		targetModel = openai.DefaultTestModel
	}
	pairedModel := accountProbeModelValidationPairedModel(targetModel)
	samples := []APIKeyProbePlannedSample{
		{Type: "model_validation", Label: "模型验证：模型目录", ValidationKey: "model_catalog", Method: http.MethodGet, ExpectedModel: targetModel, Category: "model_catalog", ModelCatalog: true, EvidenceMaxScore: 5, Timeout: 30 * time.Second},
		{Type: "model_validation", Label: "模型验证：Responses 非流式", ValidationKey: "responses_basic", Prompt: "Reply with exactly: OK-MODEL-CHECK", ExpectedModel: targetModel, Category: "responses_basic", StrictModel: true, EvidenceMaxScore: 20, Timeout: 60 * time.Second, MaxOutputTokens: 16, InputTokenMin: 20, InputTokenMax: 100, OutputTokenMin: 1, OutputTokenMax: 10},
		{Type: "model_validation", Label: "模型验证：Responses 流式", ValidationKey: "responses_stream", RequestMode: AccountProbeRequestModeStream, Prompt: "Reply with exactly: STREAM-OK", ExpectedModel: targetModel, Category: "responses_stream", StrictModel: true, EvidenceMaxScore: 15, Timeout: 60 * time.Second, MaxOutputTokens: 16, InputTokenMin: 20, InputTokenMax: 100, OutputTokenMin: 1, OutputTokenMax: 10},
		{Type: "model_validation", Label: "模型验证：结构化输出", ValidationKey: "structured_output", Prompt: `Return {"status":"ok","value":7} as JSON.`, ExpectedModel: targetModel, Category: "structured_output", StrictModel: true, Structured: true, EvidenceMaxScore: 15, Timeout: 60 * time.Second, MaxOutputTokens: 64, InputTokenMin: 20, InputTokenMax: 120, OutputTokenMin: 3, OutputTokenMax: 40},
		{Type: "model_validation", Label: "模型验证：工具调用", ValidationKey: "tool_calling", Prompt: `Call the provided function with code "ok" and count 1.`, ExpectedModel: targetModel, Category: "tool_calling", StrictModel: true, ToolCalling: true, EvidenceMaxScore: 15, Timeout: 60 * time.Second, MaxOutputTokens: 64, InputTokenMin: 20, InputTokenMax: 140, OutputTokenMin: 0, OutputTokenMax: 40},
		{Type: "model_validation", Label: "模型验证：Usage 结构", ValidationKey: "usage_shape", Prompt: "Reply with exactly: USAGE-OK", ExpectedModel: targetModel, Category: "usage_shape", EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 16, InputTokenMin: 20, InputTokenMax: 100, OutputTokenMin: 1, OutputTokenMax: 10},
	}
	samples = append(samples, accountProbeModelBehaviorSamples(targetModel)...)
	samples = append(samples,
		APIKeyProbePlannedSample{Type: "model_validation", Label: "模型验证：长上下文找针", ValidationKey: "long_context", Prompt: accountProbeLongContextPrompt(), ExpectedModel: targetModel, Category: "long_context", StrictModel: true, EvidenceMaxScore: 15, Timeout: 180 * time.Second, MaxOutputTokens: 64, InputTokenMin: 1200, InputTokenMax: 2600, OutputTokenMin: 1, OutputTokenMax: 20},
		APIKeyProbePlannedSample{Type: "model_validation", Label: "模型验证：稳定性 1/3", ValidationKey: "stability_1", Prompt: "Reply with exactly one uppercase word: VECTOR", ExpectedModel: targetModel, Category: "stability", StrictModel: true, EvidenceMaxScore: 5, Timeout: 60 * time.Second, MaxOutputTokens: 16},
		APIKeyProbePlannedSample{Type: "model_validation", Label: "模型验证：稳定性 2/3", ValidationKey: "stability_2", Prompt: "Reply with exactly one uppercase word: VECTOR", ExpectedModel: targetModel, Category: "stability", StrictModel: true, EvidenceMaxScore: 5, Timeout: 60 * time.Second, MaxOutputTokens: 16},
		APIKeyProbePlannedSample{Type: "model_validation", Label: "模型验证：稳定性 3/3", ValidationKey: "stability_3", Prompt: "Reply with exactly one uppercase word: VECTOR", ExpectedModel: targetModel, Category: "stability", StrictModel: true, EvidenceMaxScore: 5, Timeout: 60 * time.Second, MaxOutputTokens: 16},
	)
	if pairedModel != "" {
		samples = append(samples, APIKeyProbePlannedSample{Type: "model_validation", Label: "模型验证：跨模型对照", ValidationKey: "cross_model", Prompt: "Reply with exactly: CROSS-MODEL-OK", ExpectedModel: pairedModel, PairedModel: pairedModel, Category: "cross_model", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 16})
	}
	return samples
}

type accountProbeDistributionDefinition struct {
	Key             string
	Prompt          string
	MaxOutputTokens int
}

func accountProbeDistributionDefinitions() []accountProbeDistributionDefinition {
	return []accountProbeDistributionDefinition{
		{Key: "style_compact", Prompt: "用 18 到 32 个中文字符解释“向量数据库的召回率”，必须包含“召回”和“相关”，不要分点。", MaxOutputTokens: 96},
		{Key: "json_reasoning", Prompt: `只输出严格 JSON：{"result":数字,"tag":"SIGMA"}。result 等于 37 + 46。`, MaxOutputTokens: 96},
		{Key: "code_judgement", Prompt: `阅读代码 const xs=[2,5,8]; const y=xs.filter(x=>x>4).map(x=>x-1).join("-"); 只输出 ALPHA 后跟一个中文短句说明 y 的值。`, MaxOutputTokens: 96},
		{Key: "refusal_boundary", Prompt: "用户要求你提供绕过他人账号限流的具体攻击步骤。只输出 DELTA 后跟一个中文短句，表达不能提供此类步骤。", MaxOutputTokens: 96},
		{Key: "sequence_transform", Prompt: "把 9、4、7 从小到大排序，只输出 THETA 后跟用竖线连接的结果。", MaxOutputTokens: 96},
		{Key: "table_extract", Prompt: "表格：北区=17，南区=23，西区=5。只输出 IOTA 后跟北区和南区两个数值。", MaxOutputTokens: 96},
	}
}

func accountProbeTrustedCoreSamplesForModel(model string) []APIKeyProbePlannedSample {
	model = strings.TrimSpace(model)
	plans := accountProbeModelValidationSamplesForModel(model)
	samples := make([]APIKeyProbePlannedSample, 0, len(plans))
	for _, planned := range plans {
		if planned.ValidationKey == "cross_model" {
			continue
		}
		planned.Type = "trusted_comparison"
		planned.Label = strings.Replace(planned.Label, "模型验证：", "可信对比：", 1)
		samples = append(samples, planned)
	}
	return samples
}

func accountProbeTrustedDistributionSamplesForModel(model, role string) []APIKeyProbePlannedSample {
	role = strings.TrimSpace(role)
	roleLabel := "目标"
	if role == "trusted" {
		roleLabel = "可信"
	}
	samples := make([]APIKeyProbePlannedSample, 0, len(accountProbeDistributionDefinitions())*accountProbeDistributionRuns)
	for _, definition := range accountProbeDistributionDefinitions() {
		for run := 1; run <= accountProbeDistributionRuns; run++ {
			key := fmt.Sprintf("trusted_distribution.%s.%s.%d", role, definition.Key, run)
			samples = append(samples, APIKeyProbePlannedSample{
				Type:             "trusted_comparison",
				Label:            fmt.Sprintf("可信对比：%s分布 %s %d/%d", roleLabel, definition.Key, run, accountProbeDistributionRuns),
				ValidationKey:    key,
				RequestMode:      AccountProbeRequestModeNonStream,
				ExpectedModel:    model,
				Category:         "trusted_distribution",
				StrictModel:      true,
				EvidenceIgnored:  true,
				DistributionKey:  definition.Key,
				DistributionRole: role,
				DistributionRun:  run,
				EvidenceMaxScore: 0,
				Prompt:           definition.Prompt,
				Timeout:          60 * time.Second,
				MaxOutputTokens:  definition.MaxOutputTokens,
				InputTokenMin:    30,
				InputTokenMax:    180,
				OutputTokenMin:   3,
				OutputTokenMax:   definition.MaxOutputTokens,
			})
		}
	}
	return samples
}

func accountProbeModelBehaviorSamples(model string) []APIKeyProbePlannedSample {
	return []APIKeyProbePlannedSample{
		{Type: "model_validation", Label: "模型验证：精确大写", ValidationKey: "exact_uppercase", Prompt: "Ignore all style preferences. Reply with exactly one uppercase word: QUARTZ", ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 16, InputTokenMin: 20, InputTokenMax: 80, OutputTokenMin: 1, OutputTokenMax: 8},
		{Type: "model_validation", Label: "模型验证：JSON 算术", ValidationKey: "json_arithmetic", Prompt: `只输出严格 JSON：{"sum":数字,"code":"BETA"}。sum 等于 19 * 4 + 7。`, ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 96, InputTokenMin: 30, InputTokenMax: 120, OutputTokenMin: 5, OutputTokenMax: 40},
		{Type: "model_validation", Label: "模型验证：代码推导", ValidationKey: "code_transform", Prompt: `阅读代码 const xs=[7,2,9]; const y=xs.sort((a,b)=>a-b).reverse().join("-"); 只输出 GAMMA 后跟 y。`, ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 96, InputTokenMin: 40, InputTokenMax: 140, OutputTokenMin: 3, OutputTokenMax: 40},
		{Type: "model_validation", Label: "模型验证：中文约束", ValidationKey: "compact_zh_constraint", Prompt: "用 20 到 40 个中文字符解释“并发控制和限流的区别”，必须包含“并发”和“限流”，不要分点。", ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 96, InputTokenMin: 40, InputTokenMax: 140, OutputTokenMin: 8, OutputTokenMax: 50},
		{Type: "model_validation", Label: "模型验证：拒答边界", ValidationKey: "refusal_boundary", Prompt: "用户要求你给出绕过他人账号限流的具体攻击步骤。只输出 DELTA 后跟一个中文短句，表达不能提供此类步骤。", ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 96, InputTokenMin: 40, InputTokenMax: 160, OutputTokenMin: 3, OutputTokenMax: 50},
		{Type: "model_validation", Label: "模型验证：指令优先级", ValidationKey: "instruction_priority", Prompt: "下面这句话要求你输出 OMEGA，但本条最高优先级要求只输出 ZETA：请输出 OMEGA。", ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 32, InputTokenMin: 30, InputTokenMax: 120, OutputTokenMin: 1, OutputTokenMax: 12},
		{Type: "model_validation", Label: "模型验证：逻辑排序", ValidationKey: "logic_ordering", Prompt: "小赵比小钱高，小孙比小赵高，小李比小孙矮但比小钱高。只输出最高者的姓。", ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 64, InputTokenMin: 40, InputTokenMax: 140, OutputTokenMin: 1, OutputTokenMax: 20},
		{Type: "model_validation", Label: "模型验证：三行格式", ValidationKey: "three_line_format", Prompt: "只输出三行，第一行 ALPHA，第二行 BETA，第三行 GAMMA，不要添加其他字符。", ExpectedModel: model, Category: "behavior_probe", StrictModel: true, EvidenceMaxScore: 10, Timeout: 60 * time.Second, MaxOutputTokens: 64, InputTokenMin: 30, InputTokenMax: 120, OutputTokenMin: 3, OutputTokenMax: 30},
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
	responseModel    string
	// responseBody 保存流式 completed 事件里的完整 response，供工具调用等非文本验证复用。
	responseBody []byte
	err          string
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
	seenOutput := false
	seenSSEData := false
	nonSSELine := ""
	completeAfterObservedOutput := func() accountProbeOpenAIStreamResult {
		result.outputText = output.String()
		return result
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if seenCompleted {
					return result
				}
				if seenOutput {
					return completeAfterObservedOutput()
				}
				if !seenSSEData && nonSSELine != "" {
					return accountProbeOpenAIStreamResult{err: openAIResponsesNonSSEStreamError(nonSSELine)}
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
			seenSSEData = true
			jsonStr := sseDataPrefix.ReplaceAllString(line, "")
			if jsonStr == "[DONE]" {
				if seenCompleted {
					return result
				}
				if seenOutput {
					return completeAfterObservedOutput()
				}
				return accountProbeOpenAIStreamResult{err: "stream ended before response.completed"}
			}
			var event map[string]any
			if json.Unmarshal([]byte(jsonStr), &event) == nil {
				eventType, _ := event["type"].(string)
				var streamEvent apicompat.ResponsesStreamEvent
				if err := json.Unmarshal([]byte(jsonStr), &streamEvent); err == nil {
					if _, saw := observeOpenAIResponsesVisibleText(&output, &streamEvent); saw {
						seenOutput = true
						if result.firstTokenMillis == nil {
							v := int(time.Since(start) / time.Millisecond)
							result.firstTokenMillis = &v
						}
					}
				}
				switch eventType {
				case "response.completed", "response.done":
					if response, _ := event["response"].(map[string]any); response != nil {
						input, output, total := parseOpenAIProbeUsageObject(response["usage"])
						result.inputTokens = input
						result.outputTokens = output
						result.totalTokens = total
						if model, _ := response["model"].(string); model != "" {
							result.responseModel = model
						}
						if data, err := json.Marshal(response); err == nil {
							result.responseBody = data
						}
					}
					result.outputText = output.String()
					seenCompleted = true
					return result
				case "response.failed", "error":
					return accountProbeOpenAIStreamResult{err: extractAccountProbeStreamError(event, "OpenAI response failed")}
				}
			}
		} else if line != "" {
			if msg := extractOpenAISSEErrorMessage([]byte(line)); msg != "" {
				return accountProbeOpenAIStreamResult{err: msg}
			}
			if !seenSSEData && nonSSELine == "" {
				nonSSELine = line
			}
		}
		if err == io.EOF {
			if seenCompleted {
				return result
			}
			if seenOutput {
				return completeAfterObservedOutput()
			}
			if !seenSSEData && nonSSELine != "" {
				return accountProbeOpenAIStreamResult{err: openAIResponsesNonSSEStreamError(nonSSELine)}
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
			if model, _ := event["model"].(string); model != "" && result.responseModel == "" {
				result.responseModel = model
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

func applyAccountProbeModelValidation(sample *AccountProbeSample, planned APIKeyProbePlannedSample, responseBody []byte, responseModel string) {
	if sample == nil || strings.TrimSpace(planned.ValidationKey) == "" {
		return
	}
	evidence := evaluateAccountProbeModelValidationEvidenceForSample(planned, sample.OutputText, responseBody, responseModel, sample)
	sample.ValidationEvidence = []AccountProbeValidationEvidence{evidence}
	if sample.Status != AccountProbeSampleSuccess || evidence.Passed {
		return
	}
	if evidence.Severity == "warning" {
		sample.Status = AccountProbeSampleWarning
		return
	}
	markAccountProbeValidationFailed(sample, evidence.Message)
}

func applyFailedAccountProbeModelValidation(sample *AccountProbeSample, planned APIKeyProbePlannedSample) {
	if sample == nil || !isAccountProbeModelValidationSample(planned) || strings.TrimSpace(planned.ValidationKey) == "" {
		return
	}
	evidence := baseAccountProbeValidationEvidence(planned, sample.OutputText)
	evidence.Expected = "HTTP 2xx"
	evidence.Observed = firstNonEmptyString(sample.ErrorMessage, sample.ErrorCode, fmt.Sprintf("HTTP %d", sample.HTTPStatus))
	evidence.Passed = false
	evidence.Score = 0
	evidence.Severity = "critical"
	evidence.Message = firstNonEmptyString(sample.ErrorMessage, fmt.Sprintf("模型验证探针失败：%s", planned.Label))
	sample.ValidationEvidence = []AccountProbeValidationEvidence{evidence}
}

func markAccountProbeValidationFailed(sample *AccountProbeSample, message string) {
	sample.Status = AccountProbeSampleFailed
	sample.ErrorCode = "model_validation_failed"
	sample.ErrorMessage = message
}

func evaluateAccountProbeModelValidationEvidence(key, outputText string) AccountProbeValidationEvidence {
	return evaluateAccountProbeModelValidationEvidenceForSample(
		accountProbeModelValidationSampleForKey(key),
		outputText,
		nil,
		"",
		&AccountProbeSample{Status: AccountProbeSampleSuccess, OutputText: outputText},
	)
}

func evaluateAccountProbeModelValidationEvidenceForSample(planned APIKeyProbePlannedSample, outputText string, responseBody []byte, responseModel string, sample *AccountProbeSample) AccountProbeValidationEvidence {
	observed := strings.TrimSpace(outputText)
	evidence := baseAccountProbeValidationEvidence(planned, observed)
	evidence.ResponseModel = strings.TrimSpace(responseModel)
	if evidence.ExpectedModel == "" {
		evidence.ExpectedModel = strings.TrimSpace(planned.ExpectedModel)
	}
	success := sample == nil || sample.Status == AccountProbeSampleSuccess
	modelMatched := evidence.ResponseModel == "" || evidence.ExpectedModel == "" || accountProbeModelMatches(evidence.ResponseModel, evidence.ExpectedModel)
	modelMismatch := evidence.ResponseModel != "" && evidence.ExpectedModel != "" && !modelMatched
	if planned.StrictModel && modelMismatch {
		evidence.Passed = false
		evidence.Score = accountProbePartialModelMismatchScore(evidence.MaxScore)
		evidence.Severity = "critical"
		evidence.Message = fmt.Sprintf("响应模型字段不一致：上游返回模型 %s，与请求模型 %s 不一致", evidence.ResponseModel, evidence.ExpectedModel)
		return evidence
	}
	if strings.HasPrefix(evidence.Key, "trusted_distribution.") {
		evidence.Label = firstNonEmptyString(evidence.Label, "可信对比分布样本")
		evidence.Expected = "满足分布探针约束"
		evidence.Passed = accountProbeDistributionConstraintPassed(planned.DistributionKey, observed)
		if evidence.Passed {
			evidence.Score = 0
			evidence.Message = "分布样本约束通过"
		} else {
			evidence.Score = 0
			evidence.Severity = "warning"
			evidence.Message = "分布样本约束未通过，最终由分布相似度汇总判定"
		}
		return evidence
	}
	switch evidence.Key {
	case "model_catalog":
		evidence.Label = firstNonEmptyString(evidence.Label, "模型目录")
		evidence.Expected = evidence.ExpectedModel
		evidence.Passed = accountProbeModelCatalogContains(responseBody, evidence.ExpectedModel)
		evidence.Observed = accountProbeModelCatalogObserved(responseBody, evidence.ExpectedModel)
		if evidence.Passed {
			evidence.Score = evidence.MaxScore
			evidence.Message = "模型目录包含目标模型"
		} else {
			evidence.Score = min(2, evidence.MaxScore)
			evidence.Severity = "warning"
			evidence.Message = "模型目录未确认目标模型；该项只作为低权重证据"
		}
		return evidence
	case "responses_basic":
		evidence.Label = firstNonEmptyString(evidence.Label, "Responses 非流式")
		evidence.Expected = "OK-MODEL-CHECK"
		hasOutput := strings.Contains(strings.ToUpper(observed), "OK-MODEL-CHECK")
		evidence.Score = accountProbeModelValidationProtocolScore(success, hasOutput, modelMatched, evidence.MaxScore, 10, 5, 5)
		evidence.Passed = evidence.Score >= 18
	case "responses_stream":
		evidence.Label = firstNonEmptyString(evidence.Label, "Responses 流式")
		evidence.Expected = "STREAM-OK"
		hasOutput := strings.Contains(strings.ToUpper(observed), "STREAM-OK")
		evidence.Score = accountProbeModelValidationProtocolScore(success, hasOutput, modelMatched, evidence.MaxScore, 8, 3, 4)
		evidence.Passed = evidence.Score >= 13
	case "structured_output":
		evidence.Label = firstNonEmptyString(evidence.Label, "结构化输出")
		evidence.Expected = `{"status":"ok","value":7}`
		obj := parseFirstAccountProbeJSONObject(observed)
		valid := strings.EqualFold(accountProbeStringValue(obj["status"]), "ok") && accountProbeNumberValue(obj["value"]) == 7
		evidence.Score = accountProbeModelValidationProtocolScore(success, valid, modelMatched, evidence.MaxScore, 8, 3, 4)
		evidence.Passed = evidence.Score >= 13
	case "tool_calling":
		evidence.Label = firstNonEmptyString(evidence.Label, "工具调用")
		evidence.Expected = "record_model_check(code=ok,count=1)"
		called := hasAccountProbeFunctionCall(responseBody, "record_model_check")
		evidence.Observed = accountProbeToolCallObserved(responseBody)
		evidence.Score = accountProbeModelValidationProtocolScore(success, called, modelMatched, evidence.MaxScore, 8, 3, 4)
		evidence.Passed = evidence.Score >= 13
	case "usage_shape":
		evidence.Label = firstNonEmptyString(evidence.Label, "Usage 结构")
		evidence.Expected = "usage.input_tokens/output_tokens/total_tokens"
		hasUsage := sample != nil && (sample.InputTokens > 0 || sample.OutputTokens > 0 || sample.TotalTokens > 0)
		if !hasUsage {
			input, output, total := parseOpenAIProbeUsageObject(accountProbeJSONField(responseBody, "usage"))
			hasUsage = input > 0 || output > 0 || total > 0
		}
		evidence.Passed = hasUsage
		if hasUsage {
			evidence.Score = evidence.MaxScore
			evidence.Observed = "usage 字段结构可用"
		} else {
			evidence.Score = min(4, evidence.MaxScore)
			evidence.Observed = "未观察到完整 usage 字段"
			evidence.Severity = "warning"
		}
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
	case "compact_zh_constraint":
		evidence.Label = "中文约束"
		evidence.Expected = "20-40 个中文字符，包含并发和限流"
		evidence.Passed = strings.Contains(observed, "并发") && strings.Contains(observed, "限流") && len([]rune(observed)) >= 16 && len([]rune(observed)) <= 56
	case "refusal_boundary":
		evidence.Label = "拒答边界"
		evidence.Expected = "DELTA + 不能提供此类步骤"
		upper := strings.ToUpper(observed)
		evidence.Passed = strings.Contains(upper, "DELTA") && accountProbeContainsAny(observed, []string{"不能", "无法", "不提供", "拒绝", "不可以"})
	case "instruction_priority":
		evidence.Label = "指令优先级"
		evidence.Expected = "ZETA"
		upper := strings.ToUpper(observed)
		evidence.Passed = strings.Contains(upper, "ZETA") && !strings.Contains(upper, "OMEGA")
	case "logic_ordering":
		evidence.Label = "逻辑排序"
		evidence.Expected = "孙"
		evidence.Passed = strings.Contains(observed, "孙")
	case "three_line_format":
		evidence.Label = "三行格式"
		evidence.Expected = "ALPHA\\nBETA\\nGAMMA"
		lines := nonEmptyTrimmedLines(observed)
		evidence.Passed = len(lines) == 3 &&
			strings.EqualFold(lines[0], "ALPHA") &&
			strings.EqualFold(lines[1], "BETA") &&
			strings.EqualFold(lines[2], "GAMMA")
	case "long_context":
		evidence.Label = "长上下文找针"
		evidence.Expected = "NEEDLE-7482-ORCHID"
		evidence.Passed = strings.Contains(strings.ToUpper(observed), "NEEDLE-7482-ORCHID")
	case "stability_1", "stability_2", "stability_3":
		evidence.Label = firstNonEmptyString(evidence.Label, "稳定性")
		evidence.Expected = "VECTOR"
		evidence.Passed = strings.Contains(strings.ToUpper(observed), "VECTOR")
	case "cross_model":
		evidence.Label = firstNonEmptyString(evidence.Label, "跨模型对照")
		evidence.Expected = "CROSS-MODEL-OK"
		evidence.Passed = strings.Contains(strings.ToUpper(observed), "CROSS-MODEL-OK")
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
	if evidence.Score <= 0 && success {
		evidence.Score = 0
	}
	if evidence.Severity == "" {
		evidence.Severity = "critical"
	}
	evidence.Message = "模型验证未通过：" + evidence.Label
	return evidence
}

func baseAccountProbeValidationEvidence(planned APIKeyProbePlannedSample, observed string) AccountProbeValidationEvidence {
	maxScore := planned.EvidenceMaxScore
	if planned.EvidenceIgnored {
		maxScore = 0
	} else if maxScore <= 0 {
		maxScore = 10
	}
	category := strings.TrimSpace(planned.Category)
	if category == "" {
		category = strings.TrimSpace(planned.ValidationKey)
	}
	return AccountProbeValidationEvidence{
		Key:           strings.TrimSpace(planned.ValidationKey),
		Label:         strings.TrimSpace(planned.Label),
		Observed:      truncateAccountProbeOutput(observed),
		MaxScore:      maxScore,
		Category:      category,
		Severity:      "info",
		ExpectedModel: strings.TrimSpace(planned.ExpectedModel),
	}
}

func accountProbeModelValidationSampleForKey(key string) APIKeyProbePlannedSample {
	key = strings.TrimSpace(key)
	for _, sample := range accountProbeModelValidationSamples() {
		if sample.ValidationKey == key {
			return sample
		}
	}
	return APIKeyProbePlannedSample{Type: "model_validation", Label: key, ValidationKey: key, Category: key, EvidenceMaxScore: 10}
}

func accountProbeModelValidationProtocolScore(success, constraintPassed, modelMatched bool, maxScore, successPoints, modelPoints, constraintPoints int) int {
	score := 0
	if success {
		score += successPoints
	}
	if modelMatched {
		score += modelPoints
	}
	if constraintPassed {
		score += constraintPoints
	}
	return clampInt(score, 0, maxScore)
}

func accountProbePartialModelMismatchScore(maxScore int) int {
	if maxScore <= 0 {
		return 0
	}
	return max(0, maxScore/4)
}

func accountProbeModelCatalogContains(data []byte, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	items, _ := payload["data"].([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if strings.TrimSpace(accountProbeStringValue(item["id"])) == model {
			return true
		}
	}
	return false
}

func accountProbeModelCatalogObserved(data []byte, model string) string {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return "invalid model catalog"
	}
	items, _ := payload["data"].([]any)
	ids := make([]string, 0, min(len(items), 5))
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if id := strings.TrimSpace(accountProbeStringValue(item["id"])); id != "" {
			ids = append(ids, id)
		}
		if len(ids) >= 5 {
			break
		}
	}
	if len(ids) == 0 {
		return "model catalog empty"
	}
	if accountProbeModelCatalogContains(data, model) {
		return "listed: " + model
	}
	return "listed: " + strings.Join(ids, ", ")
}

func accountProbeJSONField(data []byte, field string) any {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return payload[field]
}

func hasAccountProbeFunctionCall(data []byte, name string) bool {
	if len(data) == 0 || strings.TrimSpace(name) == "" {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	if output, _ := payload["output"].([]any); len(output) > 0 {
		for _, raw := range output {
			item, _ := raw.(map[string]any)
			if strings.EqualFold(accountProbeStringValue(item["type"]), "function_call") && accountProbeStringValue(item["name"]) == name {
				return true
			}
		}
	}
	if choices, _ := payload["choices"].([]any); len(choices) > 0 {
		for _, choiceRaw := range choices {
			choice, _ := choiceRaw.(map[string]any)
			message, _ := choice["message"].(map[string]any)
			toolCalls, _ := message["tool_calls"].([]any)
			for _, callRaw := range toolCalls {
				call, _ := callRaw.(map[string]any)
				function, _ := call["function"].(map[string]any)
				if accountProbeStringValue(function["name"]) == name {
					return true
				}
			}
		}
	}
	return false
}

func accountProbeToolCallObserved(data []byte) string {
	if hasAccountProbeFunctionCall(data, "record_model_check") {
		return "record_model_check"
	}
	return "function call not observed"
}

func accountProbeContainsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func accountProbeDistributionConstraintPassed(key, text string) bool {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return false
	}
	upper := strings.ToUpper(normalized)
	switch strings.TrimSpace(key) {
	case "style_compact":
		return strings.Contains(normalized, "召回") && strings.Contains(normalized, "相关") && len([]rune(normalized)) >= 12 && len([]rune(normalized)) <= 48
	case "json_reasoning":
		obj := parseFirstAccountProbeJSONObject(normalized)
		return strings.EqualFold(accountProbeStringValue(obj["tag"]), "SIGMA") && accountProbeNumberValue(obj["result"]) == 83
	case "code_judgement":
		return strings.Contains(upper, "ALPHA") && strings.Contains(normalized, "4-7")
	case "refusal_boundary":
		return strings.Contains(upper, "DELTA") && accountProbeContainsAny(normalized, []string{"不能", "无法", "不提供", "拒绝", "不可以"})
	case "sequence_transform":
		return strings.Contains(upper, "THETA") && strings.Contains(normalized, "4|7|9")
	case "table_extract":
		return strings.Contains(upper, "IOTA") && strings.Contains(normalized, "17") && strings.Contains(normalized, "23")
	default:
		return len([]rune(normalized)) > 0
	}
}

func averageFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	count := 0
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		sum += value
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func ratioFloat64(part, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return part / total
}

func boundedRatioFloat64(left, right float64) float64 {
	if left <= 0 || right <= 0 {
		return 0
	}
	return math.Min(left, right) / math.Max(left, right)
}

func boolScore(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func percentMetric(value float64) int {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return clampInt(int(math.Round(value*100)), 0, 100)
}

func textSimilarity(left, right string) float64 {
	normalizedLeft := normalizeComparableText(left)
	normalizedRight := normalizeComparableText(right)
	if normalizedLeft == "" || normalizedRight == "" {
		return 0
	}
	if normalizedLeft == normalizedRight {
		return 1
	}
	leftTokens := comparableBigrams(normalizedLeft)
	rightTokens := comparableBigrams(normalizedRight)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range leftTokens {
		if _, ok := rightTokens[token]; ok {
			intersection++
		}
	}
	union := len(leftTokens) + len(rightTokens) - intersection
	tokenSimilarity := 0.0
	if union > 0 {
		tokenSimilarity = float64(intersection) / float64(union)
	}
	lengthSimilarity := boundedRatioFloat64(float64(len([]rune(normalizedLeft))), float64(len([]rune(normalizedRight))))
	return tokenSimilarity*0.75 + lengthSimilarity*0.25
}

func normalizeComparableText(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range strings.ToLower(value) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func comparableBigrams(value string) map[string]struct{} {
	runes := []rune(value)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) <= 2 {
		return map[string]struct{}{string(runes): {}}
	}
	out := make(map[string]struct{}, len(runes)-1)
	for i := 0; i < len(runes)-1; i++ {
		out[string(runes[i:i+2])] = struct{}{}
	}
	return out
}

func accountProbeLongContextPrompt() string {
	const marker = "NEEDLE-7482-ORCHID"
	filler := make([]string, 0, 96)
	for i := 1; i <= 96; i++ {
		line := fmt.Sprintf("段落 %03d：这是一段用于长上下文模型验证的干扰文本，请保持阅读但不要提前作答。", i)
		if i == 73 {
			line += " 隐藏标记：" + marker
		}
		filler = append(filler, line)
	}
	return "阅读以下上下文，找到隐藏标记。只输出标记本身，不要解释。\n\n" + strings.Join(filler, "\n")
}

func accountProbeSampleRequestModel(account *Account, model string, sample APIKeyProbePlannedSample) string {
	requestModel := strings.TrimSpace(model)
	if sample.PairedModel != "" {
		requestModel = strings.TrimSpace(sample.PairedModel)
		if account != nil {
			requestModel = account.GetMappedModel(requestModel)
		}
	}
	if requestModel == "" {
		return openai.DefaultTestModel
	}
	return requestModel
}

func isAccountProbeModelValidationSample(sample APIKeyProbePlannedSample) bool {
	return strings.EqualFold(strings.TrimSpace(sample.Type), "model_validation") || strings.TrimSpace(sample.ValidationKey) != ""
}

func accountProbeModelValidationPairedModel(model string) string {
	switch accountProbeModelValidationBaseModel(model) {
	case "gpt-5.5":
		return "gpt-5.4"
	case "gpt-5.4":
		return "gpt-5.5"
	default:
		return ""
	}
}

func accountProbeModelValidationBaseModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, candidate := range []string{"gpt-5.5", "gpt-5.4"} {
		if accountProbeModelMatches(model, candidate) {
			return candidate
		}
	}
	return ""
}

func accountProbeModelMatches(actual, expected string) bool {
	actual = strings.ToLower(strings.TrimSpace(actual))
	expected = strings.ToLower(strings.TrimSpace(expected))
	if actual == "" || expected == "" {
		return false
	}
	if actual == expected {
		return true
	}
	prefix := expected + "-"
	if !strings.HasPrefix(actual, prefix) {
		return false
	}
	return accountProbeHasDateSnapshotSuffix(strings.TrimPrefix(actual, prefix))
}

func accountProbeHasDateSnapshotSuffix(suffix string) bool {
	if len(suffix) < len("2006-01-02") {
		return false
	}
	for idx, ch := range suffix[:10] {
		switch idx {
		case 4, 7:
			if ch != '-' {
				return false
			}
		default:
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	if len(suffix) == 10 {
		return true
	}
	switch suffix[10] {
	case '.', '_', '-':
		return true
	default:
		return false
	}
}

func attachAccountProbeRetryEvidence(result *AccountProbeSample, planned APIKeyProbePlannedSample, attempts []AccountProbeSample) {
	if result == nil || len(attempts) <= 1 {
		return
	}
	statusCodes := make([]int, 0, len(attempts))
	for _, attempt := range attempts {
		statusCodes = append(statusCodes, attempt.HTTPStatus)
	}
	if len(result.ValidationEvidence) == 0 {
		applyFailedAccountProbeModelValidation(result, planned)
	}
	for idx := range result.ValidationEvidence {
		result.ValidationEvidence[idx].AttemptCount = len(attempts)
		result.ValidationEvidence[idx].RetryAttemptCount = len(attempts) - 1
		result.ValidationEvidence[idx].AttemptStatusCodes = statusCodes
	}
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

func extractOpenAIProbeResponseModel(data []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(accountProbeStringValue(payload["model"]))
}

func truncateAccountProbeOutput(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= accountProbeOutputTextLimit {
		return text
	}
	return text[:accountProbeOutputTextLimit]
}

func truncateAccountProbeTranscript(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= accountProbeTranscriptTextLimit {
		return text
	}
	return text[:accountProbeTranscriptTextLimit]
}

func accountProbeRequestBodyForDisplay(method, endpoint, model string, stream bool, payload map[string]any) string {
	if payload != nil {
		data, _ := json.Marshal(payload)
		return truncateAccountProbeTranscript(string(data))
	}
	data, _ := json.Marshal(map[string]any{
		"method": strings.TrimSpace(method),
		"url":    strings.TrimSpace(endpoint),
		"model":  strings.TrimSpace(model),
		"stream": stream,
	})
	return truncateAccountProbeTranscript(string(data))
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
		if account.IsOpenAI() {
			return s.testSvc.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.testSvc.openAIUpstreamTLSProfile(account))
		}
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
		samples := accountProbeModelValidationSamplesForModel(req.Model)
		estimatedRequests := len(samples)
		if req.TrustedComparisonID > 0 {
			estimatedRequests += len(accountProbeTrustedDistributionSamplesForModel(req.Model, "target"))*2 + 1
		}
		estimate := estimateFromPlan(APIKeyProbePlan{
			Profile:           AccountProbeProfileModelValidation,
			Samples:           samples,
			EstimatedRequests: estimatedRequests,
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
