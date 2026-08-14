package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGrokChatResponsesBridgeEligibilityCompat 验证只有语义可无损转换的请求进入 Responses bridge。
func TestGrokChatResponsesBridgeEligibilityCompat(t *testing.T) {
	eligible, reason := grokChatResponsesBridgeEligibility([]byte(`{"model":"grok","messages":[{"role":"user","content":"hello"}],"stream":true,"stream_options":{"include_usage":true}}`))
	require.True(t, eligible)
	require.Empty(t, reason)

	eligible, reason = grokChatResponsesBridgeEligibility([]byte(`{"model":"grok","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"lookup"}}]}`))
	require.False(t, eligible)
	require.Equal(t, "unsupported_tools", reason)
}

// TestGrokChatResponsesRuntimeEligibleCompat 验证 bridge 仅用于已建立租户隔离缓存身份的 4.5/4.6。
func TestGrokChatResponsesRuntimeEligibleCompat(t *testing.T) {
	require.True(t, grokChatResponsesRuntimeEligible("grok-4.5", "cache-id"))
	require.True(t, grokChatResponsesRuntimeEligible("grok-4.6", "cache-id"))
	require.True(t, grokChatResponsesRuntimeEligible("grok-4.6-latest", "cache-id"))
	require.False(t, grokChatResponsesRuntimeEligible("grok-4.3", "cache-id"))
	require.False(t, grokChatResponsesRuntimeEligible("grok-4.5", ""))
}
