package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func (r *manualProbeAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, service.ErrAccountNotFound
}

func (r *manualProbeAccountRepo) UpdateExtra(context.Context, int64, map[string]any) error {
	return nil
}

func (r *manualProbeAccountRepo) ClearError(context.Context, int64) error {
	return nil
}

func (r *manualProbeAccountRepo) SetError(context.Context, int64, string) error {
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
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, nil, nil),
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
		accountTestService: service.NewAccountTestService(repo, nil, nil, nil, upstream, nil, nil),
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
