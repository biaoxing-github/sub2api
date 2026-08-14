package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const maxGrokStandaloneSearchResponseBytes = 4 << 20

// ResolveGrokStandaloneSearchModel 返回独立搜索入口使用的 Grok 默认文本模型。
func ResolveGrokStandaloneSearchModel() string {
	return grokDefaultResponsesModel
}

// DoGrokNativeResponsesJSON 向指定 Grok 账号发送非流式 Responses 请求，
// 并把可切换账号的凭据、传输和上游状态统一转换为故障切换错误。
func (s *OpenAIGatewayService) DoGrokNativeResponsesJSON(ctx context.Context, account *Account, body []byte) ([]byte, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, errors.New("http upstream not configured")
	}
	if account == nil || account.Platform != PlatformGrok {
		return nil, errors.New("grok account required")
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, &UpstreamFailoverError{StatusCode: http.StatusUnauthorized, ResponseBody: []byte(err.Error())}
	}
	upstreamReq, err := buildGrokResponsesRequest(ctx, nil, account, body, token)
	if err != nil {
		return nil, fmt.Errorf("build grok responses request: %w", err)
	}
	upstreamReq.Header.Set("Accept", "application/json")

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway, ResponseBody: []byte(err.Error())}
	}
	defer func() { _ = resp.Body.Close() }()

	limited := io.LimitReader(resp.Body, maxGrokStandaloneSearchResponseBytes+1)
	respBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway, ResponseBody: []byte(err.Error())}
	}
	if len(respBytes) > maxGrokStandaloneSearchResponseBytes {
		return nil, fmt.Errorf("grok search response exceeds %d bytes", maxGrokStandaloneSearchResponseBytes)
	}
	s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
	if resp.StatusCode < http.StatusBadRequest {
		return respBytes, nil
	}

	if isGrokContentPolicyRejection(resp.StatusCode, respBytes) {
		return nil, fmt.Errorf("grok content policy rejection: %s", sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBytes)))
	}
	if s.shouldFailoverGrokUpstreamError(resp.StatusCode, respBytes) {
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           bytes.Clone(respBytes),
			RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
		}
	}
	message := string(respBytes)
	if len(message) > 200 {
		message = message[:200]
	}
	return nil, fmt.Errorf("grok upstream %d: %s", resp.StatusCode, message)
}
