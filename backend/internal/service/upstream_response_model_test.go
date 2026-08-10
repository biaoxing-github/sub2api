package service

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpstreamResponseModelObserver(t *testing.T) {
	observer := &upstreamResponseModelObserver{}
	observer.ObserveOpenAI([]byte(`{"type":"response.created","response":{"model":"gpt-5.5"}}`), "response.created")
	observer.ObserveOpenAI([]byte(`{"type":"response.completed","response":{"model":"gpt-5.4"}}`), "response.completed")
	require.Equal(t, "gpt-5.4", observer.Model())
	require.True(t, observer.Conflict())
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
