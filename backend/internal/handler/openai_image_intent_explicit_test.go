package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayHandlerResponses_PassiveImageNamespaceBypassesPermissionGate(t *testing.T) {
	body := `{"model":"gpt-5.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto","input":"write code"}`
	recorder := runOpenAIResponsesExplicitImageIntentGate(t, body)

	require.NotEqual(t, http.StatusForbidden, recorder.Code)
	require.NotContains(t, recorder.Body.String(), service.ImageGenerationPermissionMessage())
}

func TestOpenAIGatewayHandlerResponses_NativeImageToolStillUsesPermissionGate(t *testing.T) {
	body := `{"model":"gpt-5.5","tools":[{"type":"image_generation"}],"input":"draw"}`
	recorder := runOpenAIResponsesExplicitImageIntentGate(t, body)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), service.ImageGenerationPermissionMessage())
}

// runOpenAIResponsesExplicitImageIntentGate 构造禁用生图分组，隔离验证入口级图片意图判断。
func runOpenAIResponsesExplicitImageIntentGate(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(6301)
	userID := int64(6302)
	ctx.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      6303,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			AllowImageGeneration: false,
		},
		User: &service.User{ID: userID, Status: service.StatusActive},
	})
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID, Concurrency: 1})

	handler := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}),
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper: &ConcurrencyHelper{concurrencyService: service.NewConcurrencyService(
			&helperConcurrencyCacheStub{userSeq: []bool{true}},
		)},
		cfg:          &config.Config{},
		imageLimiter: &imageConcurrencyLimiter{},
	}

	handler.Responses(ctx)
	return recorder
}
