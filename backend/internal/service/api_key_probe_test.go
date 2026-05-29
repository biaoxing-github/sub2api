//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type apiKeyProbeKeyRepoStub struct {
	key *APIKey
}

func (r *apiKeyProbeKeyRepoStub) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	if r.key == nil || r.key.ID != id {
		return nil, ErrAPIKeyNotFound
	}
	out := *r.key
	return &out, nil
}

type apiKeyProbeRunnerStub struct {
	durations []time.Duration
	inputs    []int
	outputs   []int
	failUsage map[int]bool
	calls     []APIKeyProbeSampleRequest
}

func (r *apiKeyProbeRunnerStub) RunSample(ctx context.Context, req APIKeyProbeSampleRequest) (APIKeyProbeSampleResult, error) {
	idx := len(r.calls)
	r.calls = append(r.calls, req)
	result := APIKeyProbeSampleResult{
		Duration:     r.durations[idx],
		InputTokens:  r.inputs[idx],
		OutputTokens: r.outputs[idx],
		UsageLogID:   int64(100 + idx),
	}
	if r.failUsage[idx] {
		result.UsageLogID = 0
		result.UsageLogStatus = APIKeyProbeUsageLogMissing
		result.UsageLogMessage = "usage log was not persisted before probe collection timeout"
	}
	return result, nil
}

type apiKeyProbeSampleRepoStub struct {
	saved []APIKeyProbeSample
}

func (r *apiKeyProbeSampleRepoStub) SaveSample(ctx context.Context, sample APIKeyProbeSample) error {
	r.saved = append(r.saved, sample)
	return nil
}

func (r *apiKeyProbeSampleRepoStub) ListProbeSamples(ctx context.Context, runID int64) ([]APIKeyProbeSample, error) {
	return r.saved, nil
}

type apiKeyProbeHTTPClientCapture struct {
	request *http.Request
	body    string
}

func (c *apiKeyProbeHTTPClientCapture) Do(req *http.Request) (*http.Response, error) {
	c.request = req
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		c.body = string(data)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp-test"}`)),
		Header:     make(http.Header),
	}, nil
}

func TestAPIKeyProbeService_PlansStandardQuickAndOptionalSamples(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  APIKeyProbeRunRequest
		want int
	}{
		{
			name: "default standard",
			req:  APIKeyProbeRunRequest{UserID: 42, APIKeyID: 7},
			want: 3,
		},
		{
			name: "explicit quick",
			req:  APIKeyProbeRunRequest{UserID: 42, APIKeyID: 7, Profile: APIKeyProbeProfileQuick},
			want: 1,
		},
		{
			name: "standard plus codex stability",
			req:  APIKeyProbeRunRequest{UserID: 42, APIKeyID: 7, IncludeCodexStability: true},
			want: 8,
		},
		{
			name: "standard plus long context",
			req:  APIKeyProbeRunRequest{UserID: 42, APIKeyID: 7, IncludeLongContext: true},
			want: 4,
		},
		{
			name: "standard plus all optional suites",
			req:  APIKeyProbeRunRequest{UserID: 42, APIKeyID: 7, IncludeCodexStability: true, IncludeLongContext: true},
			want: 9,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewAPIKeyProbeService(APIKeyProbeDependencies{})
			plan, err := svc.Plan(context.Background(), tc.req)

			require.NoError(t, err)
			require.Equal(t, tc.want, plan.EstimatedRequests)
			require.Equal(t, tc.want, len(plan.Samples))
			require.Positive(t, plan.EstimatedInputTokensMin)
			require.GreaterOrEqual(t, plan.EstimatedInputTokensMax, plan.EstimatedInputTokensMin)
			require.Positive(t, plan.EstimatedOutputTokensMin)
			require.GreaterOrEqual(t, plan.EstimatedOutputTokensMax, plan.EstimatedOutputTokensMin)
		})
	}
}

func TestAPIKeyProbeService_RunAggregatesLatencyAndPersistsSamples(t *testing.T) {
	t.Parallel()

	runner := &apiKeyProbeRunnerStub{
		durations: []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond},
		inputs:    []int{10, 20, 30},
		outputs:   []int{2, 4, 6},
	}
	samples := &apiKeyProbeSampleRepoStub{}
	svc := NewAPIKeyProbeService(APIKeyProbeDependencies{
		APIKeys: &apiKeyProbeKeyRepoStub{key: &APIKey{ID: 7, UserID: 42, Key: "sk-test", Status: StatusActive}},
		Runner:  runner,
		Samples: samples,
	})

	result, err := svc.Run(context.Background(), APIKeyProbeRunRequest{UserID: 42, APIKeyID: 7})

	require.NoError(t, err)
	require.Equal(t, 3, result.Estimate.Requests)
	require.Equal(t, 3, len(runner.calls))
	require.Len(t, samples.saved, 3)
	require.Equal(t, 200, result.Latency.P50Millis)
	require.Equal(t, 400, result.Latency.P95Millis)
	require.Equal(t, 233, result.Latency.AvgMillis)
	require.Equal(t, 400, result.Latency.MaxMillis)
	require.Equal(t, 60, result.Usage.InputTokens)
	require.Equal(t, 12, result.Usage.OutputTokens)
	require.Equal(t, APIKeyProbeUsageLogPersisted, samples.saved[0].UsageLogStatus)
}

func TestAPIKeyProbeService_RunSavesSampleWhenUsageLogIsMissing(t *testing.T) {
	t.Parallel()

	runner := &apiKeyProbeRunnerStub{
		durations: []time.Duration{100 * time.Millisecond},
		inputs:    []int{12},
		outputs:   []int{3},
		failUsage: map[int]bool{0: true},
	}
	samples := &apiKeyProbeSampleRepoStub{}
	svc := NewAPIKeyProbeService(APIKeyProbeDependencies{
		APIKeys: &apiKeyProbeKeyRepoStub{key: &APIKey{ID: 7, UserID: 42, Key: "sk-test", Status: StatusActive}},
		Runner:  runner,
		Samples: samples,
	})

	result, err := svc.Run(context.Background(), APIKeyProbeRunRequest{
		UserID:   42,
		APIKeyID: 7,
		Profile:  APIKeyProbeProfileQuick,
	})

	require.NoError(t, err)
	require.Len(t, result.Samples, 1)
	require.Len(t, samples.saved, 1)
	require.Zero(t, samples.saved[0].UsageLogID)
	require.Equal(t, APIKeyProbeUsageLogMissing, samples.saved[0].UsageLogStatus)
	require.Contains(t, samples.saved[0].UsageLogMessage, "not persisted")
	require.Equal(t, APIKeyProbeSampleWarning, samples.saved[0].Status)
}

func TestHTTPAPIKeyProbeRunner_RunSampleUsesResponsesListInput(t *testing.T) {
	t.Parallel()

	client := &apiKeyProbeHTTPClientCapture{}
	runner := NewHTTPAPIKeyProbeRunner(client, nil)

	result, err := runner.RunSample(context.Background(), APIKeyProbeSampleRequest{
		APIKey:    &APIKey{ID: 7, Key: "sk-test"},
		BaseURL:   "https://example.test/v1",
		Model:     "gpt-test",
		RequestID: "client:req-1",
		Sample: APIKeyProbePlannedSample{
			Prompt:          "hi",
			Timeout:         time.Second,
			MaxOutputTokens: 20,
		},
	})

	require.NoError(t, err)
	require.Equal(t, APIKeyProbeUsageLogMissing, result.UsageLogStatus)
	require.Equal(t, "https://example.test/v1/responses", client.request.URL.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(client.body), &payload))
	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	textPart, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "input_text", textPart["type"])
	require.Equal(t, "hi", textPart["text"])
	require.NotEmpty(t, payload["instructions"])
}

func TestAPIKeyProbeService_RunRejectsKeysOwnedByAnotherUser(t *testing.T) {
	t.Parallel()

	runner := &apiKeyProbeRunnerStub{}
	svc := NewAPIKeyProbeService(APIKeyProbeDependencies{
		APIKeys: &apiKeyProbeKeyRepoStub{key: &APIKey{ID: 7, UserID: 99, Key: "sk-other", Status: StatusActive}},
		Runner:  runner,
	})

	_, err := svc.Run(context.Background(), APIKeyProbeRunRequest{UserID: 42, APIKeyID: 7})

	require.ErrorIs(t, err, ErrInsufficientPerms)
	require.Empty(t, runner.calls)
}

func TestAPIKeyProbeService_RunReturnsClearErrorWhenAPIKeyMissing(t *testing.T) {
	t.Parallel()

	svc := NewAPIKeyProbeService(APIKeyProbeDependencies{
		APIKeys: &apiKeyProbeKeyRepoStub{},
	})

	_, err := svc.Run(context.Background(), APIKeyProbeRunRequest{UserID: 42, APIKeyID: 404})

	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.False(t, errors.Is(err, ErrInsufficientPerms))
}
