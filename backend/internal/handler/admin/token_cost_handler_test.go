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

func TestTokenCostHandlerStateViewReturnsPageCompatibleDataState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestTokenCostHandler()
	router := gin.New()
	router.GET("/state", handler.GetState)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/state?view=page", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
	require.Contains(t, recorder.Body.String(), "data-state='")
	require.Contains(t, recorder.Body.String(), "tokeness")
}

func TestTokenCostHandlerHealthUsesSQLStorageLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestTokenCostHandler()
	router := gin.New()
	router.GET("/health", handler.Health)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		OK      bool   `json:"ok"`
		State   string `json:"state"`
		Storage string `json:"storage"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.OK)
	require.Equal(t, "sql:token-cost", payload.State)
	require.Equal(t, "sql:token-cost", payload.Storage)
}

func TestTokenCostHandlerPostStateSavesRawStatePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestTokenCostHandler()
	router := gin.New()
	router.POST("/state", handler.SaveState)
	router.GET("/state", handler.GetState)

	body := strings.NewReader(`{"state":{"version":1,"updatedAt":"2026-07-06T03:38:28.869Z","rankMode":"plus","personalRechargeR":400,"platforms":[{"id":"xiaobai-code","name":"小白code","balanceUsd":20,"rateR":1,"rateUsd":1,"plus":0.1,"proMin":0.18,"note":"余额涨到 20$"}],"history":[],"events":[{"at":"2026/7/6 11:38:28","title":"编辑平台","detail":"小白code 的 plus：0.13 -> 0.1"}]}}`)
	postRecorder := httptest.NewRecorder()
	router.ServeHTTP(postRecorder, httptest.NewRequest(http.MethodPost, "/state", body))

	require.Equal(t, http.StatusOK, postRecorder.Code)
	var postPayload struct {
		OK      bool   `json:"ok"`
		State   string `json:"state"`
		Storage string `json:"storage"`
	}
	require.NoError(t, json.Unmarshal(postRecorder.Body.Bytes(), &postPayload))
	require.True(t, postPayload.OK)
	require.Equal(t, "sql:token-cost", postPayload.Storage)

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/state", nil))
	var getPayload struct {
		OK    bool                   `json:"ok"`
		State service.TokenCostState `json:"state"`
	}
	require.NoError(t, json.Unmarshal(getRecorder.Body.Bytes(), &getPayload))
	require.True(t, getPayload.OK)
	require.Len(t, getPayload.State.Platforms, 1)
	require.Equal(t, "小白code", getPayload.State.Platforms[0].Name)
	require.Equal(t, "编辑平台", getPayload.State.Events[0].Title)
}

func newTestTokenCostHandler() *TokenCostHandler {
	calcBalance := 10.7142857143
	repo := &tokenCostHandlerMemoryRepo{
		state: service.TokenCostState{
			Version:           1,
			UpdatedAt:         "2026-07-06T03:38:28.869Z",
			RankMode:          "plus",
			PersonalRechargeR: 400,
			Platforms: []service.TokenCostPlatform{
				{ID: "tokeness", Name: "tokeness", BalanceUSD: 75, CalcBalanceUSD: &calcBalance, RateR: 1, RateUSD: 1},
			},
			History: []service.TokenCostHistoryEntry{},
			Events:  []service.TokenCostEvent{},
		},
		found: true,
	}
	svc := service.NewTokenCostService(service.TokenCostOptions{
		Repository: repo,
		Now: func() time.Time {
			return time.Date(2026, 7, 8, 9, 10, 11, 0, time.UTC)
		},
	})
	return NewTokenCostHandler(svc)
}

type tokenCostHandlerMemoryRepo struct {
	state service.TokenCostState
	found bool
}

func (r *tokenCostHandlerMemoryRepo) LoadState(context.Context) (service.TokenCostState, bool, error) {
	return r.state, r.found, nil
}

func (r *tokenCostHandlerMemoryRepo) SaveState(_ context.Context, state service.TokenCostState) error {
	r.state = state
	r.found = true
	return nil
}

func (r *tokenCostHandlerMemoryRepo) StorageLabel() string {
	return "sql:token-cost"
}

var _ service.TokenCostRepository = (*tokenCostHandlerMemoryRepo)(nil)
