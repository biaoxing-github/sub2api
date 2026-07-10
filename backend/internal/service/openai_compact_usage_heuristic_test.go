//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHandleNonStreamingResponseOAuthJSONBodyWithDataEventTextKeepsJSONUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	jsonBody := `{"id":"resp_oauth_compact","object":"response","model":"gpt-5.4","status":"completed",` +
		`"output":[{"type":"message","content":[{"type":"output_text",` +
		`"text":"processing data: 1,2,3 then event: click finished"}]}],` +
		`"usage":{"input_tokens":11,"output_tokens":22,"total_tokens":33}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(jsonBody)),
	}
	account := &Account{ID: 146, Type: AccountTypeOAuth}

	result, err := svc.handleNonStreamingResponse(context.Background(), resp, c, account, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 11, result.InputTokens)
	require.Equal(t, 22, result.OutputTokens)
	require.Equal(t, "resp_oauth_compact", gjson.Get(rec.Body.String(), "id").String())
	require.Equal(t, int64(33), gjson.Get(rec.Body.String(), "usage.total_tokens").Int())
	require.Contains(t, rec.Body.String(), "processing data: 1,2,3 then event: click finished")
}
