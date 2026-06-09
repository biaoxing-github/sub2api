package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountHandlerListSchedulingPoolMapsFilterAndRedactsAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	generatedAt := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	reader := &stubAccountSchedulingPoolReader{
		snapshot: service.OpenAIAccountSchedulingPoolSnapshot{
			Items: []service.OpenAIAccountSchedulingPoolItem{
				{
					Account: service.Account{
						ID:          101,
						Name:        "ready-pool",
						Platform:    service.PlatformOpenAI,
						Type:        service.AccountTypeAPIKey,
						Status:      service.StatusActive,
						Schedulable: true,
						Credentials: map[string]any{
							"api_key": "sk-should-not-leak",
						},
					},
					PoolStatus:          service.OpenAIAccountSchedulingPoolStatusDegraded,
					PoolReasons:         []string{"path_health:degraded:unexpected_eof"},
					RuntimeBlock:        &service.OpenAIAccountRuntimeBlockSnapshot{Reason: "429", Until: &generatedAt},
					PathHealth:          service.OpenAIPathHealthRecord{State: service.OpenAIPathHealthStateDegraded, LastFailureReason: string(service.OpenAIPathFailureEOF)},
					PathHealthAvailable: true,
					DerivedHealth:       service.AccountDerivedHealthState{State: service.AccountDerivedHealthLineDegraded, Label: "线路降级"},
					EffectiveLoadFactor: 3,
				},
			},
			Total:           1,
			DegradedCount:   1,
			GeneratedAt:     generatedAt,
			GroupID:         schedulingPoolHandlerInt64Ptr(7),
			Model:           "gpt-5.5",
			Endpoint:        string(service.OpenAIEndpointCapabilityResponses),
			Transport:       string(service.OpenAIUpstreamTransportHTTPSSE),
			ImageCapability: string(service.OpenAIImagesCapabilityNative),
		},
	}
	handler := &AccountHandler{openAIAccountSchedulingPoolReader: reader}
	router := gin.New()
	router.GET("/api/v1/admin/accounts/scheduling-pool", handler.ListSchedulingPool)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/scheduling-pool?group=7&model=gpt-5.5&endpoint=responses&transport=http_sse&image_capability=images-native&search=ready", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, reader.filter.GroupID)
	require.Equal(t, int64(7), *reader.filter.GroupID)
	require.Equal(t, "gpt-5.5", reader.filter.Model)
	require.Equal(t, service.OpenAIEndpointCapabilityResponses, reader.filter.Endpoint)
	require.Equal(t, service.OpenAIUpstreamTransportHTTPSSE, reader.filter.Transport)
	require.Equal(t, service.OpenAIImagesCapabilityNative, reader.filter.ImageCapability)
	require.Equal(t, "ready", reader.filter.Search)
	require.Contains(t, rec.Body.String(), `"name":"ready-pool"`)
	require.Contains(t, rec.Body.String(), `"pool_status":"degraded"`)
	require.Contains(t, rec.Body.String(), `"pool_reasons":["path_health:degraded:unexpected_eof"]`)
	require.Contains(t, rec.Body.String(), `"derived_health":{"state":"line_degraded"`)
	require.NotContains(t, rec.Body.String(), "sk-should-not-leak")
}

type stubAccountSchedulingPoolReader struct {
	filter   service.OpenAIAccountSchedulingPoolFilter
	snapshot service.OpenAIAccountSchedulingPoolSnapshot
}

func (r *stubAccountSchedulingPoolReader) ListOpenAIAccountSchedulingPool(ctx context.Context, filter service.OpenAIAccountSchedulingPoolFilter, now time.Time) (service.OpenAIAccountSchedulingPoolSnapshot, error) {
	r.filter = filter
	if r.snapshot.GeneratedAt.IsZero() {
		r.snapshot.GeneratedAt = now
	}
	return r.snapshot, nil
}

func schedulingPoolHandlerInt64Ptr(v int64) *int64 {
	return &v
}
