package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasBillableOpenAIUsage(t *testing.T) {
	require.False(t, hasBillableOpenAIUsage(OpenAIUsage{}))
	require.True(t, hasBillableOpenAIUsage(OpenAIUsage{InputTokens: 1}))
	require.True(t, hasBillableOpenAIUsage(OpenAIUsage{ImageOutputTokens: 1}))
}

func TestRequiresBillableGrokChatUsage(t *testing.T) {
	require.True(t, requiresBillableGrokChatUsage(&Account{Platform: PlatformGrok}, "gpt-4o"))
	require.True(t, requiresBillableGrokChatUsage(&Account{Platform: PlatformOpenAI}, "grok-4.5"))
	require.False(t, requiresBillableGrokChatUsage(&Account{Platform: PlatformOpenAI}, "gpt-4o"))
}
