package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type manualProbeAccountRepo struct {
	service.AccountRepository
	account *service.Account
}

// manualProbeAdminService 用同一份账号状态模拟恢复后从数据库重新读取的结果。
type manualProbeAdminService struct {
	*stubAdminService
	account *service.Account
}

func (s *manualProbeAdminService) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	if s.account != nil && s.account.ID == id {
		account := *s.account
		return &account, nil
	}
	return s.stubAdminService.GetAccount(ctx, id)
}

func (r *manualProbeAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, service.ErrAccountNotFound
}

func (r *manualProbeAccountRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.account != nil {
		if r.account.Extra == nil {
			r.account.Extra = map[string]any{}
		}
		for key, value := range updates {
			r.account.Extra[key] = value
		}
	}
	return nil
}

func (r *manualProbeAccountRepo) UpdateCredentials(_ context.Context, _ int64, credentials map[string]any) error {
	if r.account != nil {
		r.account.Credentials = make(map[string]any, len(credentials))
		for key, value := range credentials {
			r.account.Credentials[key] = value
		}
	}
	return nil
}

func (r *manualProbeAccountRepo) ClearError(context.Context, int64) error {
	if r.account != nil {
		r.account.Status = service.StatusActive
		r.account.ErrorMessage = ""
	}
	return nil
}

func (r *manualProbeAccountRepo) SetError(context.Context, int64, string) error {
	return nil
}

func (r *manualProbeAccountRepo) ClearRateLimit(context.Context, int64) error {
	return nil
}

func (r *manualProbeAccountRepo) ClearTempUnschedulable(context.Context, int64) error {
	if r.account != nil {
		r.account.TempUnschedulableUntil = nil
		r.account.TempUnschedulableReason = ""
	}
	return nil
}

func (r *manualProbeAccountRepo) ClearAntigravityQuotaScopes(context.Context, int64) error {
	return nil
}

func (r *manualProbeAccountRepo) ClearModelRateLimits(context.Context, int64) error {
	return nil
}

func (r *manualProbeAccountRepo) SetSchedulable(_ context.Context, _ int64, schedulable bool) error {
	if r.account != nil {
		r.account.Schedulable = schedulable
	}
	return nil
}

type manualProbeHTTPUpstream struct {
	response      *http.Response
	requestBodies []string
}

func (u *manualProbeHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.captureBody(req)
	return u.response, nil
}

func (u *manualProbeHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.captureBody(req)
	return u.response, nil
}

func (u *manualProbeHTTPUpstream) captureBody(req *http.Request) {
	if req == nil || req.Body == nil {
		return
	}
	body, _ := io.ReadAll(req.Body)
	raw := string(body)
	u.requestBodies = append(u.requestBodies, raw)
	req.Body = io.NopCloser(strings.NewReader(raw))
}

func TestAccountHandler_ManualProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid account ID", func(t *testing.T) {
		handler := &AccountHandler{}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}
		c.Request = httptest.NewRequest("POST", "/api/v1/admin/accounts/invalid/manual-probe", strings.NewReader(`{}`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ManualProbe(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAccountHandler_ManualProbeReturnsJSONWithoutSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := service.Account{
		ID:          42,
		Name:        "openai-oauth",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "test-token",
		},
	}
	adminSvc := &stubAdminService{accounts: []service.Account{account}}
	repo := &manualProbeAccountRepo{account: &account}
	upstream := &manualProbeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			``,
			`data: {"type":"response.completed"}`,
			``,
		}, "\n"))),
	}}
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, &config.Config{}, nil),
	}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/manual-probe", handler.ManualProbe)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/manual-probe", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.NotContains(t, w.Body.String(), "data:")

	var payload ManualProbeResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	if assert.NotNil(t, payload.Result) {
		assert.True(t, payload.Result.Success)
		assert.NotNil(t, payload.Result.LatencyMS)
	}
}

func TestAccountHandler_ManualProbeKeepsOpenAIDefaultModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := service.Account{
		ID:          43,
		Name:        "openai-oauth-default",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "test-token",
		},
	}
	adminSvc := &stubAdminService{accounts: []service.Account{account}}
	repo := &manualProbeAccountRepo{account: &account}
	upstream := &manualProbeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			``,
			`data: {"type":"response.completed"}`,
			``,
		}, "\n"))),
	}}
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, &config.Config{}, nil),
	}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/manual-probe", handler.ManualProbe)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/43/manual-probe", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	if assert.NotEmpty(t, upstream.requestBodies) {
		assert.Contains(t, upstream.requestBodies[0], openai.DefaultTestModel)
		assert.NotContains(t, upstream.requestBodies[0], "claude-opus-4-8")
	}
}

func TestAccountHandler_ManualProbeReturnsAPIKeyItemsWithCoolingError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := service.Account{
		ID:          46,
		Name:        "openai-api-key-cooling",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_keys": []string{"sk-first", "sk-second"},
			service.CredentialAPIKeysDisabled: map[string]any{
				service.FingerprintAPIKey("sk-first"): map[string]any{
					"reason":         "invalid_api_key",
					"last_error":     "API returned 401: invalid key",
					"disabled_at":    time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
					"disabled_until": time.Now().Add(29 * time.Minute).UTC().Format(time.RFC3339),
					"disabled_count": 1,
				},
			},
		},
	}
	adminSvc := &manualProbeAdminService{stubAdminService: &stubAdminService{}, account: &account}
	repo := &manualProbeAccountRepo{account: &account}
	upstream := &manualProbeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			``,
			`data: {"type":"response.completed"}`,
			``,
		}, "\n"))),
	}}
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, &config.Config{}, nil),
	}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/manual-probe", handler.ManualProbe)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/46/manual-probe", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var payload struct {
		Success bool `json:"success"`
		Account struct {
			APIKeyItems []struct {
				Fingerprint   string `json:"fingerprint"`
				Status        string `json:"status"`
				Disabled      bool   `json:"disabled"`
				Reason        string `json:"reason"`
				LastError     string `json:"last_error"`
				DisabledUntil string `json:"disabled_until"`
			} `json:"api_key_items"`
			Credentials map[string]any `json:"credentials"`
		} `json:"account"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	if assert.Len(t, payload.Account.APIKeyItems, 2) {
		assert.Equal(t, service.FingerprintAPIKey("sk-first"), payload.Account.APIKeyItems[0].Fingerprint)
		assert.Equal(t, "cooling", payload.Account.APIKeyItems[0].Status)
		assert.True(t, payload.Account.APIKeyItems[0].Disabled)
		assert.Equal(t, "invalid_api_key", payload.Account.APIKeyItems[0].Reason)
		assert.Equal(t, "API returned 401: invalid key", payload.Account.APIKeyItems[0].LastError)
		assert.NotEmpty(t, payload.Account.APIKeyItems[0].DisabledUntil)
	}
	assert.NotContains(t, payload.Account.Credentials, "api_keys")
}

func TestAccountHandler_ManualProbeReturnsAPIKeyItemsWithUpstream503Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := service.Account{
		ID:          47,
		Name:        "openai-api-key-upstream-503",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_keys": []string{"sk-upstream-503"},
		},
	}
	adminSvc := &manualProbeAdminService{stubAdminService: &stubAdminService{}, account: &account}
	repo := &manualProbeAccountRepo{account: &account}
	upstream := &manualProbeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream temporarily unavailable"}}`)),
	}}
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, &config.Config{}, nil),
	}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/manual-probe", handler.ManualProbe)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/47/manual-probe", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var payload struct {
		Success bool `json:"success"`
		Account struct {
			APIKeyItems []struct {
				Fingerprint   string `json:"fingerprint"`
				Status        string `json:"status"`
				Disabled      bool   `json:"disabled"`
				Reason        string `json:"reason"`
				LastError     string `json:"last_error"`
				DisabledUntil string `json:"disabled_until"`
			} `json:"api_key_items"`
			Credentials map[string]any `json:"credentials"`
		} `json:"account"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	if assert.Len(t, payload.Account.APIKeyItems, 1) {
		assert.Equal(t, service.FingerprintAPIKey("sk-upstream-503"), payload.Account.APIKeyItems[0].Fingerprint)
		assert.Equal(t, "cooling", payload.Account.APIKeyItems[0].Status)
		assert.True(t, payload.Account.APIKeyItems[0].Disabled)
		assert.Equal(t, "upstream_error", payload.Account.APIKeyItems[0].Reason)
		assert.Contains(t, payload.Account.APIKeyItems[0].LastError, "API returned 503")
		assert.Contains(t, payload.Account.APIKeyItems[0].LastError, "upstream temporarily unavailable")
		assert.NotEmpty(t, payload.Account.APIKeyItems[0].DisabledUntil)
	}
	assert.NotContains(t, payload.Account.Credentials, "api_keys")
}

func TestAccountHandler_ManualProbeRepairsSchedulingPoolState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	until := time.Now().Add(10 * time.Minute)
	account := service.Account{
		ID:                      44,
		Name:                    "openai-scheduling-pool",
		Platform:                service.PlatformOpenAI,
		Type:                    service.AccountTypeOAuth,
		Status:                  service.StatusError,
		ErrorMessage:            "upstream failed",
		Schedulable:             false,
		TempUnschedulableUntil:  &until,
		TempUnschedulableReason: "upstream_5xx",
		Credentials: map[string]any{
			"access_token": "test-token",
		},
		Extra: map[string]any{
			service.AccountProbeHealthExtraKey: map[string]any{
				"level":         service.AccountProbeHealthLineDegraded,
				"failure_count": 2,
				"last_error":    "unexpected EOF",
			},
		},
	}
	adminSvc := &manualProbeAdminService{stubAdminService: &stubAdminService{}, account: &account}
	repo := &manualProbeAccountRepo{account: &account}
	upstream := &manualProbeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			``,
			`data: {"type":"response.completed"}`,
			``,
		}, "\n"))),
	}}
	rateLimitSvc := service.NewRateLimitService(repo, nil, nil, nil, nil)
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, nil, nil),
		rateLimitService:   rateLimitSvc,
	}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/manual-probe", handler.ManualProbe)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/44/manual-probe", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var payload ManualProbeResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	if assert.NotNil(t, payload.Account) {
		assert.True(t, payload.Account.Schedulable)
		assert.Empty(t, strings.TrimSpace(payload.Account.ErrorMessage))
		assert.Nil(t, payload.Account.TempUnschedulableUntil)
	}
	health, ok := account.Extra[service.AccountProbeHealthExtraKey].(map[string]any)
	if assert.True(t, ok) {
		assert.Equal(t, service.AccountProbeHealthNormal, health["level"])
		assert.Equal(t, 0, health["failure_count"])
		assert.NotContains(t, health, "last_error")
	}
}

func TestAccountHandler_TestRepairsSchedulingPoolState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	until := time.Now().Add(10 * time.Minute)
	account := service.Account{
		ID:                      45,
		Name:                    "openai-test-restore",
		Platform:                service.PlatformOpenAI,
		Type:                    service.AccountTypeOAuth,
		Status:                  service.StatusError,
		ErrorMessage:            "upstream failed",
		Schedulable:             false,
		TempUnschedulableUntil:  &until,
		TempUnschedulableReason: "upstream_5xx",
		Credentials: map[string]any{
			"access_token": "test-token",
		},
		Extra: map[string]any{
			service.AccountProbeHealthExtraKey: map[string]any{
				"level":         service.AccountProbeHealthTempUnsched,
				"failure_count": 3,
				"last_error":    "upstream failed",
			},
		},
	}
	repo := &manualProbeAccountRepo{account: &account}
	upstream := &manualProbeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			``,
			`data: {"type":"response.completed"}`,
			``,
		}, "\n"))),
	}}
	rateLimitSvc := service.NewRateLimitService(repo, nil, nil, nil, nil)
	handler := &AccountHandler{
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, nil, nil),
		rateLimitService:   rateLimitSvc,
	}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/test", handler.Test)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/45/test", strings.NewReader(`{"model_id":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, account.Schedulable)
	assert.Empty(t, strings.TrimSpace(account.ErrorMessage))
	assert.Nil(t, account.TempUnschedulableUntil)
	assert.Contains(t, w.Body.String(), `"type":"status"`)
	assert.Contains(t, w.Body.String(), `"state_before":"temp_unschedulable"`)
	assert.Contains(t, w.Body.String(), `"state_after":"normal"`)
	assert.NotContains(t, w.Body.String(), "账号状态保持")

	health, ok := account.Extra[service.AccountProbeHealthExtraKey].(map[string]any)
	if assert.True(t, ok) {
		assert.Equal(t, service.AccountProbeHealthNormal, health["level"])
		assert.Equal(t, 0, health["failure_count"])
		assert.NotContains(t, health, "last_error")
	}
}

func TestManualProbeResultResponseFromService(t *testing.T) {
	latencyMS := 1472
	firstTokenMS := 310
	result := manualProbeResultResponseFromService(&service.AccountTestConnectionResult{
		Success:      true,
		LatencyMs:    &latencyMS,
		FirstTokenMs: &firstTokenMS,
		HTTPStatus:   http.StatusOK,
		Reason:       "probe_success",
	})

	payload := ManualProbeResponse{
		Success: true,
		Result:  result,
	}

	raw, err := json.Marshal(payload)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"success": true,
		"result": {
			"success": true,
			"latency_ms": 1472,
			"first_token_ms": 310,
			"http_status": 200,
			"reason": "probe_success"
		}
	}`, string(raw))
	assert.NotContains(t, string(raw), "Success")
	assert.NotContains(t, string(raw), "LatencyMs")
	assert.NotContains(t, string(raw), "FirstTokenMs")
}

func TestManualProbeResultResponseFromServiceMapsFailureMessage(t *testing.T) {
	result := manualProbeResultResponseFromService(&service.AccountTestConnectionResult{
		Success:      false,
		ErrorMessage: "invalid api key",
		HTTPStatus:   http.StatusUnauthorized,
		Reason:       "auth_failed",
	})

	raw, err := json.Marshal(result)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"success": false,
		"message": "invalid api key",
		"error": "invalid api key",
		"http_status": 401,
		"reason": "auth_failed"
	}`, string(raw))
}
