package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCodexFingerprintAccount 构造只包含指纹测试所需字段的 OpenAI OAuth 账号。
func newTestCodexFingerprintAccount(id int64, extra map[string]any) *Account {
	return &Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}
}

func newTestOAuthAccount(id int64, extra map[string]any) *Account {
	return newTestCodexFingerprintAccount(id, extra)
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
		{name: "未配置默认off", account: newTestCodexFingerprintAccount(1, nil), expected: codexFingerprintOff},
		{name: "非法值默认off", account: newTestCodexFingerprintAccount(1, map[string]any{codexFingerprintModeExtraKey: "invalid"}), expected: codexFingerprintOff},
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

	account := newTestCodexFingerprintAccount(1, map[string]any{codexFingerprintModeExtraKey: "session"})
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
	account := newTestCodexFingerprintAccount(7, map[string]any{codexFingerprintModeExtraKey: "session"})
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
	account := newTestCodexFingerprintAccount(11, map[string]any{codexFingerprintModeExtraKey: "session"})
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

// --- 透传路径：raw 字节版 client_metadata 改写 ---

// rawVsMapClientMetadata 用同一份 ids 分别跑 map 版与 raw 字节版，
// 返回两侧最终的 client_metadata 解码结果。
func rawVsMapClientMetadata(t *testing.T, body []byte, ids *codexFingerprintIDs) (map[string]any, map[string]any) {
	t.Helper()

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	applyCodexFingerprintClientMetadata(decoded, ids)
	mapCM, _ := decoded["client_metadata"].(map[string]any)

	rawBody, changed, err := applyCodexFingerprintClientMetadataRaw(body, ids)
	require.NoError(t, err)
	require.True(t, changed)
	var rawDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawBody, &rawDecoded))
	rawCM, _ := rawDecoded["client_metadata"].(map[string]any)
	return mapCM, rawCM
}

func TestApplyCodexFingerprintClientMetadataRaw_MatchesMapVariant(t *testing.T) {
	embedded := `{\"installation_id\":\"real-install\",\"session_id\":\"real-session\",\"sandbox\":\"seatbelt\"}`
	bodies := map[string]string{
		"no_client_metadata": `{"model":"gpt-5.6-sol","input":[],"stream":true}`,
		"object_with_extras": `{"model":"gpt-5.6-sol","client_metadata":{"session_id":"client-session","traceparent":"00-abc-def-01","x-codex-turn-metadata":"` + embedded + `"},"stream":true}`,
		"non_object_value":   `{"model":"gpt-5.6-sol","client_metadata":"bogus","stream":true}`,
	}
	for _, mode := range []codexFingerprintMode{codexFingerprintDevice, codexFingerprintSession, codexFingerprintFull} {
		account := newTestOAuthAccount(4242, nil)
		ids := resolveCodexFingerprintIDs(account, "client-sess-raw", mode)
		require.NotNil(t, ids)
		for name, body := range bodies {
			t.Run(string(mode)+"/"+name, func(t *testing.T) {
				mapCM, rawCM := rawVsMapClientMetadata(t, []byte(body), ids)
				assert.Equal(t, mapCM, rawCM, "raw 字节版与 map 版的 client_metadata 结果必须逐点一致")
			})
		}
	}
}

func TestApplyCodexFingerprintClientMetadataRaw_PreservesUnrelatedFields(t *testing.T) {
	account := newTestOAuthAccount(4243, nil)
	ids := resolveCodexFingerprintIDs(account, "client-sess-preserve", codexFingerprintSession)
	require.NotNil(t, ids)

	body := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"message","role":"user","content":"hi"}],"stream":true,"prompt_cache_key":"pck-1"}`)
	out, changed, err := applyCodexFingerprintClientMetadataRaw(body, ids)
	require.NoError(t, err)
	require.True(t, changed)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out, &decoded))
	assert.Equal(t, "gpt-5.6-sol", decoded["model"])
	assert.Equal(t, "pck-1", decoded["prompt_cache_key"])
	assert.Equal(t, true, decoded["stream"])
	cm, _ := decoded["client_metadata"].(map[string]any)
	require.NotNil(t, cm)
	assert.Equal(t, ids.sessionID, cm["session_id"])
	assert.Equal(t, ids.turnID, cm["turn_id"])
}

func TestApplyCodexFingerprintClientMetadataRaw_Noop(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol"}`)
	out, changed, err := applyCodexFingerprintClientMetadataRaw(body, nil)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, body, out)

	out, changed, err = applyCodexFingerprintClientMetadataRaw(nil, &codexFingerprintIDs{mode: codexFingerprintSession, installationID: "x"})
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Nil(t, out)
}

// --- context 暂存与出站头应用（透传/非透传共用 seam）---

func newFingerprintStageTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c
}

func TestStageCodexFingerprintIDs_NilOverwritesPreviousAccount(t *testing.T) {
	c := newFingerprintStageTestContext(t)
	accountA := newTestOAuthAccount(1001, nil)
	idsA := resolveCodexFingerprintIDs(accountA, "sess-x", codexFingerprintSession)
	require.NotNil(t, idsA)
	stageCodexFingerprintIDs(c, idsA)

	// failover 切到 off 模式账号：无条件覆写为 nil，上一账号 IDs 不得残留
	stageCodexFingerprintIDs(c, nil)

	h := http.Header{}
	h.Set("session_id", "isolated-session")
	accountB := newTestOAuthAccount(1002, map[string]any{"codex_fingerprint_mode": "off"})
	applyStagedCodexFingerprintHeaders(c, accountB, h)
	assert.Equal(t, "isolated-session", h.Get("session_id"), "off 账号不得应用上一账号的收敛 ID")
	assert.Empty(t, h.Get("x-codex-installation-id"))
}

func TestApplyStagedCodexFingerprintHeaders_SkipsNonOAuthAccount(t *testing.T) {
	c := newFingerprintStageTestContext(t)
	oauthIDs := resolveCodexFingerprintIDs(newTestOAuthAccount(1003, nil), "sess-y", codexFingerprintSession)
	require.NotNil(t, oauthIDs)
	stageCodexFingerprintIDs(c, oauthIDs)

	h := http.Header{}
	apiKeyAccount := &Account{ID: 1004, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	applyStagedCodexFingerprintHeaders(c, apiKeyAccount, h)
	assert.Empty(t, h.Get("x-codex-installation-id"), "stale 收敛 ID 不得应用到非 OAuth 账号")
}

func TestBuildUpstreamRequestOpenAIPassthrough_AppliesStagedFingerprint(t *testing.T) {
	svc := &OpenAIGatewayService{}
	// 收敛是显式 opt-in（#5610）：显式开启后验证透传路径的出站头收敛。
	account := newTestOAuthAccount(2001, map[string]any{
		"openai_oauth_passthrough": true,
		"codex_fingerprint_mode":   "session",
	})

	c := newFingerprintStageTestContext(t)
	c.Request.Header.Set("session_id", "real-client-session")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color")
	c.Request.Header.Set("originator", "codex_cli_rs")
	c.Request.Header.Set("x-codex-turn-metadata", `{"installation_id":"real-install","session_id":"real-session","sandbox":"seatbelt"}`)

	// 复刻 forwardOpenAIPassthrough 的解析+暂存 seam（默认 session 模式）
	ids := resolveCodexFingerprintIDsFromRequest(account, c.Request.Header)
	require.NotNil(t, ids)
	stageCodexFingerprintIDs(c, ids)

	body := []byte(`{"model":"gpt-5.6-sol","input":[],"stream":true}`)
	req, err := svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "test-token")
	require.NoError(t, err)

	assert.Equal(t, ids.sessionID, req.Header.Get("session_id"), "session 模式下出站 session_id 应为账号级收敛值")
	assert.Equal(t, ids.installationID, req.Header.Get("x-codex-installation-id"))
	assert.Equal(t, ids.windowID, req.Header.Get("x-codex-window-id"))
	assert.Equal(t, ids.threadID, req.Header.Get("x-client-request-id"))
	turnMetadata := req.Header.Get("x-codex-turn-metadata")
	require.NotEmpty(t, turnMetadata)
	assert.Contains(t, turnMetadata, ids.sessionID, "turn-metadata JSON 中的 session_id 应被收敛")
	assert.Contains(t, turnMetadata, `"sandbox":"seatbelt"`, "turn-metadata 未指定字段应原样保留")
}

func TestBuildUpstreamRequestOpenAIPassthrough_OffModeKeepsIsolatedSession(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := newTestOAuthAccount(2002, map[string]any{
		"openai_oauth_passthrough": true,
		"codex_fingerprint_mode":   "off",
	})

	c := newFingerprintStageTestContext(t)
	c.Request.Header.Set("session_id", "real-client-session")
	c.Request.Header.Set("originator", "codex_cli_rs")

	ids := resolveCodexFingerprintIDsFromRequest(account, c.Request.Header)
	require.Nil(t, ids)
	stageCodexFingerprintIDs(c, ids)

	body := []byte(`{"model":"gpt-5.6-sol","input":[],"stream":true}`)
	req, err := svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "test-token")
	require.NoError(t, err)

	assert.NotEmpty(t, req.Header.Get("session_id"))
	assert.NotEqual(t, resolveConvergedSessionID(account), req.Header.Get("session_id"), "off 模式不得收敛 session_id")
	assert.Empty(t, req.Header.Get("x-codex-window-id"))
}

func TestApplyCodexFingerprintClientMetadataRaw_NonObjectBodyUntouched(t *testing.T) {
	account := newTestOAuthAccount(4244, nil)
	ids := resolveCodexFingerprintIDs(account, "client-sess-nonobj", codexFingerprintSession)
	require.NotNil(t, ids)

	for _, body := range []string{`[1,2,3]`, `"plain string"`, `not json at all`} {
		out, changed, err := applyCodexFingerprintClientMetadataRaw([]byte(body), ids)
		require.NoError(t, err)
		assert.False(t, changed, "非 JSON 对象 body 不应被改写: %s", body)
		assert.Equal(t, []byte(body), out)
	}
}
