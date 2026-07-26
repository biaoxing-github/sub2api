package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApplyOpenAIAccountPassthroughClientHeadersReplacesGoDefaultUserAgent
// 锁定 OpenAI 出站身份不变量：即使模拟开关关闭且入站来自 Go 客户端，也不能把
// Go-http-client/1.1 继续发送给上游。
func TestApplyOpenAIAccountPassthroughClientHeadersReplacesGoDefaultUserAgent(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "Go-http-client/1.1")

	applyOpenAIAccountPassthroughClientHeaders(req, nil, account, nil)

	require.Equal(t, codexCLIUserAgent(), req.Header.Get("User-Agent"))
}
