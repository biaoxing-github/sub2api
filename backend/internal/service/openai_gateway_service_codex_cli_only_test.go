package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubCodexRestrictionDetector struct {
	result CodexClientRestrictionDetectionResult
}

func (s *stubCodexRestrictionDetector) Detect(_ *gin.Context, _ *Account, _ []string) CodexClientRestrictionDetectionResult {
	return s.result
}

func TestOpenAIGatewayService_GetCodexClientRestrictionDetector(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("使用注入的 detector", func(t *testing.T) {
		expected := &stubCodexRestrictionDetector{
			result: CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: "stub"},
		}
		svc := &OpenAIGatewayService{codexDetector: expected}

		got := svc.getCodexClientRestrictionDetector()
		require.Same(t, expected, got)
	})

	t.Run("service 为 nil 时返回默认 detector", func(t *testing.T) {
		var svc *OpenAIGatewayService
		got := svc.getCodexClientRestrictionDetector()
		require.NotNil(t, got)
	})

	t.Run("service 未注入 detector 时返回默认 detector", func(t *testing.T) {
		svc := &OpenAIGatewayService{cfg: &config.Config{}}
		got := svc.getCodexClientRestrictionDetector()
		require.NotNil(t, got)

		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "curl/8.0")
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_cli_only": true}}

		result := got.Detect(c, account, nil)
		require.True(t, result.Enabled)
		require.False(t, result.Matched)
		require.Equal(t, CodexClientRestrictionReasonNotMatchedUA, result.Reason)
	})
}

func TestOpenAIGatewayService_Forward_VersionGateMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtx := func() (*httptest.ResponseRecorder, *gin.Context) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
		return rec, c
	}
	account := func() *Account {
		return &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_cli_only": true}}
	}
	body := []byte(`{"model":"gpt-5.4"}`)

	t.Run("版本太低：返回带版本号的差异化文案", func(t *testing.T) {
		rec, c := newCtx()
		svc := &OpenAIGatewayService{codexDetector: &stubCodexRestrictionDetector{result: CodexClientRestrictionDetectionResult{
			Enabled:         true,
			Matched:         false,
			Reason:          CodexClientRestrictionReasonVersionTooLow,
			DetectedVersion: "0.39.0",
			MinCodexVersion: "0.42.0",
		}}}

		_, err := svc.Forward(context.Background(), c, account(), body)
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, rec.Body.String(), "Your Codex version (0.39.0) is below the minimum required version (0.42.0)")
		require.NotContains(t, rec.Body.String(), CodexOfficialClientsOnlyMessage)
	})

	t.Run("未命中官方：仍返回通用兜底文案", func(t *testing.T) {
		rec, c := newCtx()
		svc := &OpenAIGatewayService{codexDetector: &stubCodexRestrictionDetector{result: CodexClientRestrictionDetectionResult{
			Enabled: true,
			Matched: false,
			Reason:  CodexClientRestrictionReasonNotMatchedUA,
		}}}

		_, err := svc.Forward(context.Background(), c, account(), body)
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, rec.Body.String(), CodexOfficialClientsOnlyMessage)
	})
}

// 回归 #3887：compact 首轮已用心跳提交 SSE 后，下一轮切号命中的本地限制
// 不能把 JSON 错误追加到既有 SSE 流，必须以 response.failed 结束协议。
func TestOpenAIGatewayService_Forward_LocalRestrictionAfterCompactRetryEmitsSSEFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	MarkOpenAICompactClientStream(c)

	stopFirstAttempt := StartOpenAICompactSSEKeepalive(c, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	stopFirstAttempt()
	stopSecondAttempt := StartOpenAICompactSSEKeepalive(c, time.Hour)
	defer stopSecondAttempt()

	svc := &OpenAIGatewayService{codexDetector: &stubCodexRestrictionDetector{result: CodexClientRestrictionDetectionResult{
		Enabled: true,
		Matched: false,
		Reason:  CodexClientRestrictionReasonNotMatchedUA,
	}}}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_cli_only": true}}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`))
	require.Error(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "event: response.failed\n")
	require.Contains(t, rec.Body.String(), `"code":"forbidden_error"`)
	require.NotContains(t, rec.Body.String(), "\n\n{\"error\":")
}

func TestOpenAIGatewayService_ForwardAsChatCompletions_RejectsCodexCLIOnlyNonOfficialClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":11,"output_tokens":5,"total_tokens":16}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
		codexDetector: &stubCodexRestrictionDetector{
			result: CodexClientRestrictionDetectionResult{
				Enabled: true,
				Matched: false,
				Reason:  CodexClientRestrictionReasonNotMatchedUA,
			},
		},
	}
	account := &Account{
		ID:          1001,
		Name:        "openai-oauth-codex-only",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Extra: map[string]any{"codex_cli_only": true},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "codex_cli_only restriction")
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "This account only allows Codex official clients")
	require.Empty(t, upstream.requests)
}

func TestOpenAIGatewayService_ForwardAsChatCompletions_AllowsAPIKeyRawChatWhenCodexCLIOnlyExtraIsPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_raw_apikey"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_api_key","object":"chat.completion","model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1002,
		Name:        "openai-apikey-raw-chat",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
		},
		Extra: map[string]any{
			"codex_cli_only":             true,
			"openai_responses_supported": false,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"content":"ok"`)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-test", upstream.requests[0].Header.Get("Authorization"))
}

func TestOpenAICodexCLISimulationUsesLatestClientVersion(t *testing.T) {
	require.Equal(t, "0.144.1", codexCLIVersion())
	require.Equal(t, "Codex Desktop/0.144.1 (Windows 10.0.26200; x86_64) unknown (Codex Desktop; 26.616.32156)", codexCLIUserAgent())
	require.NotContains(t, codexCLIUserAgent(), "0.125.0")
	require.Contains(t, defaultOpenAICodexUserAgent(), "0.144.1")
	require.NotContains(t, defaultOpenAICodexUserAgent(), "0.125.0")
	require.Equal(t, "codex_cli_rs", codexCLIOriginator)
	require.Equal(t, "compact-history", codexCLIBetaFeatures)
}

func TestApplyOpenAICodexLatestClientHeadersMatchesCapturedClientShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(nil))
	req.Header.Set("User-Agent", "codex_cli_rs/0.125.0")
	req.Header.Set("originator", "codex_cli_rs")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("version", "0.125.0")
	req.Header.Set("thread-id", "thread-from-client")
	body := []byte(`{"prompt_cache_key":"prompt-cache-from-body"}`)
	account := &Account{
		ID:          470,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-sharedchat", "base_url": "https://new.sharedchat.cc/codex"},
	}

	applyOpenAICodexLatestClientHeaders(req, body, account)
	keyFingerprint := codexSimulationInstallationFingerprint(account)
	expectedSessionID := namespaceCodexSimulationIdentifier(keyFingerprint, "session", "prompt-cache-from-body")
	expectedThreadID := namespaceCodexSimulationIdentifier(keyFingerprint, "thread", "thread-from-client")

	require.Equal(t, codexCLIUserAgent(), req.Header.Get("User-Agent"))
	require.Equal(t, codexCLIOriginator, req.Header.Get("originator"))
	require.Equal(t, "responses=experimental", req.Header.Get("OpenAI-Beta"))
	require.Equal(t, codexCLIVersion(), req.Header.Get("version"))
	require.Equal(t, "text/event-stream", req.Header.Get("Accept"))
	require.Equal(t, codexCLIBetaFeatures, req.Header.Get("X-Codex-Beta-Features"))
	require.Equal(t, expectedSessionID, req.Header.Get("Session-Id"))
	require.Equal(t, expectedThreadID, req.Header.Get("Thread-Id"))
	require.NotEmpty(t, req.Header.Get("X-Client-Request-Id"))
	require.NotEqual(t, expectedSessionID, req.Header.Get("X-Client-Request-Id"))
	require.Equal(t, expectedSessionID+":0", req.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, resolveCodexSimulationInstallationID(account), req.Header.Get("X-Codex-Installation-Id"))

	var turnMetadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(req.Header.Get("X-Codex-Turn-Metadata")), &turnMetadata))
	require.Equal(t, expectedSessionID, turnMetadata["session_id"])
	require.Equal(t, expectedThreadID, turnMetadata["thread_id"])
	require.Equal(t, expectedSessionID+":0", turnMetadata["window_id"])
	require.Equal(t, resolveCodexSimulationInstallationID(account), turnMetadata["installation_id"])
	require.Equal(t, "turn", turnMetadata["request_kind"])
	require.NotEmpty(t, turnMetadata["turn_id"])
	require.NotZero(t, turnMetadata["turn_started_at_unix_ms"])
}

func TestGetAPIKeyIDFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("context 为 nil", func(t *testing.T) {
		require.Equal(t, int64(0), getAPIKeyIDFromContext(nil))
	})

	t.Run("上下文没有 api_key", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		require.Equal(t, int64(0), getAPIKeyIDFromContext(c))
	})

	t.Run("api_key 类型错误", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("api_key", "not-api-key")
		require.Equal(t, int64(0), getAPIKeyIDFromContext(c))
	})

	t.Run("api_key 指针为空", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		var k *APIKey
		c.Set("api_key", k)
		require.Equal(t, int64(0), getAPIKeyIDFromContext(c))
	})

	t.Run("正常读取 api_key_id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("api_key", &APIKey{ID: 12345})
		require.Equal(t, int64(12345), getAPIKeyIDFromContext(c))
	})
}

func TestLogCodexCLIOnlyDetection_NilSafety(t *testing.T) {
	// 不校验日志内容，仅保证在 nil 入参下不会 panic。
	require.NotPanics(t, func() {
		logCodexCLIOnlyDetection(context.TODO(), nil, nil, 0, CodexClientRestrictionDetectionResult{Enabled: true, Matched: false, Reason: "test"}, nil)
		logCodexCLIOnlyDetection(context.Background(), nil, nil, 0, CodexClientRestrictionDetectionResult{Enabled: false, Matched: false, Reason: "disabled"}, nil)
	})
}

func TestLogCodexCLIOnlyDetection_OnlyLogsRejected(t *testing.T) {
	logSink, restore := captureStructuredLog(t)
	defer restore()

	account := &Account{ID: 1001}
	logCodexCLIOnlyDetection(context.Background(), nil, account, 2002, CodexClientRestrictionDetectionResult{
		Enabled: true,
		Matched: true,
		Reason:  CodexClientRestrictionReasonMatchedUA,
	}, nil)
	logCodexCLIOnlyDetection(context.Background(), nil, account, 2002, CodexClientRestrictionDetectionResult{
		Enabled: true,
		Matched: false,
		Reason:  CodexClientRestrictionReasonNotMatchedUA,
	}, nil)

	require.False(t, logSink.ContainsMessage("OpenAI codex_cli_only 允许官方客户端请求"))
	require.True(t, logSink.ContainsMessage("OpenAI codex_cli_only 拒绝非官方客户端请求"))
}

func TestLogCodexCLIOnlyDetection_RejectedIncludesRequestDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", bytes.NewReader(nil))
	c.Request.RemoteAddr = "172.18.0.1:54321"
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0 (Windows 10.0.19045; x86_64) unknown")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Originator", "codex_cli_rs")
	c.Request.Header.Set("Version", "0.98.0")
	c.Request.Header.Set("X-OpenAI-Client-Source", "codex")
	c.Request.Header.Set("X-Codex-Beta-Features", "compact-history")
	c.Request.Header.Set("X-Client-Request-Id", "client-req-123")
	c.Request.Header.Set("Session-Id", "header-session-123")
	c.Request.Header.Set("Thread-Id", "header-thread-456")
	c.Request.Header.Set("X-Codex-Window-Id", "header-window-abc")
	c.Request.Header.Set("X-Codex-Installation-Id", "header-install-secret")
	c.Request.Header.Set("X-Codex-Turn-Metadata", `{"session_id":"header-turn-session-123","thread_id":"header-turn-thread-456","turn_id":"header-turn-id-789","window_id":"header-turn-window-abc","installation_id":"header-turn-install-secret","request_kind":"turn"}`)
	c.Request.Header.Set("X-Real-IP", "203.0.113.42")
	c.Request.Header.Set("OpenAI-Beta", "assistants=v2")

	body := []byte(`{"model":"gpt-5.2","stream":false,"prompt_cache_key":"pc-123","access_token":"secret-token","client_metadata":{"originator":"codex_cli_rs","session_id":"session-123","thread_id":"thread-456","turn_id":"turn-789","x-codex-window-id":"window-abc","x-codex-installation-id":"install-secret","x-openai-client-source":"codex","x-codex-turn-metadata":"{\"session_id\":\"turn-session-123\",\"thread_id\":\"turn-thread-456\",\"turn_id\":\"turn-id-789\",\"window_id\":\"turn-window-abc\",\"installation_id\":\"turn-install-secret\",\"request_kind\":\"turn\"}"},"input":[{"type":"text","text":"hello"}]}`)
	account := &Account{ID: 1001}
	logCodexCLIOnlyDetection(context.Background(), c, account, 2002, CodexClientRestrictionDetectionResult{
		Enabled: true,
		Matched: false,
		Reason:  CodexClientRestrictionReasonNotMatchedUA,
	}, body)

	require.True(t, logSink.ContainsFieldValue("request_user_agent", "codex_cli_rs/0.98.0 (Windows 10.0.19045; x86_64) unknown"))
	require.True(t, logSink.ContainsFieldValue("request_model", "gpt-5.2"))
	require.True(t, logSink.ContainsFieldValue("request_query", "trace=1"))
	require.True(t, logSink.ContainsFieldValue("request_client_ip", "203.0.113.42"))
	require.True(t, logSink.ContainsFieldValue("request_remote_addr", "172.18.0.1:54321"))
	require.True(t, logSink.ContainsFieldValue("codex_signal_family", "unmatched"))
	require.True(t, logSink.ContainsFieldValue("codex_fingerprint_profile", "metadata_and_headers"))
	require.True(t, logSink.ContainsFieldValue("request_prompt_cache_key_sha256", hashSensitiveValueForLog("pc-123")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_originator", "codex_cli_rs"))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_source", "codex"))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_session_id_sha256", hashSensitiveValueForLog("session-123")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_thread_id_sha256", hashSensitiveValueForLog("thread-456")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_turn_id_sha256", hashSensitiveValueForLog("turn-789")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_window_id_sha256", hashSensitiveValueForLog("window-abc")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_installation_id_sha256", hashSensitiveValueForLog("install-secret")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_turn_request_kind", "turn"))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_turn_session_id_sha256", hashSensitiveValueForLog("turn-session-123")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_turn_thread_id_sha256", hashSensitiveValueForLog("turn-thread-456")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_turn_turn_id_sha256", hashSensitiveValueForLog("turn-id-789")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_turn_window_id_sha256", hashSensitiveValueForLog("turn-window-abc")))
	require.True(t, logSink.ContainsFieldValue("request_client_metadata_turn_installation_id_sha256", hashSensitiveValueForLog("turn-install-secret")))
	require.True(t, logSink.ContainsFieldValue("request_header_originator", "codex_cli_rs"))
	require.True(t, logSink.ContainsFieldValue("request_header_version", "0.98.0"))
	require.True(t, logSink.ContainsFieldValue("request_header_x_openai_client_source", "codex"))
	require.True(t, logSink.ContainsFieldValue("request_header_x_codex_beta_features", "compact-history"))
	require.True(t, logSink.ContainsFieldValue("request_header_client_request_id_sha256", hashSensitiveValueForLog("client-req-123")))
	require.True(t, logSink.ContainsFieldValue("request_header_session_id_sha256", hashSensitiveValueForLog("header-session-123")))
	require.True(t, logSink.ContainsFieldValue("request_header_thread_id_sha256", hashSensitiveValueForLog("header-thread-456")))
	require.True(t, logSink.ContainsFieldValue("request_header_window_id_sha256", hashSensitiveValueForLog("header-window-abc")))
	require.True(t, logSink.ContainsFieldValue("request_header_installation_id_sha256", hashSensitiveValueForLog("header-install-secret")))
	require.True(t, logSink.ContainsFieldValue("request_header_turn_request_kind", "turn"))
	require.True(t, logSink.ContainsFieldValue("request_header_turn_session_id_sha256", hashSensitiveValueForLog("header-turn-session-123")))
	require.True(t, logSink.ContainsFieldValue("request_header_turn_thread_id_sha256", hashSensitiveValueForLog("header-turn-thread-456")))
	require.True(t, logSink.ContainsFieldValue("request_header_turn_turn_id_sha256", hashSensitiveValueForLog("header-turn-id-789")))
	require.True(t, logSink.ContainsFieldValue("request_header_turn_window_id_sha256", hashSensitiveValueForLog("header-turn-window-abc")))
	require.True(t, logSink.ContainsFieldValue("request_header_turn_installation_id_sha256", hashSensitiveValueForLog("header-turn-install-secret")))
	require.True(t, logSink.ContainsFieldValue("request_headers", "openai-beta"))
	require.True(t, logSink.ContainsField("request_body_size"))
	require.False(t, logSink.ContainsField("request_body_preview"))
}

func TestCodexCLIOnlyFingerprintProfile(t *testing.T) {
	header := http.Header{}
	header.Set("X-Codex-Turn-Metadata", `{"session_id":"s1","thread_id":"t1","turn_id":"turn1","window_id":"w1","installation_id":"i1","request_kind":"turn"}`)

	bodyWithMetadata := []byte(`{"client_metadata":{"session_id":"s1","thread_id":"t1","turn_id":"turn1","x-codex-turn-metadata":"{\"session_id\":\"s2\",\"request_kind\":\"turn\"}"}}`)

	require.Equal(t, "metadata_and_headers", codexCLIOnlyFingerprintProfile(header, bodyWithMetadata))
	require.Equal(t, "metadata_only", codexCLIOnlyFingerprintProfile(http.Header{}, bodyWithMetadata))
	require.Equal(t, "headers_only", codexCLIOnlyFingerprintProfile(header, []byte(`{"model":"gpt-5.5"}`)))
	require.Equal(t, "none", codexCLIOnlyFingerprintProfile(http.Header{}, []byte(`{"model":"gpt-5.5"}`)))
}

func TestLogOpenAIInstructionsRequiredDebug_LogsRequestDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("OpenAI-Beta", "assistants=v2")

	body := []byte(`{"model":"gpt-5.1-codex","stream":false,"prompt_cache_key":"pc-abc","access_token":"secret-token","input":[{"type":"text","text":"hello"}]}`)
	account := &Account{ID: 1001, Name: "codex max套餐"}

	logOpenAIInstructionsRequiredDebug(
		context.Background(),
		c,
		account,
		http.StatusBadRequest,
		"Instructions are required",
		body,
		[]byte(`{"error":{"message":"Instructions are required","type":"invalid_request_error","param":"instructions","code":"missing_required_parameter"}}`),
	)

	require.True(t, logSink.ContainsMessageAtLevel("OpenAI 上游返回 Instructions are required，已记录请求详情用于排查", "warn"))
	require.True(t, logSink.ContainsFieldValue("request_user_agent", "curl/8.0"))
	require.True(t, logSink.ContainsFieldValue("request_model", "gpt-5.1-codex"))
	require.True(t, logSink.ContainsFieldValue("request_query", "trace=1"))
	require.True(t, logSink.ContainsFieldValue("account_name", "codex max套餐"))
	require.True(t, logSink.ContainsFieldValue("request_headers", "openai-beta"))
	require.True(t, logSink.ContainsField("request_body_size"))
	require.False(t, logSink.ContainsField("request_body_preview"))
}

func TestLogOpenAIInstructionsRequiredDebug_NonTargetErrorSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	body := []byte(`{"model":"gpt-5.1-codex","stream":false}`)

	logOpenAIInstructionsRequiredDebug(
		context.Background(),
		c,
		&Account{ID: 1001},
		http.StatusForbidden,
		"forbidden",
		body,
		[]byte(`{"error":{"message":"forbidden"}}`),
	)

	require.False(t, logSink.ContainsMessage("OpenAI 上游返回 Instructions are required，已记录请求详情用于排查"))
}

func TestIsOpenAITransientProcessingError(t *testing.T) {
	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"An error occurred while processing your request.",
		nil,
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"Selected model is at capacity. Please try a different model.",
		[]byte(`{"error":{"message":"Selected model is at capacity. Please try a different model.","type":"invalid_request_error"}}`),
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"",
		[]byte(`{"error":{"message":"An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com if the error persists. Please include the request ID req_123 in your message."}}`),
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"There was an issue with the format or content of your request. (request id: 202606100732187175600488268d9d6syj42KHv)",
		[]byte(`{"error":{"type":"<nil>","message":"There was an issue with the format or content of your request. (request id: 202606100732187175600488268d9d6syj42KHv) (request id: 202606100732185428879478268d9d65Q8VA5iy)"}}`),
	))

	require.False(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"Missing required parameter: 'instructions'",
		[]byte(`{"error":{"message":"Missing required parameter: 'instructions'"}}`),
	))
}

func TestIsOpenAIContextWindowError(t *testing.T) {
	require.True(t, isOpenAIContextWindowError(
		"",
		[]byte(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"upstream_error","code":null}}`),
	))
	require.True(t, isOpenAIContextWindowError("maximum context length exceeded", nil))
	require.False(t, isOpenAIContextWindowError("context canceled", nil))
}

func TestShouldFailoverOpenAIUpstreamResponseUsesAllNon2xxStatuses(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"upstream_error","code":null}}`)

	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusBadGateway, "", body))
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusBadGateway, "temporary upstream outage", []byte(`{"error":{"message":"temporary upstream outage"}}`)))
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusRequestEntityTooLarge, "Request Entity Too Large", nil))
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusBadRequest, "invalid parameter", nil))
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusMultipleChoices, "redirected upstream", nil))
	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusOK, "upstream response failed", nil))
}

func TestOpenAIGatewayService_Forward_LogsInstructionsRequiredDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("OpenAI-Beta", "assistants=v2")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-upstream"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Missing required parameter: 'instructions'","type":"invalid_request_error","param":"instructions","code":"missing_required_parameter"}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             1001,
		Name:           "codex max套餐",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Concurrency:    1,
		Credentials:    map[string]any{"api_key": "sk-test"},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}
	body := []byte(`{"model":"gpt-5.1-codex","stream":false,"input":[{"type":"text","text":"hello"}],"prompt_cache_key":"pc-forward","access_token":"secret-token"}`)

	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, c.Writer.Written(), "切号前不得向客户端提交 400 响应")
	require.Contains(t, err.Error(), "upstream error: 400")

	require.True(t, logSink.ContainsMessageAtLevel("OpenAI 上游返回 Instructions are required，已记录请求详情用于排查", "warn"))
	require.True(t, logSink.ContainsFieldValue("request_user_agent", "codex_cli_rs/0.1.0"))
	require.True(t, logSink.ContainsFieldValue("request_model", "gpt-5.1-codex"))
	require.True(t, logSink.ContainsFieldValue("request_headers", "openai-beta"))
	require.True(t, logSink.ContainsField("request_body_size"))
	require.False(t, logSink.ContainsField("request_body_preview"))
}

func TestOpenAIGatewayService_Forward_TransientProcessingErrorTriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-processing-400"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com if the error persists. Please include the request ID req_123 in your message.","type":"invalid_request_error"}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             1001,
		Name:           "codex max套餐",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Concurrency:    1,
		Credentials:    map[string]any{"api_key": "sk-test"},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}
	body := []byte(`{"model":"gpt-5.1-codex","stream":false,"input":[{"type":"text","text":"hello"}]}`)

	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "An error occurred while processing your request")
	require.False(t, c.Writer.Written(), "service 层应返回 failover 错误给上层换号，而不是直接向客户端写响应")
}

func TestOpenAIGatewayService_ForwardAsAnthropic_FormatContentIssueTriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"claude-3-7-sonnet-20250219","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "claude-code/1.0.0")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-format-content-400"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"type":"<nil>","message":"There was an issue with the format or content of your request. (request id: 202606100732187175600488268d9d6syj42KHv) (request id: 202606100732185428879478268d9d65Q8VA5iy)"}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1001,
		Name:        "claude-code-openai-compat",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.1")
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "format or content")
	require.False(t, c.Writer.Written(), "Claude Code 兼容路径应把该类上游 400 交给 handler 换远端，不能先写给客户端")
}

func TestOpenAIGatewayService_Forward_ModelCapacityErrorTriggersFailoverAndSameAccountRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-capacity-400"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Selected model is at capacity. Please try a different model.","type":"invalid_request_error"}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1001,
		Name:        "codex max套餐",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":   "sk-test",
			"pool_mode": true,
		},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}
	body := []byte(`{"model":"gpt-5.4","stream":false,"input":[{"type":"text","text":"hello"}]}`)

	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Contains(t, string(failoverErr.ResponseBody), "Selected model is at capacity")
	require.False(t, c.Writer.Written(), "service 层应返回 failover 错误给上层重试/换号，而不是直接向客户端写响应")
}
