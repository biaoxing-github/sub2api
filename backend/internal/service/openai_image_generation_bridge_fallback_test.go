//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayServiceForward_CodexBridgePermission403RetriesWithoutBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusForbidden,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Image generation is not enabled for this group","type":"permission_error"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"resp_retry_without_bridge","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":1}}`)),
			},
		},
	}
	repo := &rateLimitAccountRepoStub{}
	rateLimitSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rateLimitSvc.SetOpenAI403CounterCache(&openAI403CounterCacheStub{counts: []int64{1}})
	svc := newOpenAIImageGenerationControlTestService(upstream)
	svc.cfg.Gateway.CodexImageGenerationBridgeEnabled = true
	svc.accountRepo = repo
	svc.rateLimitService = rateLimitSvc
	c, recorder := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.98.0")
	account := newOpenAIImageGenerationControlTestAccount()

	result, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.4","input":"write memo","stream":false}`))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, upstream.bodies, 2)
	require.True(t, gjson.GetBytes(upstream.bodies[0], `tools.#(type=="image_generation")`).Exists(), "first attempt uses the bridge")
	require.False(t, gjson.GetBytes(upstream.bodies[1], `tools.#(type=="image_generation")`).Exists(), "retry must remove bridge-injected image_generation")
	require.Equal(t, 0, repo.tempCalls)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.updateExtraCalls)
	require.Equal(t, false, repo.lastExtraUpdates[featureKeyCodexImageGenerationBridge])
	require.Equal(t, false, account.Extra[featureKeyCodexImageGenerationBridge])
}
