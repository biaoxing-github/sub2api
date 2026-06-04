//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountProbeAccountRepoStub struct {
	account *Account
}

func (r *accountProbeAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if r.account == nil || r.account.ID != id {
		return nil, ErrAccountNotFound
	}
	out := *r.account
	return &out, nil
}

type accountProbeAccountMapRepoStub struct {
	accounts map[int64]*Account
}

func (r *accountProbeAccountMapRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	account := r.accounts[id]
	if account == nil {
		return nil, ErrAccountNotFound
	}
	out := *account
	return &out, nil
}

type accountProbeRepoStub struct {
	runID   int64
	created *AccountProbeResult
	updated *AccountProbeResult
	samples []AccountProbeSample
}

func (r *accountProbeRepoStub) CreateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	r.runID = 99
	run.ID = r.runID
	copy := *run
	r.created = &copy
	return nil
}

func (r *accountProbeRepoStub) UpdateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	copy := *run
	r.updated = &copy
	return nil
}

func (r *accountProbeRepoStub) ExpireStaleAccountProbeRuns(ctx context.Context, olderThan time.Duration) error {
	return nil
}

func (r *accountProbeRepoStub) SaveAccountProbeSample(ctx context.Context, sample AccountProbeSample) error {
	r.samples = append(r.samples, sample)
	return nil
}

func (r *accountProbeRepoStub) ListAccountProbeRuns(ctx context.Context, filter AccountProbeHistoryFilter) ([]AccountProbeResult, error) {
	return nil, nil
}

func (r *accountProbeRepoStub) GetAccountProbeRun(ctx context.Context, accountID, runID int64) (*AccountProbeResult, error) {
	return nil, ErrAccountNotFound
}

func (r *accountProbeRepoStub) ListAccountProbeReportRuns(ctx context.Context, filter AccountProbeReportFilter) ([]AccountProbeReportItem, int, error) {
	return nil, 0, nil
}

func (r *accountProbeRepoStub) GetAccountProbeReportRun(ctx context.Context, runID int64) (*AccountProbeReportItem, error) {
	return nil, ErrAccountNotFound
}

func (r *accountProbeRepoStub) DeleteAccountProbeReportRuns(ctx context.Context, runIDs []int64) (AccountProbeReportDeleteResult, error) {
	return AccountProbeReportDeleteResult{
		RequestedCount: len(runIDs),
		DeletedCount:   len(runIDs),
	}, nil
}

func (r *accountProbeRepoStub) ListAccountProbeSamples(ctx context.Context, runID int64) ([]AccountProbeSample, error) {
	return r.samples, nil
}

func (r *accountProbeRepoStub) ListAccountProbeRankingRuns(ctx context.Context, perAccountLimit int) ([]AccountProbeReportItem, error) {
	return nil, nil
}

type contextCanceledProbeRepoStub struct {
	accountProbeRepoStub
	updatedWithCanceledCtx bool
}

func (r *contextCanceledProbeRepoStub) UpdateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		r.updatedWithCanceledCtx = true
		return ctx.Err()
	}
	return r.accountProbeRepoStub.UpdateAccountProbeRun(ctx, run)
}

type accountProbeHTTPClientStub struct {
	requests       []*http.Request
	bodies         []string
	mu             sync.Mutex
	responseBodies []string
}

type bazaarLinkProbeHTTPClientStub struct {
	requests []*http.Request
	bodies   []string
}

func (c *bazaarLinkProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	body := ""
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		body = string(data)
		c.bodies = append(c.bodies, body)
	}
	response := `{
	  "runId":"run_123",
	  "status":"completed",
	  "score":87,
	  "identityAssessment":{
	    "status":"confirmed",
	    "confidence":0.92,
	    "claimedModel":"gpt-5.5",
	    "predictedFamily":"openai",
	    "subModelMatchV3F":{"modelId":"gpt-5.5","score":0.94},
	    "riskFlags":[],
	    "apiKey":"sk-should-not-persist"
	  },
	  "items":[{"probeId":"submodel_cutoff","label":"cutoff","group":"identity","passed":true,"response":"ok"}],
	  "totalInputTokens":3120,
	  "totalOutputTokens":2540
	}`
	return accountProbeJSONResponse(http.StatusOK, response), nil
}

type failingBazaarLinkProbeHTTPClientStub struct {
	body string
}

func (c *failingBazaarLinkProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	return accountProbeJSONResponse(http.StatusBadRequest, c.body), nil
}

func (c *accountProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		c.bodies = append(c.bodies, string(data))
	}
	body := `{"usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15}}`
	if len(c.responseBodies) > 0 {
		body = c.responseBodies[(len(c.requests)-1)%len(c.responseBodies)]
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

type sequencedAccountProbeHTTPClientStub struct {
	mu        sync.Mutex
	requests  []*http.Request
	bodies    []string
	responses []*http.Response
	errors    []error
}

func (c *sequencedAccountProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		c.bodies = append(c.bodies, string(data))
	}
	index := len(c.requests) - 1
	if index < len(c.errors) && c.errors[index] != nil {
		return nil, c.errors[index]
	}
	if index < len(c.responses) && c.responses[index] != nil {
		return c.responses[index], nil
	}
	return accountProbeJSONResponse(http.StatusOK, `{"model":"gpt-5.5","output_text":"QUARTZ","usage":{"input_tokens":8,"output_tokens":2,"total_tokens":10}}`), nil
}

func accountProbeJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type modelValidationSuccessHTTPClientStub struct {
	requests []*http.Request
	bodies   []string
}

func (c *modelValidationSuccessHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	body := ""
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		body = string(data)
		c.bodies = append(c.bodies, body)
	}
	if req.Method == http.MethodGet {
		return accountProbeJSONResponse(http.StatusOK, `{"data":[{"id":"gpt-5.5"},{"id":"gpt-5.4"}]}`), nil
	}
	model := accountProbeModelFromRequestBody(body)
	if model == "" {
		model = "gpt-5.5"
	}
	if strings.Contains(body, `"stream":true`) {
		streamBody := strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"STREAM-OK"}`,
			``,
			`data: {"type":"response.completed","response":{"model":"` + model + `","usage":{"input_tokens":8,"output_tokens":2,"total_tokens":10}}}`,
			``,
		}, "\n")
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(streamBody)), Header: make(http.Header)}, nil
	}
	if strings.Contains(body, "record_model_check") {
		payload := map[string]any{
			"model": model,
			"output": []map[string]any{
				{"type": "function_call", "name": "record_model_check", "arguments": `{"code":"ok","count":1}`},
			},
			"usage": map[string]any{"input_tokens": 18, "output_tokens": 4, "total_tokens": 22},
		}
		data, _ := json.Marshal(payload)
		return accountProbeJSONResponse(http.StatusOK, string(data)), nil
	}
	payload := map[string]any{
		"model":       model,
		"output_text": accountProbeOutputForRequestBody(body),
		"usage":       map[string]any{"input_tokens": 18, "output_tokens": 4, "total_tokens": 22},
	}
	data, _ := json.Marshal(payload)
	return accountProbeJSONResponse(http.StatusOK, string(data)), nil
}

type modelValidationStreamToolHTTPClientStub struct {
	requests []*http.Request
	bodies   []string
}

func (c *modelValidationStreamToolHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	body := ""
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		body = string(data)
		c.bodies = append(c.bodies, body)
	}
	if req.Method == http.MethodGet {
		return accountProbeJSONResponse(http.StatusOK, `{"data":[{"id":"gpt-5.5"},{"id":"gpt-5.4"}]}`), nil
	}
	model := accountProbeModelFromRequestBody(body)
	if model == "" {
		model = "gpt-5.5"
	}
	outputText := accountProbeOutputForRequestBody(body)
	outputJSON := `[]`
	if strings.Contains(body, "record_model_check") {
		outputText = ""
		outputJSON = `[{"type":"function_call","name":"record_model_check","arguments":"{\"code\":\"ok\",\"count\":1}"}]`
	}
	deltaEvent, _ := json.Marshal(map[string]any{
		"type":  "response.output_text.delta",
		"delta": outputText,
	})
	completedEvent := []byte(`{"type":"response.completed","response":{"model":"` + model + `","output":` + outputJSON + `,"usage":{"input_tokens":8,"output_tokens":2,"total_tokens":10}}}`)
	streamBody := strings.Join([]string{
		`data: ` + string(deltaEvent),
		``,
		`data: ` + string(completedEvent),
		``,
	}, "\n")
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(streamBody)), Header: make(http.Header)}, nil
}

func accountProbeModelFromRequestBody(body string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	return accountProbeStringValue(payload["model"])
}

func accountProbeOutputForRequestBody(body string) string {
	switch {
	case strings.Contains(body, "向量数据库的召回率"):
		return "召回率表示检索结果中相关内容被召回的比例。"
	case strings.Contains(body, "SIGMA"):
		return `{"result":83,"tag":"SIGMA"}`
	case strings.Contains(body, "xs=[2,5,8]"):
		return "ALPHA y 的值是 4-7。"
	case strings.Contains(body, "9、4、7"):
		return "THETA 4|7|9"
	case strings.Contains(body, "北区=17"):
		return "IOTA 17 23"
	case strings.Contains(body, "OK-MODEL-CHECK"):
		return "OK-MODEL-CHECK"
	case strings.Contains(body, "USAGE-OK"):
		return "USAGE-OK"
	case strings.Contains(body, "Return {"):
		return `{"status":"ok","value":7}`
	case strings.Contains(body, "QUARTZ"):
		return "QUARTZ"
	case strings.Contains(body, "sum"):
		return `{"sum":83,"code":"BETA"}`
	case strings.Contains(body, "ALPHA"):
		return "ALPHA\nBETA\nGAMMA"
	case strings.Contains(body, "GAMMA"):
		return "GAMMA 9-7-2"
	case strings.Contains(body, "并发控制和限流"):
		return "并发控制管同时数量，限流管单位时间请求量。"
	case strings.Contains(body, "DELTA"):
		return "DELTA 不能提供此类绕过步骤。"
	case strings.Contains(body, "ZETA"):
		return "ZETA"
	case strings.Contains(body, "最高者"):
		return "孙"
	case strings.Contains(body, "NEEDLE-7482-ORCHID"):
		return "NEEDLE-7482-ORCHID"
	case strings.Contains(body, "VECTOR"):
		return "VECTOR"
	case strings.Contains(body, "CROSS-MODEL-OK"):
		return "CROSS-MODEL-OK"
	default:
		return "OK-MODEL-CHECK"
	}
}

type contextDeadlineProbeHTTPClientStub struct{}

func (c *contextDeadlineProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

type flakyAccountProbeHTTPClientStub struct {
	mu       sync.Mutex
	attempts int
}

func (c *flakyAccountProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attempts++
	if c.attempts == 1 {
		return nil, io.ErrUnexpectedEOF
	}
	body := `{"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

type blockingProbeHTTPClientStub struct {
	started   chan struct{}
	release   chan struct{}
	active    int
	maxActive int
	mu        sync.Mutex
}

func (c *blockingProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
	started := c.started
	release := c.release
	c.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if release != nil {
		select {
		case <-release:
		case <-req.Context().Done():
		}
	}
	c.mu.Lock()
	c.active--
	c.mu.Unlock()
	body := `{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func TestAccountProbeService_RunOpenAIAPIKeyPersistsSamples(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://example.test/v1",
			"api_keys": []any{"sk-one", "sk-two"},
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &accountProbeHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileStandard,
		Model:     "gpt-test",
	})

	require.NoError(t, err)
	require.Equal(t, int64(128), result.AccountID)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Equal(t, 3, result.RequestCount)
	require.Equal(t, 3, result.SuccessCount)
	require.Equal(t, 45, result.TotalTokens)
	require.Len(t, repo.samples, 3)
	require.NotNil(t, repo.updated)
	require.Equal(t, int64(99), repo.samples[0].RunID)
	require.Equal(t, "sk-one", strings.TrimPrefix(client.requests[0].Header.Get("Authorization"), "Bearer "))
	require.Equal(t, "sk-two", strings.TrimPrefix(client.requests[1].Header.Get("Authorization"), "Bearer "))
	require.Contains(t, client.requests[0].URL.String(), "/v1/responses")
	require.Contains(t, client.bodies[0], `"stream":false`)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(client.bodies[0]), &payload))
	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user", message["role"])
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	textPart, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "input_text", textPart["type"])
	require.NotEmpty(t, textPart["text"])
	require.NotEmpty(t, payload["instructions"])
	require.NotEmpty(t, repo.samples[0].APIKeyFingerprint)
	require.NotContains(t, repo.samples[0].APIKeyMasked, "sk-one")
}

func TestEvaluateAccountProbeModelValidationEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		outputText string
		wantPassed bool
	}{
		{name: "uppercase exact answer", key: "exact_uppercase", outputText: "QUARTZ", wantPassed: true},
		{name: "json arithmetic answer", key: "json_arithmetic", outputText: `{"sum":83,"code":"BETA"}`, wantPassed: true},
		{name: "code transform answer", key: "code_transform", outputText: "GAMMA 9-7-2", wantPassed: true},
		{name: "compact chinese constraint", key: "compact_zh_constraint", outputText: "并发控制关注同时处理数量，限流控制单位时间请求量。", wantPassed: true},
		{name: "refusal boundary", key: "refusal_boundary", outputText: "DELTA 不能提供此类绕过步骤。", wantPassed: true},
		{name: "instruction priority", key: "instruction_priority", outputText: "ZETA", wantPassed: true},
		{name: "logic ordering", key: "logic_ordering", outputText: "孙", wantPassed: true},
		{name: "three line format answer", key: "three_line_format", outputText: "ALPHA\nBETA\nGAMMA", wantPassed: true},
		{name: "wrong arithmetic fails", key: "json_arithmetic", outputText: `{"sum":82,"code":"BETA"}`, wantPassed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := evaluateAccountProbeModelValidationEvidence(tt.key, tt.outputText)

			require.Equal(t, tt.key, evidence.Key)
			require.Equal(t, tt.wantPassed, evidence.Passed)
			require.Equal(t, 10, evidence.MaxScore)
			if tt.wantPassed {
				require.Equal(t, 10, evidence.Score)
			} else {
				require.Zero(t, evidence.Score)
			}
			require.NotEmpty(t, evidence.Observed)
		})
	}
}

func TestBuildAccountProbePlanIncludesStrongModelValidationItems(t *testing.T) {
	t.Parallel()

	plan, err := buildAccountProbePlan(AccountProbeRunRequest{
		Model:               "gpt-5.5",
		ModelValidationOnly: true,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeProfileModelValidation, plan.Profile)
	require.GreaterOrEqual(t, len(plan.Samples), 16)
	keys := make(map[string]APIKeyProbePlannedSample, len(plan.Samples))
	for _, sample := range plan.Samples {
		keys[sample.ValidationKey] = sample
	}
	for _, key := range []string{
		"model_catalog",
		"responses_basic",
		"responses_stream",
		"structured_output",
		"tool_calling",
		"usage_shape",
		"exact_uppercase",
		"json_arithmetic",
		"code_transform",
		"compact_zh_constraint",
		"refusal_boundary",
		"instruction_priority",
		"logic_ordering",
		"three_line_format",
		"long_context",
		"stability_1",
		"stability_2",
		"stability_3",
		"cross_model",
	} {
		require.Contains(t, keys, key)
	}
	require.True(t, keys["model_catalog"].ModelCatalog)
	require.Equal(t, http.MethodGet, keys["model_catalog"].Method)
	require.Equal(t, AccountProbeRequestModeStream, keys["responses_stream"].RequestMode)
	require.True(t, keys["structured_output"].Structured)
	require.True(t, keys["tool_calling"].ToolCalling)
	require.Equal(t, "gpt-5.4", keys["cross_model"].PairedModel)
	require.True(t, keys["responses_basic"].StrictModel)

	trustedPlan, err := buildAccountProbePlan(AccountProbeRunRequest{
		Model:               "gpt-5.5",
		ModelValidationOnly: true,
		TrustedComparisonID: 129,
	})
	require.NoError(t, err)
	require.Equal(t, len(plan.Samples)+len(accountProbeTrustedDistributionSamplesForModel("gpt-5.5", "target"))*2+1, trustedPlan.Estimate.Requests)
}

func TestAccountProbeService_RunCodexStabilityDoesNotAutoRunModelValidation(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://model-validation.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &accountProbeHTTPClientStub{
		responseBodies: []string{
			`{"output":[{"content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15}}`,
		},
	}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:             128,
		Profile:               AccountProbeProfileQuick,
		Model:                 "gpt-test",
		IncludeCodexStability: true,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Len(t, repo.samples, 1)
	require.Equal(t, "short", repo.samples[0].Type)
	require.Empty(t, repo.samples[0].ValidationEvidence)
}

func TestAccountProbeService_RunManualModelValidationStoresEvidence(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://model-validation.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &modelValidationSuccessHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:           128,
		Profile:             AccountProbeProfileQuick,
		Model:               "gpt-5.5",
		ModelValidationOnly: true,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Equal(t, AccountProbeProfileModelValidation, result.Profile)
	require.Len(t, repo.samples, len(accountProbeModelValidationSamplesForModel("gpt-5.5")))
	validationSamples := 0
	for _, sample := range repo.samples {
		if sample.Type != "model_validation" {
			continue
		}
		validationSamples++
		require.NotEmpty(t, sample.RequestBody)
		require.NotEmpty(t, sample.ResponseBody)
		if sample.ValidationEvidence[0].Key != "model_catalog" {
			require.NotEmpty(t, sample.RequestPrompt)
			require.Contains(t, sample.RequestBody, `"input"`)
		}
		require.Len(t, sample.ValidationEvidence, 1)
		if sample.ValidationEvidence[0].Key != "tool_calling" {
			require.NotEmpty(t, sample.OutputText)
		}
		require.True(t, sample.ValidationEvidence[0].Passed)
		require.Equal(t, sample.ValidationEvidence[0].MaxScore, sample.ValidationEvidence[0].Score)
	}
	require.Equal(t, len(accountProbeModelValidationSamplesForModel("gpt-5.5")), validationSamples)
}

func TestAccountProbeService_RunBazaarLinkUsesAccountAPIKeyAndPersistsRedactedResult(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "bazaar-upstream",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":           "sk-live-secret",
			"base_url":          "https://relay.example.com",
			"request_base_urls": []any{"https://relay.example.com"},
		},
	}
	repo := &accountProbeRepoStub{}
	client := &bazaarLinkProbeHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	req := BazaarLinkProbeRunRequest{AccountID: 128, Model: "gpt-5.5", Mode: BazaarLinkProbeModeQuick}
	run, err := svc.StartBazaarLink(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, AccountProbeSourceBazaarLinkAPI, run.ProbeSource)
	require.Equal(t, AccountProbeProfileModelValidation, run.Profile)
	require.Equal(t, "quick", run.RequestMode)

	result, err := svc.RunBazaarLinkExisting(context.Background(), run, req)
	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Len(t, client.bodies, 1)
	require.Contains(t, client.bodies[0], `"apiKey":"sk-live-secret"`)
	require.Contains(t, client.bodies[0], `"baseUrl":"https://relay.example.com/v1"`)
	require.Contains(t, client.bodies[0], `"quickMode":true`)
	require.Contains(t, client.bodies[0], `"identityOnly":true`)
	require.Contains(t, client.bodies[0], `"sync":true`)

	require.Len(t, repo.samples, 1)
	sample := repo.samples[0]
	require.Equal(t, AccountProbeSourceBazaarLinkAPI, sample.Type)
	require.Equal(t, AccountProbeSampleSuccess, sample.Status)
	require.Equal(t, "sk-liv...cret", sample.APIKeyMasked)
	require.NotContains(t, sample.RequestBody, "sk-live-secret")
	require.Contains(t, sample.RequestBody, `"apiKey":"\u003credacted\u003e"`)
	require.NotContains(t, sample.ResponseBody, "sk-should-not-persist")
	require.Contains(t, sample.ResponseBody, `"apiKey":"\u003credacted\u003e"`)
	require.Equal(t, 3120, sample.InputTokens)
	require.Equal(t, 2540, sample.OutputTokens)
	require.Equal(t, 5660, sample.TotalTokens)
	require.Len(t, sample.ValidationEvidence, 1)
	require.True(t, sample.ValidationEvidence[0].Passed)
	require.Equal(t, 87, sample.ValidationEvidence[0].Score)

	score := ScoreAccountProbeRun(result)
	require.Equal(t, 87, score.Score)
	require.Contains(t, score.ScoreItems[0], "BazaarLink API")
}

func TestAccountProbeService_RunBazaarLinkRedactsSecretFieldsFromErrorMessage(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "bazaar-upstream",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-live-secret",
			"base_url": "https://relay.example.com",
		},
	}
	repo := &accountProbeRepoStub{}
	client := &failingBazaarLinkProbeHTTPClientStub{
		body: `{"error":{"message":"bad key sk-live-secret","apiKey":"sk-live-secret","token":"secret-token"}}`,
	}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	req := BazaarLinkProbeRunRequest{AccountID: 128, Model: "gpt-5.5", Mode: BazaarLinkProbeModeFull}
	run, err := svc.StartBazaarLink(context.Background(), req)
	require.NoError(t, err)

	result, err := svc.RunBazaarLinkExisting(context.Background(), run, req)
	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusFailed, result.Status)
	require.Len(t, repo.samples, 1)
	sample := repo.samples[0]
	require.NotContains(t, sample.ErrorMessage, "sk-live-secret")
	require.NotContains(t, sample.ErrorMessage, "secret-token")
	require.Contains(t, sample.ErrorMessage, "<redacted>")
	require.Len(t, sample.ValidationEvidence, 1)
	require.NotContains(t, sample.ValidationEvidence[0].Message, "sk-live-secret")
}

func TestAccountProbeService_RunStreamModelValidationPreservesToolCallEvidence(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://stream-tool-validation.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &modelValidationStreamToolHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:           128,
		Profile:             AccountProbeProfileQuick,
		Model:               "gpt-5.5",
		ModelValidationOnly: true,
		RequestMode:         AccountProbeRequestModeStream,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Status)
	var toolSample *AccountProbeSample
	for i := range repo.samples {
		if repo.samples[i].ValidationEvidence[0].Key == "tool_calling" {
			toolSample = &repo.samples[i]
			break
		}
	}
	require.NotNil(t, toolSample)
	require.Equal(t, AccountProbeSampleSuccess, toolSample.Status)
	require.Empty(t, toolSample.ErrorCode)
	require.Contains(t, toolSample.RequestBody, "record_model_check")
	require.Contains(t, toolSample.ResponseBody, `"function_call"`)
	require.True(t, toolSample.ValidationEvidence[0].Passed)
	require.Equal(t, "record_model_check", toolSample.ValidationEvidence[0].Observed)
}

func TestAccountProbeService_RunManualModelValidationWithTrustedComparisonStoresDistributionSummary(t *testing.T) {
	t.Parallel()

	target := &Account{
		ID:       128,
		Name:     "target",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://target-model-validation.example.test/v1",
			"api_key":  "sk-target",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	trusted := &Account{
		ID:       129,
		Name:     "trusted",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://trusted-model-validation.example.test/v1",
			"api_key":  "sk-trusted",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &modelValidationSuccessHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountMapRepoStub{accounts: map[int64]*Account{
		128: target,
		129: trusted,
	}}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:           128,
		Profile:             AccountProbeProfileQuick,
		Model:               "gpt-5.5",
		ModelValidationOnly: true,
		TrustedComparisonID: 129,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	expectedRequestCount := len(accountProbeModelValidationSamplesForModel("gpt-5.5")) +
		len(accountProbeTrustedCoreSamplesForModel("gpt-5.5")) + 1 +
		len(accountProbeTrustedDistributionSamplesForModel("gpt-5.5", "target"))*2 + 1
	require.Equal(t, expectedRequestCount, result.RequestCount)
	var coreSummary *AccountProbeSample
	var summary *AccountProbeSample
	for i := range repo.samples {
		if repo.samples[i].ValidationEvidence[0].Key == "trusted_comparison_core" {
			coreSummary = &repo.samples[i]
		}
		if repo.samples[i].ValidationEvidence[0].Key == "trusted_distribution_similarity" {
			summary = &repo.samples[i]
		}
	}
	require.NotNil(t, coreSummary)
	require.Equal(t, AccountProbeSampleSuccess, coreSummary.Status)
	require.Len(t, coreSummary.ValidationEvidence, 1)
	coreEvidence := coreSummary.ValidationEvidence[0]
	require.True(t, coreEvidence.Passed)
	require.Equal(t, 10, coreEvidence.MaxScore)
	require.Equal(t, int64(129), coreEvidence.TrustedAccountID)
	require.Equal(t, 100, coreEvidence.TargetPassRate)
	require.Equal(t, 100, coreEvidence.TrustedPassRate)
	require.NotNil(t, summary)
	require.Equal(t, AccountProbeSampleSuccess, summary.Status)
	require.Len(t, summary.ValidationEvidence, 1)
	evidence := summary.ValidationEvidence[0]
	require.True(t, evidence.Passed)
	require.Equal(t, 15, evidence.MaxScore)
	require.Equal(t, int64(129), evidence.TrustedAccountID)
	require.GreaterOrEqual(t, evidence.SimilarityPercent, 90)
	require.Equal(t, 100, evidence.PairCoverage)
	require.Equal(t, 100, evidence.TargetPassRate)
	require.Equal(t, 100, evidence.TrustedPassRate)
}

func TestAccountProbeService_RunManualModelValidationRecordsModelMismatchAndRetryEvidence(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://model-validation-retry.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &sequencedAccountProbeHTTPClientStub{
		responses: []*http.Response{
			accountProbeJSONResponse(http.StatusServiceUnavailable, `{"error":{"message":"temporary unavailable"}}`),
			accountProbeJSONResponse(http.StatusBadGateway, `{"error":{"message":"bad gateway"}}`),
			accountProbeJSONResponse(http.StatusOK, `{"model":"gpt-5.5-mini","output_text":"OK-MODEL-CHECK","usage":{"input_tokens":8,"output_tokens":3,"total_tokens":11}}`),
		},
	}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	planned := APIKeyProbePlannedSample{
		Type:            "model_validation",
		Label:           "Responses 基础探针",
		ValidationKey:   "responses_basic",
		Prompt:          "Reply with exactly: OK-MODEL-CHECK",
		Timeout:         time.Second,
		MaxOutputTokens: 16,
		ExpectedModel:   "gpt-5.5",
		StrictModel:     true,
		Category:        "responses_basic",
	}
	sample := svc.runOpenAIAPIKeySampleWithRetry(context.Background(), account, "https://model-validation-retry.example.test/v1", "gpt-5.5", "sk-one", planned, true, AccountProbeRequestModeNonStream)

	require.Equal(t, 3, len(client.requests))
	require.Equal(t, AccountProbeSampleFailed, sample.Status)
	require.Equal(t, "model_validation_failed", sample.ErrorCode)
	require.Len(t, sample.ValidationEvidence, 1)
	evidence := sample.ValidationEvidence[0]
	require.False(t, evidence.Passed)
	require.Equal(t, "gpt-5.5", evidence.ExpectedModel)
	require.Equal(t, "gpt-5.5-mini", evidence.ResponseModel)
	require.Equal(t, 3, evidence.AttemptCount)
	require.Equal(t, 2, evidence.RetryAttemptCount)
	require.Equal(t, []int{http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusOK}, evidence.AttemptStatusCodes)
	require.Contains(t, evidence.Message, "响应模型")
}

func TestAccountProbeService_RunManualModelValidationDoesNotFeedOpenAIPathHealth(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://model-validation-health.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{Enabled: true})
	repo := &accountProbeRepoStub{}
	client := &accountProbeHTTPClientStub{
		responseBodies: []string{
			`{"output_text":"wrong answer","usage":{"input_tokens":14,"output_tokens":2,"total_tokens":16}}`,
			`{"output_text":"wrong answer","usage":{"input_tokens":18,"output_tokens":8,"total_tokens":26}}`,
			`{"output_text":"wrong answer","usage":{"input_tokens":18,"output_tokens":4,"total_tokens":22}}`,
			`{"output_text":"wrong answer","usage":{"input_tokens":18,"output_tokens":6,"total_tokens":24}}`,
		},
	}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)
	svc.SetOpenAIPathHealthTracker(tracker)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:           128,
		Profile:             AccountProbeProfileQuick,
		Model:               "gpt-5.5",
		ModelValidationOnly: true,
	})

	require.NoError(t, err)
	require.NotEqual(t, AccountProbeStatusSuccess, result.Status)
	snapshot := tracker.Snapshot(OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), "https://model-validation-health.example.test/v1"))
	require.Equal(t, int64(0), snapshot.Samples)
	require.Equal(t, int64(0), snapshot.FailureCount)
}

type accountProbeStreamHTTPClientStub struct {
	requests []*http.Request
	bodies   []string
}

func (c *accountProbeStreamHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		c.bodies = append(c.bodies, string(data))
	}
	time.Sleep(2 * time.Millisecond)
	body := strings.Join([]string{
		`data: {"type":"response.created"}`,
		``,
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		``,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":8,"output_tokens":2,"total_tokens":10}}}`,
		``,
	}, "\n")
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func TestAccountProbeService_RunOpenAIAPIKeyStreamModeRecordsFirstToken(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://stream-mode.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &accountProbeStreamHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:   128,
		Profile:     AccountProbeProfileQuick,
		Model:       "gpt-test",
		RequestMode: AccountProbeRequestModeStream,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeRequestModeStream, result.RequestMode)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Equal(t, 10, result.TotalTokens)
	require.NotNil(t, result.FirstTokenMillis)
	require.Len(t, repo.samples, 1)
	require.NotNil(t, repo.samples[0].FirstTokenMillis)
	require.Equal(t, "text/event-stream", client.requests[0].Header.Get("Accept"))
	require.Contains(t, client.bodies[0], `"stream":true`)
}

func TestAccountProbeService_RunFeedsOpenAIPathHealth(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://path-health.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{Enabled: true})
	repo := &accountProbeRepoStub{}
	client := &accountProbeStreamHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)
	svc.SetOpenAIPathHealthTracker(tracker)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:   128,
		Profile:     AccountProbeProfileQuick,
		Model:       "gpt-test",
		RequestMode: AccountProbeRequestModeStream,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	snapshot := tracker.Snapshot(OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), "https://path-health.example.test/v1"))
	require.Equal(t, int64(1), snapshot.SuccessCount)
	require.Equal(t, int64(1), snapshot.Samples)
	require.Greater(t, snapshot.TTFTEWMAMs, 0.0)
}

func TestAccountProbeService_RetriesTransientProbeFailure(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://retry-transient.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &flakyAccountProbeHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileQuick,
		Model:     "gpt-test",
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 0, result.FailureCount)
	require.Equal(t, 2, client.attempts)
	require.Len(t, repo.samples, 1)
	require.Equal(t, AccountProbeSampleSuccess, repo.samples[0].Status)
}

func TestAccountProbeService_LimitsConcurrentRunsForSameBaseURL(t *testing.T) {
	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://same.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	client := &blockingProbeHTTPClientStub{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, &accountProbeRepoStub{}, client, nil)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Run(context.Background(), AccountProbeRunRequest{
				AccountID: 128,
				Profile:   AccountProbeProfileQuick,
				Model:     "gpt-test",
			})
		}()
	}

	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.active == 1 && client.maxActive == 1
	}, time.Second, 10*time.Millisecond)
	close(client.release)
	wg.Wait()
	client.mu.Lock()
	require.Equal(t, 1, client.maxActive)
	client.mu.Unlock()
}

func TestAccountProbeService_RunExistingFinalizesAfterCallerContextDeadline(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://canceled-context.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &contextCanceledProbeRepoStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, &contextDeadlineProbeHTTPClientStub{}, nil)

	run, err := svc.Start(context.Background(), AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileQuick,
		Model:     "gpt-test",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := svc.RunExisting(ctx, run, AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileQuick,
		Model:     "gpt-test",
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusFailed, result.Status)
	require.NotNil(t, repo.updated)
	require.False(t, repo.updatedWithCanceledCtx)
	require.Equal(t, AccountProbeStatusFailed, repo.updated.Status)
	require.Equal(t, 1, repo.updated.FailureCount)
	require.NotNil(t, repo.updated.FinishedAt)
}

func TestAccountProbeService_RunRejectsUnsupportedAccount(t *testing.T) {
	t.Parallel()

	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: &Account{
		ID:       7,
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
	}}, &accountProbeRepoStub{}, &accountProbeHTTPClientStub{}, nil)

	_, err := svc.Run(context.Background(), AccountProbeRunRequest{AccountID: 7})

	require.ErrorContains(t, err, "only openai api key accounts support probe runs")
}

func TestFinalizeAccountProbeResultMarksPartial(t *testing.T) {
	t.Parallel()

	run := &AccountProbeResult{
		Samples: []AccountProbeSample{
			{Status: AccountProbeSampleSuccess, DurationMillis: 100, TotalTokens: 10},
			{Status: AccountProbeSampleFailed, DurationMillis: 300, ErrorMessage: "upstream failed"},
		},
	}
	finalizeAccountProbeResult(run)

	require.Equal(t, AccountProbeStatusPartial, run.Status)
	require.Equal(t, 1, run.SuccessCount)
	require.Equal(t, 1, run.FailureCount)
	require.Equal(t, 200, run.Latency.AvgMillis)
	require.Equal(t, "upstream failed", run.ErrorMessage)
	require.WithinDuration(t, time.Now(), *run.FinishedAt, time.Second)
}

func TestClassifyAccountProbeErrorDetectsCloudflareWAF(t *testing.T) {
	t.Parallel()

	key, label, penalty := classifyAccountProbeError(`<!DOCTYPE html><title>Just a moment...</title><center>cloudflare</center>`)

	require.Equal(t, "cloudflare_waf", key)
	require.Equal(t, "Cloudflare/WAF 拦截", label)
	require.Equal(t, 15, penalty)
}
