package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNewAPICheckinHandlerConfigEnvelopeKeepsDisabledSiteVisible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestNewAPICheckinHandler(t)
	router := gin.New()
	router.GET("/config", handler.GetConfig)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/config", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Code int                                `json:"code"`
		Data service.NewAPICheckinConfigSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, 2, payload.Data.AllSiteCount)
	require.Equal(t, 1, payload.Data.EnabledSiteCount)
	require.False(t, payload.Data.Sites[1].Enabled)
	require.Equal(t, "turnstile-site", payload.Data.Sites[1].Name)
}

func TestNewAPICheckinHandlerSetSiteEnabledUsesSQLStorageLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestNewAPICheckinHandler(t)
	router := gin.New()
	router.POST("/site-enabled", handler.SetSiteEnabled)

	body := strings.NewReader(`{"site":"turnstile-site","enabled":false,"disabled_reason":"Turnstile 保护站点，仅保留余额与月度记录查询"}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/site-enabled", body))

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Code int                                    `json:"code"`
		Data service.NewAPICheckinSiteEnabledResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.False(t, payload.Data.Enabled)
	require.Equal(t, "sql:newapi-checkin/config", payload.Data.ConfigPath)
}

func newTestNewAPICheckinHandler(t *testing.T) *NewAPICheckinHandler {
	t.Helper()
	repo := &newAPICheckinHandlerMemoryRepo{
		config: service.NewAPICheckinConfig{
			DefaultCheckinPath:      "/api/user/checkin",
			DelayBetweenCheckinsSec: 0,
			RouteSwitchWaitSec:      0,
			NotifyFeishu:            false,
			Sites: []service.NewAPICheckinSite{
				{
					Name:                     "enabled-site",
					Enabled:                  true,
					BackgroundCheckinEnabled: true,
					BaseURL:                  "https://enabled.example",
					Accounts: []service.NewAPICheckinAccount{
						{Name: "alpha", UserID: "1001", AccessKey: "key-a", IPProfile: "slot-a", Enabled: true},
					},
				},
				{
					Name:                     "turnstile-site",
					Enabled:                  false,
					DisabledReason:           "Turnstile 保护站点，仅保留余额与月度记录查询",
					BackgroundCheckinEnabled: true,
					BaseURL:                  "https://turnstile.example",
					Accounts: []service.NewAPICheckinAccount{
						{Name: "beta", UserID: "2001", AccessKey: "key-b", IPProfile: "slot-b", Enabled: true},
					},
				},
			},
		},
		balance: service.NewAPICheckinBalancePayload{
			SiteStatuses: map[string]service.NewAPICheckinSiteStatus{},
		},
	}
	svc := service.NewNewAPICheckinService(service.NewAPICheckinOptions{
		Repository: repo,
		Now: func() time.Time {
			return time.Date(2026, 7, 8, 9, 10, 11, 0, time.UTC)
		},
	})
	return NewNewAPICheckinHandler(svc)
}

type newAPICheckinHandlerMemoryRepo struct {
	config  service.NewAPICheckinConfig
	report  service.NewAPICheckinReport
	balance service.NewAPICheckinBalancePayload
	history service.NewAPICheckinHistoryPayload
	monthly []service.NewAPICheckinMonthlyRecord
}

func (r *newAPICheckinHandlerMemoryRepo) LoadConfig(context.Context) (service.NewAPICheckinConfig, error) {
	return r.config, nil
}

func (r *newAPICheckinHandlerMemoryRepo) SaveConfig(_ context.Context, cfg service.NewAPICheckinConfig) error {
	r.config = cfg
	return nil
}

func (r *newAPICheckinHandlerMemoryRepo) LoadLatestReport(context.Context) (service.NewAPICheckinReport, error) {
	return r.report, nil
}

func (r *newAPICheckinHandlerMemoryRepo) SaveLatestReport(_ context.Context, report service.NewAPICheckinReport) error {
	r.report = report
	return nil
}

func (r *newAPICheckinHandlerMemoryRepo) LoadBalanceCache(context.Context) (service.NewAPICheckinBalancePayload, error) {
	return r.balance, nil
}

func (r *newAPICheckinHandlerMemoryRepo) SaveBalanceCache(_ context.Context, cache service.NewAPICheckinBalancePayload) error {
	r.balance = cache
	return nil
}

func (r *newAPICheckinHandlerMemoryRepo) LoadHistory(context.Context) (service.NewAPICheckinHistoryPayload, error) {
	return r.history, nil
}

func (r *newAPICheckinHandlerMemoryRepo) SaveHistory(_ context.Context, payload service.NewAPICheckinHistoryPayload) error {
	r.history = payload
	return nil
}

func (r *newAPICheckinHandlerMemoryRepo) LoadMonthlyRecords(context.Context) ([]service.NewAPICheckinMonthlyRecord, error) {
	return r.monthly, nil
}

func (r *newAPICheckinHandlerMemoryRepo) SaveMonthlyRecords(_ context.Context, records []service.NewAPICheckinMonthlyRecord) error {
	r.monthly = records
	return nil
}

func (r *newAPICheckinHandlerMemoryRepo) StorageLabel() string {
	return "sql:newapi-checkin"
}

var _ service.NewAPICheckinRepository = (*newAPICheckinHandlerMemoryRepo)(nil)
