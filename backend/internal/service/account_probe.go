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
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

const (
	AccountProbeProfileQuick    = APIKeyProbeProfileQuick
	AccountProbeProfileStandard = APIKeyProbeProfileStandard

	AccountProbeStatusSuccess = APIKeyProbeStatusSuccess
	AccountProbeStatusPartial = APIKeyProbeStatusPartial
	AccountProbeStatusFailed  = APIKeyProbeStatusFailed
	AccountProbeStatusRunning = APIKeyProbeStatusRunning

	AccountProbeSampleSuccess = APIKeyProbeSampleSuccess
	AccountProbeSampleFailed  = APIKeyProbeSampleFailed

	AccountProbeRequestModeNonStream = "non_stream"
	AccountProbeRequestModeStream    = "stream"
)

const accountProbePersistenceTimeout = 5 * time.Second

type AccountProbeRunRequest struct {
	AccountID             int64  `json:"-"`
	Profile               string `json:"mode"`
	Model                 string `json:"model"`
	IncludeCodexStability bool   `json:"codex_stability"`
	IncludeLongContext    bool   `json:"long_context"`
	RequestMode           string `json:"request_mode"`
}

type AccountProbeEstimate = APIKeyProbeEstimate
type AccountProbeLatencyStats = APIKeyProbeLatencyStats

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
	ID                int64     `json:"id"`
	RunID             int64     `json:"run_id"`
	RequestIndex      int       `json:"request_index"`
	Type              string    `json:"type"`
	Label             string    `json:"label"`
	Status            string    `json:"status"`
	Model             string    `json:"model"`
	APIKeyFingerprint string    `json:"api_key_fingerprint,omitempty"`
	APIKeyMasked      string    `json:"api_key_masked,omitempty"`
	UpstreamEndpoint  string    `json:"upstream_endpoint,omitempty"`
	HTTPStatus        int       `json:"http_status,omitempty"`
	DurationMillis    int       `json:"latency_ms"`
	FirstTokenMillis  *int      `json:"first_token_ms,omitempty"`
	InputTokens       int       `json:"input_tokens"`
	OutputTokens      int       `json:"output_tokens"`
	TotalTokens       int       `json:"tokens"`
	ErrorCode         string    `json:"error_code,omitempty"`
	ErrorMessage      string    `json:"error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type AccountProbeHistoryFilter struct {
	AccountID int64
	Limit     int
}

type AccountProbeRepository interface {
	CreateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error
	UpdateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error
	SaveAccountProbeSample(ctx context.Context, sample AccountProbeSample) error
	ListAccountProbeRuns(ctx context.Context, filter AccountProbeHistoryFilter) ([]AccountProbeResult, error)
	GetAccountProbeRun(ctx context.Context, accountID, runID int64) (*AccountProbeResult, error)
	ListAccountProbeSamples(ctx context.Context, runID int64) ([]AccountProbeSample, error)
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
}

func NewAccountProbeService(accounts AccountProbeAccountReader, repo AccountProbeRepository, client AccountProbeHTTPClient, testSvc *AccountTestService) *AccountProbeService {
	if client == nil {
		client = &http.Client{}
	}
	return &AccountProbeService{accounts: accounts, repo: repo, client: client, testSvc: testSvc}
}

func (s *AccountProbeService) Run(ctx context.Context, req AccountProbeRunRequest) (AccountProbeResult, error) {
	run, err := s.Start(ctx, req)
	if err != nil {
		return AccountProbeResult{}, err
	}
	return s.RunExisting(ctx, run, req)
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
		IncludeCodexStability: req.IncludeCodexStability,
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
	account, keys, plan, model, baseURL, useResponses, err := s.prepareRun(ctx, req)
	if err != nil {
		return s.failExistingRun(ctx, run, err.Error()), err
	}
	run.AccountID = account.ID
	run.Profile = plan.Profile
	run.Model = model
	run.RequestMode = normalizeAccountProbeRequestMode(req.RequestMode)
	run.IncludeCodexStability = req.IncludeCodexStability
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
		sample := s.runOpenAIAPIKeySample(ctx, account, baseURL, model, key, planned, useResponses, run.RequestMode)
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

func (s *AccountProbeService) prepareRun(ctx context.Context, req AccountProbeRunRequest) (*Account, []string, accountProbePlan, string, string, bool, error) {
	if s.accounts == nil {
		return nil, nil, accountProbePlan{}, "", "", false, fmt.Errorf("account probe account repository is nil")
	}
	account, err := s.accounts.GetByID(ctx, req.AccountID)
	if err != nil {
		return nil, nil, accountProbePlan{}, "", "", false, err
	}
	if account == nil {
		return nil, nil, accountProbePlan{}, "", "", false, ErrAccountNotFound
	}
	if !account.IsOpenAIApiKey() {
		return nil, nil, accountProbePlan{}, "", "", false, fmt.Errorf("only openai api key accounts support probe runs")
	}
	keys := account.GetAPIKeys()
	if len(keys) == 0 {
		if key := strings.TrimSpace(account.GetOpenAIApiKey()); key != "" {
			keys = []string{key}
		}
	}
	if len(keys) == 0 {
		return nil, nil, accountProbePlan{}, "", "", false, fmt.Errorf("no api key available")
	}

	plan, err := buildAccountProbePlan(req)
	if err != nil {
		return nil, nil, accountProbePlan{}, "", "", false, err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = openai.DefaultTestModel
	}
	model = account.GetMappedModel(model)
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if s.testSvc != nil {
		normalized, err := s.testSvc.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return nil, nil, accountProbePlan{}, "", "", false, fmt.Errorf("invalid base URL: %w", err)
		}
		baseURL = normalized
	}

	useResponses := openai_compat.ShouldUseResponsesAPI(account.Extra)
	return account, keys, plan, model, baseURL, useResponses, nil
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
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	return s.repo.ListAccountProbeRuns(ctx, filter)
}

func (s *AccountProbeService) Get(ctx context.Context, accountID, runID int64) (*AccountProbeResult, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("account probe repository is nil")
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
		payload = map[string]any{
			"model":             model,
			"input":             sample.Prompt,
			"stream":            stream,
			"store":             false,
			"max_output_tokens": sample.MaxOutputTokens,
		}
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
	return result
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
				return result
			}
			return accountProbeOpenAIStreamResult{err: "Chat Completions stream ended before [DONE]"}
		}
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
	profile := normalizeAPIKeyProbeProfile(req.Profile)
	samples := make([]APIKeyProbePlannedSample, 0, 9)
	baseCount := 3
	if profile == AccountProbeProfileQuick {
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
