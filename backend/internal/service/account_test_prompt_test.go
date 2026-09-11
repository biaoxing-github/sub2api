package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAccountTestPayloadUsesCustomPrompt 验证账号测试会把管理员输入的问题原样发送给上游。
func TestAccountTestPayloadUsesCustomPrompt(t *testing.T) {
	const prompt = "请只回复 ACCOUNT_TEST_OK"

	claudePayload, err := createTestPayload("claude-test", "", prompt)
	require.NoError(t, err)
	claudeMessages := claudePayload["messages"].([]map[string]any)
	claudeContent := claudeMessages[0]["content"].([]map[string]any)
	require.Equal(t, prompt, claudeContent[0]["text"])

	openAIPayload := createOpenAITestPayload("gpt-test", &Account{}, prompt)
	openAIInput := openAIPayload["input"].([]map[string]any)
	openAIContent := openAIInput[0]["content"].([]map[string]any)
	require.Equal(t, prompt, openAIContent[0]["text"])
}

// TestAccountTestPayloadCreatesDefaultPrompt 验证未输入问题时仍生成系统默认测试问题。
func TestAccountTestPayloadCreatesDefaultPrompt(t *testing.T) {
	payload, err := createTestPayload("claude-test", "")
	require.NoError(t, err)
	messages := payload["messages"].([]map[string]any)
	content := messages[0]["content"].([]map[string]any)
	require.NotEmpty(t, content[0]["text"])
}
