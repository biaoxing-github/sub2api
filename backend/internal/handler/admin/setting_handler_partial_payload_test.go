package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newPartialSettingsTestHandler 创建只依赖内存仓储的设置处理器。
func newPartialSettingsTestHandler(values map[string]string) (*SettingHandler, *settingHandlerRepoStub) {
	repo := &settingHandlerRepoStub{values: values}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	return NewSettingHandler(svc, nil, nil, nil, nil, nil, nil), repo
}

// doPartialSettingsUpdate 执行一次管理端设置更新请求。
func doPartialSettingsUpdate(t *testing.T, handler *SettingHandler, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateSettings(ctx)
	return recorder
}

func TestUpdateSettingsPartialPayloadKeepsUnsentKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, repo := newPartialSettingsTestHandler(map[string]string{
		service.SettingKeySiteName:         "Example Gateway",
		service.SettingKeySiteSubtitle:     "Example Gateway Platform",
		service.SettingKeySMTPHost:         "smtp.example.com",
		service.SettingKeySMTPFrom:         "noreply@example.com",
		service.SettingKeyTurnstileEnabled: "true",
	})

	recorder := doPartialSettingsUpdate(t, handler, map[string]any{"risk_control_enabled": true})
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyRiskControlEnabled])
	require.Equal(t, "Example Gateway", repo.values[service.SettingKeySiteName])
	require.Equal(t, "Example Gateway Platform", repo.values[service.SettingKeySiteSubtitle])
	require.Equal(t, "smtp.example.com", repo.values[service.SettingKeySMTPHost])
	require.Equal(t, "noreply@example.com", repo.values[service.SettingKeySMTPFrom])
	require.Equal(t, "true", repo.values[service.SettingKeyTurnstileEnabled])
}

func TestUpdateSettingsExplicitEmptyFieldStillClearsValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, repo := newPartialSettingsTestHandler(map[string]string{
		service.SettingKeySiteName: "Example Gateway",
	})

	recorder := doPartialSettingsUpdate(t, handler, map[string]any{"site_name": ""})
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "", repo.values[service.SettingKeySiteName])
}

func TestUpdateSettingsSMTPFromEmailAliasIsWritable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, repo := newPartialSettingsTestHandler(map[string]string{
		service.SettingKeySMTPFrom: "old@example.com",
	})

	recorder := doPartialSettingsUpdate(t, handler, map[string]any{"smtp_from_email": "new@example.com"})
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "new@example.com", repo.values[service.SettingKeySMTPFrom])
}

func TestUpdateSettingsChannelMonitorV2FieldsAreWritableAndReturned(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, repo := newPartialSettingsTestHandler(map[string]string{
		service.SettingKeyChannelMonitorEnabled:                "true",
		service.SettingKeyChannelMonitorMode:                   service.ChannelMonitorModeV1,
		service.SettingKeyChannelMonitorDefaultIntervalSeconds: "60",
		service.SettingKeyChannelMonitorHideThroughput:         "true",
	})

	recorder := doPartialSettingsUpdate(t, handler, map[string]any{
		"channel_monitor_mode":            service.ChannelMonitorModeV2,
		"channel_monitor_hide_throughput": false,
	})
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.ChannelMonitorModeV2, repo.values[service.SettingKeyChannelMonitorMode])
	require.Equal(t, "false", repo.values[service.SettingKeyChannelMonitorHideThroughput])
	require.Equal(t, "60", repo.values[service.SettingKeyChannelMonitorDefaultIntervalSeconds])

	var responseBody struct {
		Data struct {
			ChannelMonitorMode           string `json:"channel_monitor_mode"`
			ChannelMonitorHideThroughput bool   `json:"channel_monitor_hide_throughput"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
	require.Equal(t, service.ChannelMonitorModeV2, responseBody.Data.ChannelMonitorMode)
	require.False(t, responseBody.Data.ChannelMonitorHideThroughput)
}

func TestUpdateSettingsChannelMonitorV2FieldsKeepPreviousValuesWhenOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, repo := newPartialSettingsTestHandler(map[string]string{
		service.SettingKeyChannelMonitorMode:           service.ChannelMonitorModeV2,
		service.SettingKeyChannelMonitorHideThroughput: "false",
	})

	recorder := doPartialSettingsUpdate(t, handler, map[string]any{"risk_control_enabled": true})
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.ChannelMonitorModeV2, repo.values[service.SettingKeyChannelMonitorMode])
	require.Equal(t, "false", repo.values[service.SettingKeyChannelMonitorHideThroughput])
}
