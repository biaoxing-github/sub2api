package service

import (
	"strings"
	"testing"

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

func TestUpstreamModelMismatchAndBound(t *testing.T) {
	require.Nil(t, upstreamModelMismatch("gpt-5", ""))
	matched := upstreamModelMismatch("gpt-5", "GPT-5")
	require.NotNil(t, matched)
	require.False(t, *matched)
	observer := &upstreamResponseModelObserver{}
	observer.Observe(strings.Repeat("模", upstreamResponseModelMaxLength+1), false)
	require.Len(t, []rune(observer.Model()), upstreamResponseModelMaxLength)
}
