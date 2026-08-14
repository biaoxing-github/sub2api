package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCodexFingerprintAccount 构造只包含指纹测试所需字段的 OpenAI OAuth 账号。
func newTestCodexFingerprintAccount(id int64, extra map[string]any) *Account {
	return &Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}
}

// TestGetCodexFingerprintMode 验证默认值、显式关闭和账号类型边界。
func TestGetCodexFingerprintMode(t *testing.T) {
	tests := []struct {
		name     string
		account  *Account
		expected codexFingerprintMode
	}{
		{name: "nil账号", expected: codexFingerprintOff},
		{name: "非OAuth账号", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, expected: codexFingerprintOff},
		{name: "未配置默认session", account: newTestCodexFingerprintAccount(1, nil), expected: codexFingerprintSession},
		{name: "非法值默认session", account: newTestCodexFingerprintAccount(1, map[string]any{codexFingerprintModeExtraKey: "invalid"}), expected: codexFingerprintSession},
		{name: "显式off", account: newTestCodexFingerprintAccount(1, map[string]any{codexFingerprintModeExtraKey: "off"}), expected: codexFingerprintOff},
		{name: "device", account: newTestCodexFingerprintAccount(1, map[string]any{codexFingerprintModeExtraKey: "device"}), expected: codexFingerprintDevice},
		{name: "full", account: newTestCodexFingerprintAccount(1, map[string]any{codexFingerprintModeExtraKey: "full"}), expected: codexFingerprintFull},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.account.GetCodexFingerprintMode())
		})
	}
}

// TestResolveConvergedInstallationID 验证真实设备 ID 优先和账号级确定性派生。
func TestResolveConvergedInstallationID(t *testing.T) {
	configured := newTestCodexFingerprintAccount(1, map[string]any{"openai_device_id": "real-device-id"})
	assert.Equal(t, "real-device-id", resolveConvergedInstallationID(configured))

	derived := resolveConvergedInstallationID(newTestCodexFingerprintAccount(42, nil))
	_, err := uuid.Parse(derived)
	require.NoError(t, err)
	assert.Equal(t, derived, resolveConvergedInstallationID(newTestCodexFingerprintAccount(42, nil)))
	assert.NotEqual(t, derived, resolveConvergedInstallationID(newTestCodexFingerprintAccount(43, nil)))
}

// TestResolveCodexFingerprintIDsFromRequest 验证 off 边界及 session 模式的客户端线程隔离。
func TestResolveCodexFingerprintIDsFromRequest(t *testing.T) {
	off := newTestCodexFingerprintAccount(1, map[string]any{codexFingerprintModeExtraKey: "off"})
	assert.Nil(t, resolveCodexFingerprintIDsFromRequest(off, nil))

	account := newTestCodexFingerprintAccount(1, nil)
	headersA := http.Header{"Session-Id": []string{"client-a"}}
	headersB := http.Header{"Session-Id": []string{"client-b"}}
	idsA := resolveCodexFingerprintIDsFromRequest(account, headersA)
	idsB := resolveCodexFingerprintIDsFromRequest(account, headersB)
	require.NotNil(t, idsA)
	require.NotNil(t, idsB)
	assert.Equal(t, idsA.sessionID, idsB.sessionID)
	assert.NotEqual(t, idsA.threadID, idsB.threadID)
	assert.NotEqual(t, idsA.turnID, idsB.turnID)
}

// TestApplyCodexFingerprintHeadersDevice 验证 device 模式只改写设备字段并保留其他元数据。
func TestApplyCodexFingerprintHeadersDevice(t *testing.T) {
	account := newTestCodexFingerprintAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "device",
		"openai_device_id":           "converged-device",
	})
	headers := http.Header{}
	headers.Set("x-codex-window-id", "original-window")
	headers.Set("x-codex-turn-metadata", `{"installation_id":"old","session_id":"original-session","sandbox":"seccomp"}`)

	applyCodexFingerprintHeaders(headers, resolveCodexFingerprintIDsFromRequest(account, nil))

	assert.Equal(t, "converged-device", headers.Get("x-codex-installation-id"))
	assert.Equal(t, "original-window", headers.Get("x-codex-window-id"))
	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(headers.Get("x-codex-turn-metadata")), &metadata))
	assert.Equal(t, "converged-device", metadata["installation_id"])
	assert.Equal(t, "original-session", metadata["session_id"])
	assert.Equal(t, "seccomp", metadata["sandbox"])
}

// TestApplyCodexFingerprintHeadersSession 验证 session 模式的头字段收敛及非指纹字段保留。
func TestApplyCodexFingerprintHeadersSession(t *testing.T) {
	account := newTestCodexFingerprintAccount(7, nil)
	clientHeaders := http.Header{"Session-Id": []string{"client-session"}}
	ids := resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
	headers := http.Header{}
	headers.Set("x-codex-turn-metadata", `{"installation_id":"old","session_id":"old","thread_id":"old","turn_id":"old","window_id":"old:0","sandbox":"seccomp"}`)

	applyCodexFingerprintHeaders(headers, ids)

	assert.Equal(t, ids.installationID, headers.Get("x-codex-installation-id"))
	assert.Equal(t, ids.sessionID, headers.Get("session-id"))
	assert.Equal(t, ids.sessionID, headers.Get("session_id"))
	assert.Equal(t, ids.threadID, headers.Get("thread-id"))
	assert.Equal(t, ids.threadID, headers.Get("x-client-request-id"))
	assert.Equal(t, ids.windowID, headers.Get("x-codex-window-id"))
	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(headers.Get("x-codex-turn-metadata")), &metadata))
	assert.Equal(t, ids.turnID, metadata["turn_id"])
	assert.Equal(t, "seccomp", metadata["sandbox"])
}

// TestApplyCodexFingerprintHeadersFull 验证 full 模式不再按客户端会话拆分线程。
func TestApplyCodexFingerprintHeadersFull(t *testing.T) {
	account := newTestCodexFingerprintAccount(9, map[string]any{codexFingerprintModeExtraKey: "full"})
	idsA := resolveCodexFingerprintIDsFromRequest(account, http.Header{"Session-Id": []string{"client-a"}})
	idsB := resolveCodexFingerprintIDsFromRequest(account, http.Header{"Session-Id": []string{"client-b"}})
	assert.Equal(t, idsA.sessionID, idsA.threadID)
	assert.Equal(t, idsA.threadID, idsB.threadID)
	assert.Equal(t, idsA.windowID, idsB.windowID)
}

// TestApplyCodexFingerprintClientMetadata 验证请求体与请求头复用同一份 turn_id。
func TestApplyCodexFingerprintClientMetadata(t *testing.T) {
	account := newTestCodexFingerprintAccount(11, nil)
	ids := resolveCodexFingerprintIDsFromRequest(account, http.Header{"Session-Id": []string{"client-session"}})
	headers := http.Header{}
	headers.Set("x-codex-turn-metadata", `{"installation_id":"old","session_id":"old","thread_id":"old","turn_id":"old","window_id":"old:0"}`)
	body := map[string]any{
		"client_metadata": map[string]any{
			"x-codex-turn-metadata": `{"installation_id":"old","session_id":"old","thread_id":"old","turn_id":"old","window_id":"old:0","thread_source":"client"}`,
		},
	}

	applyCodexFingerprintHeaders(headers, ids)
	require.True(t, applyCodexFingerprintClientMetadata(body, ids))

	clientMetadata := body["client_metadata"].(map[string]any)
	assert.Equal(t, ids.installationID, clientMetadata["x-codex-installation-id"])
	assert.Equal(t, ids.sessionID, clientMetadata["session_id"])
	assert.Equal(t, ids.threadID, clientMetadata["thread_id"])
	assert.Equal(t, ids.turnID, clientMetadata["turn_id"])
	var headerMetadata map[string]any
	var bodyMetadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(headers.Get("x-codex-turn-metadata")), &headerMetadata))
	require.NoError(t, json.Unmarshal([]byte(clientMetadata["x-codex-turn-metadata"].(string)), &bodyMetadata))
	assert.Equal(t, ids.turnID, headerMetadata["turn_id"])
	assert.Equal(t, ids.turnID, bodyMetadata["turn_id"])
	assert.Equal(t, "client", bodyMetadata["thread_source"])
}

// TestExtractCodexFingerprintClientSessionID 验证标准连字符头优先并兼容下划线形式。
func TestExtractCodexFingerprintClientSessionID(t *testing.T) {
	headers := http.Header{}
	headers.Set("session-id", "hyphen")
	headers.Set("session_id", "underscore")
	assert.Equal(t, "hyphen", extractClientSessionID(headers))
	headers.Del("session-id")
	assert.Equal(t, "underscore", extractClientSessionID(headers))
}
