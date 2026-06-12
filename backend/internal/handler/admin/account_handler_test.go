package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

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
