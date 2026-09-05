package service

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpstreamModelMismatchTreatsGrokBuildRuntimeIDsAsAliases(t *testing.T) {
	tests := []struct {
		name          string
		sentModel     string
		responseModel string
	}{
		{
			name:          "issue 5634 grok 4.6",
			sentModel:     "grok-4.6",
			responseModel: "grok-4.6-build",
		},
		{
			name:          "grok 4.6 latest",
			sentModel:     "grok-4.6-latest",
			responseModel: "grok-4.6-build",
		},
		{
			name:          "issue 5647 grok 4.5 latest",
			sentModel:     "grok-4.5-latest",
			responseModel: "grok-4.5-build",
		},
		{
			name:          "grok 4.5 canonical",
			sentModel:     "grok-4.5",
			responseModel: "GROK-4.5-BUILD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mismatch := upstreamModelMismatch(tt.sentModel, tt.responseModel)

			require.NotNil(t, mismatch)
			require.False(t, *mismatch)
		})
	}
}

func TestUpstreamModelMismatchDoesNotCollapseDifferentModels(t *testing.T) {
	tests := []struct {
		name          string
		sentModel     string
		responseModel string
	}{
		{
			name:          "different grok versions",
			sentModel:     "grok-4.5",
			responseModel: "grok-4.6-build",
		},
		{
			name:          "unrelated build suffix",
			sentModel:     "gpt-5.5",
			responseModel: "gpt-5.5-build",
		},
		{
			name:          "different grok runtime",
			sentModel:     "grok-build-0.1",
			responseModel: "grok-4.5-build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mismatch := upstreamModelMismatch(tt.sentModel, tt.responseModel)

			require.NotNil(t, mismatch)
			require.True(t, *mismatch)
		})
	}
}

func TestUpstreamResponseModelObserver(t *testing.T) {
	observer := &upstreamResponseModelObserver{}
	observer.ObserveOpenAI([]byte(`{"type":"response.created","response":{"model":"gpt-5.5"}}`), "response.created")
	observer.ObserveOpenAI([]byte(`{"type":"response.completed","response":{"model":"gpt-5.4"}}`), "response.completed")
	require.Equal(t, "gpt-5.4", observer.Model())
	require.True(t, observer.Conflict())
}

// TestUpstreamResponseModelObserverServiceTierWithoutModel 验证无模型响应的档位仍独立参与观察。
func TestUpstreamResponseModelObserverServiceTierWithoutModel(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		eventType string
		wantTier  string
	}{
		{name: "chat usage chunk", payload: `{"service_tier":"default","choices":[],"usage":{"total_tokens":3}}`, wantTier: "default"},
		{name: "responses completed", payload: `{"response":{"service_tier":"priority"}}`, eventType: "response.completed", wantTier: "priority"},
		{name: "responses failed", payload: `{"response":{"service_tier":"flex"}}`, eventType: "response.failed", wantTier: "flex"},
		{name: "request echo ignored", payload: `{"response":{"service_tier":"priority"}}`, eventType: "response.created"},
		{name: "unknown tier ignored", payload: `{"service_tier":"auto"}`},
		{name: "malformed JSON ignored", payload: `{"service_tier":"priority"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer := &upstreamResponseModelObserver{}
			observer.ObserveOpenAI([]byte(tt.payload), tt.eventType)
			require.Equal(t, tt.wantTier, observer.ServiceTier())
			require.Empty(t, observer.Model())
			require.False(t, observer.Conflict())
		})
	}

	observer := &upstreamResponseModelObserver{}
	observer.ObserveOpenAI([]byte(`{"service_tier":"default"}`), "")
	observer.ObserveOpenAI([]byte(`{"service_tier":"priority"}`), "")
	require.Empty(t, observer.ServiceTier())
	observer.ObserveOpenAI([]byte(`{"response":{"service_tier":"flex"}}`), "response.completed")
	require.Equal(t, "flex", observer.ServiceTier())
}

func TestUpstreamResponseModelObserverProviderShapes(t *testing.T) {
	anthropic := &upstreamResponseModelObserver{}
	anthropic.ObserveAnthropic([]byte(`{"message":{"model":"claude-sonnet-4"}}`))
	require.Equal(t, "claude-sonnet-4", anthropic.Model())
	gemini := &upstreamResponseModelObserver{}
	gemini.ObserveGemini([]byte(`{"modelVersion":"gemini-3.6-flash"}`))
	require.Equal(t, "gemini-3.6-flash", gemini.Model())
}

func TestObserveAntigravityGeminiSSELineReadsWrapperModelWithoutUnwrap(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{name: "顶层字段", payload: `{"modelVersion":"gemini-3-pro","response":{"candidates":[]}}`, want: "gemini-3-pro"},
		{name: "单层包装", payload: `{"response":{"modelVersion":"gemini-3-pro","candidates":[]}}`, want: "gemini-3-pro"},
		{name: "双层包装", payload: `{"response":{"response":{"modelVersion":"gemini-3-pro","candidates":[]}}}`, want: "gemini-3-pro"},
		{name: "外层优先", payload: `{"modelVersion":"gemini-outer","response":{"modelVersion":"gemini-inner","candidates":[]}}`, want: "gemini-outer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(nil)
			beginUpstreamResponseModelObservation(c)

			svc := &AntigravityGatewayService{}
			svc.observeAntigravityGeminiSSELine(c, "data: "+tt.payload)

			require.Equal(t, tt.want, observedUpstreamResponseModel(c))
			require.False(t, observedUpstreamResponseModelConflict(c))
		})
	}
}

func TestUpstreamResponseModelObserverTerminalConflictAndMalformedJSON(t *testing.T) {
	observer := &upstreamResponseModelObserver{}
	observer.ObserveOpenAI([]byte(`{"response":{"model":"gpt-5.4"}`), "response.completed")
	require.Empty(t, observer.Model())

	observer.Observe("gpt-5.5", false)
	observer.Observe("gpt-5.4", true)
	require.Equal(t, "gpt-5.4", observer.Model())
	require.True(t, observer.Conflict())
}

func TestUpstreamModelMismatchAndBound(t *testing.T) {
	require.Nil(t, upstreamModelMismatch("gpt-5", ""))
	matched := upstreamModelMismatch("gpt-5", "GPT-5")
	require.NotNil(t, matched)
	require.False(t, *matched)
	observer := &upstreamResponseModelObserver{}
	observer.Observe(strings.Repeat("模", upstreamResponseModelMaxLength+1), false)
	require.Len(t, []rune(observer.Model()), upstreamResponseModelMaxLength)
}
