package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const kiroReasoningCanonicalBody = `{"model":"gpt-5.1","stream":false,"input":[` +
	`{"type":"message","role":"user","content":"hello"},` +
	`{"type":"reasoning","id":"rs_kiro_abc123","encrypted_content":"ENC_BLOB","summary":[{"type":"summary_text","text":"thinking"}]},` +
	`{"type":"message","role":"assistant","content":"hi"}` +
	`]}`

func newOpenAIPassthroughAccount(id int64, passthrough bool) *service.Account {
	return &service.Account{
		ID:       id,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Extra:    map[string]any{"openai_passthrough": passthrough},
	}
}

func reasoningItemCount(t *testing.T, body []byte) int {
	t.Helper()
	count := 0
	gjson.GetBytes(body, "input").ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "reasoning" {
			count++
		}
		return true
	})
	return count
}

func TestDeriveOpenAIForwardAttemptBody_CrossModeStripsKiroReasoning(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	canonical := []byte(kiroReasoningCanonicalBody)
	state := &openAIPassthroughFailoverState{}

	firstBody := h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(1, true), state)
	require.Equal(t, 1, reasoningItemCount(t, firstBody))
	require.Equal(t, "ENC_BLOB", gjson.GetBytes(firstBody, "input.1.encrypted_content").String())

	secondBody := h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(2, false), state)
	require.Equal(t, 0, reasoningItemCount(t, secondBody))
	require.NotContains(t, string(secondBody), "rs_kiro_abc123")
	require.NotContains(t, string(secondBody), "ENC_BLOB")
	require.Equal(t, 2, int(gjson.GetBytes(secondBody, "input.#").Int()))
	require.JSONEq(t, kiroReasoningCanonicalBody, string(canonical))
}

func TestDeriveOpenAIForwardAttemptBody_SameModePreservesReasoning(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	canonical := []byte(kiroReasoningCanonicalBody)

	t.Run("non_passthrough_to_non_passthrough", func(t *testing.T) {
		state := &openAIPassthroughFailoverState{}
		_ = h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(10, false), state)
		second := h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(11, false), state)
		require.Equal(t, 1, reasoningItemCount(t, second))
	})

	t.Run("passthrough_to_passthrough", func(t *testing.T) {
		state := &openAIPassthroughFailoverState{}
		_ = h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(20, true), state)
		second := h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(21, true), state)
		require.Equal(t, 1, reasoningItemCount(t, second))
	})

	t.Run("non_passthrough_to_passthrough", func(t *testing.T) {
		state := &openAIPassthroughFailoverState{}
		_ = h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(30, false), state)
		second := h.deriveOpenAIForwardAttemptBody(nil, canonical, newOpenAIPassthroughAccount(31, true), state)
		require.Equal(t, 1, reasoningItemCount(t, second))
	})
}

func TestDeriveOpenAIForwardAttemptBody_SanitizationSticksAcrossRetries(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	canonical := []byte(kiroReasoningCanonicalBody)
	state := &openAIPassthroughFailoverState{}
	kiro := newOpenAIPassthroughAccount(40, true)
	bedrockA := newOpenAIPassthroughAccount(41, false)
	bedrockB := newOpenAIPassthroughAccount(42, false)

	require.Equal(t, 1, reasoningItemCount(t, h.deriveOpenAIForwardAttemptBody(nil, canonical, kiro, state)))
	require.Equal(t, 0, reasoningItemCount(t, h.deriveOpenAIForwardAttemptBody(nil, canonical, bedrockA, state)))
	require.Equal(t, 0, reasoningItemCount(t, h.deriveOpenAIForwardAttemptBody(nil, canonical, bedrockA, state)))
	require.Equal(t, 0, reasoningItemCount(t, h.deriveOpenAIForwardAttemptBody(nil, canonical, bedrockB, state)))
	require.JSONEq(t, kiroReasoningCanonicalBody, string(canonical))
}
