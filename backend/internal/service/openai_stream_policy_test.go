package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIStreamInterceptDecisionUsesBuiltInRules(t *testing.T) {
	cases := []struct {
		name               string
		payload            []byte
		message            string
		wantRuleID         string
		wantAction         OpenAIStreamActionLabel
		wantMatchField     string
		wantReasonCategory string
	}{
		{
			name:               "quota code avoids account",
			payload:            []byte(`{"type":"response.failed","response":{"error":{"code":"insufficient_quota","message":"You exceeded your current quota."}}}`),
			wantRuleID:         "openai_stream_quota_or_billing",
			wantAction:         OpenAIStreamActionAvoidAccountTTL,
			wantMatchField:     "response.error.code",
			wantReasonCategory: UpstreamErrorCategoryQuota,
		},
		{
			name:               "capacity text avoids upstream bucket",
			payload:            []byte(`{"type":"response.failed","error":{"type":"server_error","message":"Selected model is at capacity. Please try a different model."}}`),
			wantRuleID:         "openai_stream_capacity_or_overload",
			wantAction:         OpenAIStreamActionAvoidUpstreamBucketTTL,
			wantMatchField:     "error.message",
			wantReasonCategory: UpstreamErrorCategoryUpstreamError,
		},
		{
			name:               "safety policy only retries next account",
			payload:            []byte(`{"type":"response.failed","error":{"type":"safety_error","message":"Request denied by safety policy."}}`),
			wantRuleID:         "openai_stream_policy_or_invalid_request",
			wantAction:         OpenAIStreamActionRetryNextAccount,
			wantMatchField:     "error.type",
			wantReasonCategory: UpstreamErrorCategoryBusinessLimited,
		},
		{
			name:               "unknown response failed retries next account",
			payload:            []byte(`{"type":"response.failed","error":{"message":"upstream rejected request"}}`),
			message:            "upstream rejected request",
			wantRuleID:         "openai_stream_default_response_failed",
			wantAction:         OpenAIStreamActionRetryNextAccount,
			wantMatchField:     "event.type",
			wantReasonCategory: UpstreamErrorCategoryUpstreamError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := openAIClassifyStreamInterceptDecision(tc.payload, tc.message)

			require.True(t, decision.FailoverBeforeOutput)
			require.True(t, decision.GatewayRetryableAfterOutput)
			require.True(t, decision.DropOriginalEvent)
			require.Equal(t, tc.wantRuleID, decision.RuleID)
			require.Equal(t, tc.wantAction, decision.ActionLabel)
			require.Equal(t, tc.wantMatchField, decision.MatchField)
			require.Equal(t, tc.wantReasonCategory, decision.ReasonCategory)
			require.NotEmpty(t, decision.DescriptionKey)
			require.Equal(t, tc.wantRuleID, decision.Metadata()["stream_rule_id"])
			require.Equal(t, string(tc.wantAction), decision.Metadata()["stream_action"])
		})
	}
}

func TestOpenAIUpstreamErrorPolicySeparatesPhasesAndActions(t *testing.T) {
	cases := []struct {
		name       string
		phase      openAIUpstreamErrorPolicyPhase
		input      UpstreamErrorInput
		wantAction OpenAIStreamActionLabel
		wantScope  string
		wantRecord bool
	}{
		{
			name:       "request phase timeout retries request only",
			phase:      openAIUpstreamErrorPolicyPhaseRequest,
			input:      UpstreamErrorInput{Err: errors.New("context deadline exceeded while awaiting response headers")},
			wantAction: OpenAIStreamActionRetryNextAccount,
			wantScope:  "request",
			wantRecord: true,
		},
		{
			name:       "http quota avoids account ttl",
			phase:      openAIUpstreamErrorPolicyPhaseHTTPResponse,
			input:      UpstreamErrorInput{StatusCode: http.StatusPaymentRequired, Message: "insufficient balance"},
			wantAction: OpenAIStreamActionAvoidAccountTTL,
			wantScope:  "account",
			wantRecord: false,
		},
		{
			name:       "http cloudflare avoids upstream bucket",
			phase:      openAIUpstreamErrorPolicyPhaseHTTPResponse,
			input:      UpstreamErrorInput{StatusCode: http.StatusForbidden, Body: []byte(`<html>Just a moment... cf-ray</html>`)},
			wantAction: OpenAIStreamActionAvoidUpstreamBucketTTL,
			wantScope:  "upstream_bucket",
			wantRecord: true,
		},
		{
			name:       "stream policy rejection only retries next account",
			phase:      openAIUpstreamErrorPolicyPhaseStream,
			input:      UpstreamErrorInput{Message: "request denied by safety policy"},
			wantAction: OpenAIStreamActionRetryNextAccount,
			wantScope:  "account",
			wantRecord: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := classifyOpenAIUpstreamErrorPolicy(tc.phase, tc.input)

			require.Equal(t, tc.wantAction, decision.ActionLabel)
			require.Equal(t, tc.wantScope, decision.AvoidanceScope)
			require.Equal(t, tc.wantRecord, decision.RecordPathHealth)
			require.NotEmpty(t, decision.Category)
			require.Equal(t, string(tc.phase), decision.Metadata()["error_phase"])
			require.Equal(t, string(tc.wantAction), decision.Metadata()["stream_action"])
		})
	}
}
