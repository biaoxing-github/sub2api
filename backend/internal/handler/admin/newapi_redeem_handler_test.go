package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProjectNewAPIRedeemRunLogsSupportsSummaryAndLatestLimit(t *testing.T) {
	logs := make([]service.NewAPIRedeemRequestLog, 150)
	for index := range logs {
		logs[index] = service.NewAPIRedeemRequestLog{Code: fmt.Sprintf("code-%03d", index)}
	}
	run := service.NewAPIRedeemRun{ID: "run-1", Logs: logs}

	summary := projectNewAPIRedeemRunLogs(run, false, 0)
	require.Empty(t, summary.Logs)

	limited := projectNewAPIRedeemRunLogs(run, true, 100)
	require.Len(t, limited.Logs, 100)
	require.Equal(t, "code-050", limited.Logs[0].Code)
	require.Equal(t, "code-149", limited.Logs[99].Code)
	require.Len(t, run.Logs, 150)
}

func TestNewAPIRedeemHandlerImportAndUploadUseStandardEnvelopeWithoutAccessKeyLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewNewAPIRedeemHandler(service.NewNewAPIRedeemService(service.NewAPIRedeemOptions{RootDir: t.TempDir()}))
	router := gin.New()
	router.POST("/accounts/import", handler.ImportAccounts)
	router.POST("/files", handler.UploadFiles)
	router.GET("/overview", handler.Overview)

	importRecorder := httptest.NewRecorder()
	importRequest := httptest.NewRequest(http.MethodPost, "/accounts/import", bytes.NewBufferString(`{"accounts":[{"user_id":"14690","access_key":"private-source-key"}]}`))
	importRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(importRecorder, importRequest)
	require.Equal(t, http.StatusOK, importRecorder.Code)
	require.NotContains(t, importRecorder.Body.String(), "private-source-key")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "codes.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("first-code\nsecond-code\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/files", &body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(uploadRecorder, uploadRequest)
	require.Equal(t, http.StatusOK, uploadRecorder.Code)
	require.Contains(t, uploadRecorder.Body.String(), `"code_count":2`)

	overviewRecorder := httptest.NewRecorder()
	router.ServeHTTP(overviewRecorder, httptest.NewRequest(http.MethodGet, "/overview", nil))
	require.Equal(t, http.StatusOK, overviewRecorder.Code)
	require.NotContains(t, overviewRecorder.Body.String(), "private-source-key")
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Accounts []service.NewAPIRedeemAccount     `json:"accounts"`
			Files    []service.NewAPIRedeemVoucherFile `json:"files"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(overviewRecorder.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Len(t, payload.Data.Accounts, 1)
	require.Len(t, payload.Data.Files, 1)
}

func TestNewAPIRedeemHandlerStartRunReturnsAcceptedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/user/topup", request.URL.Path)
		_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
	}))
	defer upstream.Close()
	redeemService := service.NewNewAPIRedeemService(service.NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: upstream.URL, HTTPClient: upstream.Client(), RequestInterval: time.Millisecond})
	accounts, err := redeemService.ImportAccounts(context.Background(), []service.NewAPIRedeemAccountInput{{UserID: "14744", AccessKey: "key"}})
	require.NoError(t, err)
	file, err := redeemService.SaveVoucherContent(context.Background(), "codes.txt", []byte("code\n"))
	require.NoError(t, err)
	handler := NewNewAPIRedeemHandler(redeemService)
	router := gin.New()
	router.POST("/runs", handler.StartRun)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewBufferString(`{"file_ids":["`+file.ID+`"],"account_ids":["`+accounts[0].ID+`"]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":0`)
	var startPayload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &startPayload))
	require.NotEmpty(t, startPayload.Data.ID)
	require.Eventually(t, func() bool {
		run, runErr := redeemService.GetRun(context.Background(), startPayload.Data.ID)
		return runErr == nil && run.Status == "completed"
	}, 2*time.Second, 5*time.Millisecond)
}
