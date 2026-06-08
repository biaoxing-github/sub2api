package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	openAISchedulerExhaustionNotifyChannelFeishuWebhook = "feishu_webhook"
	openAISchedulerExhaustionNotifyChannelFeishuApp     = "feishu_app"
	openAISchedulerExhaustionNotifyPhaseWaiting         = "waiting"
	openAISchedulerExhaustionNotifyPhaseRecovered       = "recovered"
	openAISchedulerExhaustionNotifyHTTPTimeout          = 5 * time.Second
	openAISchedulerExhaustionNotifyBodyReadLimit        = 1 << 20
)

// openAISchedulerExhaustionProbeNotifyEvent 描述无限小请求探测的等待或恢复通知内容。
type openAISchedulerExhaustionProbeNotifyEvent struct {
	Phase          string
	StartedAt      time.Time
	Elapsed        time.Duration
	Rounds         int
	Attempts       int
	CandidateCount int
	RequestedModel string
	RequireCompact bool
	AccountID      int64
	AccountName    string
	LastError      string
}

func (s *OpenAIGatewayService) nowOpenAISchedulerExhaustionProbe() time.Time {
	if s != nil && s.openAISchedulerExhaustionProbeNow != nil {
		return s.openAISchedulerExhaustionProbeNow()
	}
	return time.Now()
}

func (s *OpenAIGatewayService) openAISchedulerExhaustionProbeNotifySettings() (bool, time.Duration, time.Duration, bool) {
	if s == nil || s.cfg == nil {
		return false,
			time.Duration(config.DefaultOpenAISchedulerProbeNotifyAfterSeconds) * time.Second,
			time.Duration(config.DefaultOpenAISchedulerProbeNotifyRepeatSeconds) * time.Second,
			true
	}
	gateway := s.cfg.Gateway
	afterSeconds := positiveIntOrDefault(gateway.OpenAISchedulerProbeNotifyAfterSeconds, config.DefaultOpenAISchedulerProbeNotifyAfterSeconds)
	repeatSeconds := positiveIntOrDefault(gateway.OpenAISchedulerProbeNotifyRepeatSeconds, config.DefaultOpenAISchedulerProbeNotifyRepeatSeconds)
	return gateway.OpenAISchedulerProbeNotifyEnabled,
		time.Duration(afterSeconds) * time.Second,
		time.Duration(repeatSeconds) * time.Second,
		gateway.OpenAISchedulerProbeNotifyRecoveredEnabled
}

func (s *OpenAIGatewayService) notifyOpenAISchedulerExhaustionProbe(ctx context.Context, event openAISchedulerExhaustionProbeNotifyEvent) {
	if s == nil {
		return
	}
	if s.openAISchedulerExhaustionNotifyFunc != nil {
		if err := s.openAISchedulerExhaustionNotifyFunc(ctx, event); err != nil {
			slog.Warn("openai scheduler exhaustion probe notification hook failed", "phase", event.Phase, "err", err)
		}
		return
	}
	if s.cfg == nil || !s.cfg.Gateway.OpenAISchedulerProbeNotifyEnabled {
		return
	}
	channel := resolveOpenAISchedulerProbeNotifyChannel(s.cfg.Gateway)
	if channel == openAISchedulerExhaustionNotifyChannelFeishuApp {
		if err := s.sendOpenAISchedulerExhaustionFeishuAppNotification(ctx, s.cfg.Gateway, event); err != nil {
			slog.Warn("openai scheduler exhaustion probe feishu app notification failed",
				"phase", event.Phase,
				"elapsed", event.Elapsed.String(),
				"attempts", event.Attempts,
				"err", err,
			)
		}
		return
	}
	webhookURL := strings.TrimSpace(s.cfg.Gateway.OpenAISchedulerProbeNotifyFeishuWebhookURL)
	if webhookURL == "" {
		slog.Warn("openai scheduler exhaustion probe notification skipped because feishu webhook is empty",
			"phase", event.Phase,
			"elapsed", event.Elapsed.String(),
			"attempts", event.Attempts,
		)
		return
	}
	if err := s.sendOpenAISchedulerExhaustionFeishuNotification(ctx, webhookURL, event); err != nil {
		slog.Warn("openai scheduler exhaustion probe feishu notification failed",
			"phase", event.Phase,
			"elapsed", event.Elapsed.String(),
			"attempts", event.Attempts,
			"err", err,
		)
	}
}

func (s *OpenAIGatewayService) sendOpenAISchedulerExhaustionFeishuNotification(ctx context.Context, webhookURL string, event openAISchedulerExhaustionProbeNotifyEvent) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return fmt.Errorf("feishu webhook url is empty")
	}
	reqCtx, cancel := context.WithTimeout(ctx, openAISchedulerExhaustionNotifyHTTPTimeout)
	defer cancel()

	payload := struct {
		MsgType string `json:"msg_type"`
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	}{
		MsgType: "text",
	}
	payload.Content.Text = formatOpenAISchedulerExhaustionFeishuText(event)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal feishu scheduler exhaustion notification: %w", err)
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build feishu scheduler exhaustion notification request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")

	client := &http.Client{Timeout: openAISchedulerExhaustionNotifyHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu scheduler exhaustion notification: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("feishu scheduler exhaustion notification status %d", resp.StatusCode)
	}
	return nil
}

func (s *OpenAIGatewayService) sendOpenAISchedulerExhaustionFeishuAppNotification(ctx context.Context, gateway config.GatewayConfig, event openAISchedulerExhaustionProbeNotifyEvent) error {
	appID := strings.TrimSpace(gateway.OpenAISchedulerProbeNotifyFeishuAppID)
	appSecret := strings.TrimSpace(gateway.OpenAISchedulerProbeNotifyFeishuAppSecret)
	receiveIDType := normalizeOpenAISchedulerProbeNotifyFeishuReceiveIDType(gateway.OpenAISchedulerProbeNotifyFeishuReceiveIDType)
	receiveID := strings.TrimSpace(gateway.OpenAISchedulerProbeNotifyFeishuReceiveID)
	if appID == "" {
		return fmt.Errorf("feishu app id is empty")
	}
	if appSecret == "" {
		return fmt.Errorf("feishu app secret is empty")
	}
	if receiveID == "" {
		return fmt.Errorf("feishu receive id is empty")
	}
	baseURL, err := openAISchedulerProbeFeishuOpenAPIBaseURL(gateway.OpenAISchedulerProbeNotifyFeishuDomain)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, openAISchedulerExhaustionNotifyHTTPTimeout)
	defer cancel()
	client := &http.Client{Timeout: openAISchedulerExhaustionNotifyHTTPTimeout}
	token, err := requestOpenAISchedulerProbeFeishuTenantAccessToken(reqCtx, client, baseURL, appID, appSecret)
	if err != nil {
		return err
	}
	if err := sendOpenAISchedulerProbeFeishuAppTextMessage(reqCtx, client, baseURL, token, receiveIDType, receiveID, formatOpenAISchedulerExhaustionFeishuText(event)); err != nil {
		return err
	}
	return nil
}

func requestOpenAISchedulerProbeFeishuTenantAccessToken(ctx context.Context, client *http.Client, baseURL string, appID string, appSecret string) (string, error) {
	payload := map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal feishu tenant token request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build feishu tenant token request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request feishu tenant token: %w", err)
	}
	defer resp.Body.Close()
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, openAISchedulerExhaustionNotifyBodyReadLimit))
	if readErr != nil {
		return "", fmt.Errorf("read feishu tenant token response: %w", readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("feishu tenant token status %d", resp.StatusCode)
	}
	var tokenResp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("decode feishu tenant token response: %w", err)
	}
	if tokenResp.Code != 0 {
		return "", fmt.Errorf("feishu tenant token code %d msg %s", tokenResp.Code, truncateForLog([]byte(tokenResp.Msg), 256))
	}
	if strings.TrimSpace(tokenResp.TenantAccessToken) == "" {
		return "", fmt.Errorf("feishu tenant token is empty")
	}
	return strings.TrimSpace(tokenResp.TenantAccessToken), nil
}

func sendOpenAISchedulerProbeFeishuAppTextMessage(ctx context.Context, client *http.Client, baseURL string, tenantAccessToken string, receiveIDType string, receiveID string, text string) error {
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("marshal feishu message content: %w", err)
	}
	payload := map[string]string{
		"receive_id": strings.TrimSpace(receiveID),
		"msg_type":   "text",
		"content":    string(content),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal feishu app message: %w", err)
	}
	messageURL := strings.TrimRight(baseURL, "/") + "/open-apis/im/v1/messages?receive_id_type=" + url.QueryEscape(normalizeOpenAISchedulerProbeNotifyFeishuReceiveIDType(receiveIDType))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, messageURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build feishu app message request: %w", err)
	}
	req.Header.Set("authorization", "Bearer "+strings.TrimSpace(tenantAccessToken))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu app message: %w", err)
	}
	defer resp.Body.Close()
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, openAISchedulerExhaustionNotifyBodyReadLimit))
	if readErr != nil {
		return fmt.Errorf("read feishu app message response: %w", readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("feishu app message status %d", resp.StatusCode)
	}
	var messageResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &messageResp); err != nil {
		return fmt.Errorf("decode feishu app message response: %w", err)
	}
	if messageResp.Code != 0 {
		return fmt.Errorf("feishu app message code %d msg %s", messageResp.Code, truncateForLog([]byte(messageResp.Msg), 256))
	}
	return nil
}

func formatOpenAISchedulerExhaustionFeishuText(event openAISchedulerExhaustionProbeNotifyEvent) string {
	elapsedSeconds := int64(event.Elapsed.Round(time.Second) / time.Second)
	if elapsedSeconds < 0 {
		elapsedSeconds = 0
	}
	title := "sub2api OpenAI /responses 无限调度等待通知"
	if event.Phase == openAISchedulerExhaustionNotifyPhaseRecovered {
		title = "sub2api OpenAI /responses 无限调度等待已恢复"
	}
	lines := []string{
		title,
		fmt.Sprintf("阶段: %s", event.Phase),
		fmt.Sprintf("已等待: %ds", elapsedSeconds),
		fmt.Sprintf("探测轮次: %d", event.Rounds),
		fmt.Sprintf("探测次数: %d", event.Attempts),
		fmt.Sprintf("候选可调度账号数: %d", event.CandidateCount),
	}
	if event.RequestedModel != "" {
		lines = append(lines, "请求模型: "+event.RequestedModel)
	}
	if event.RequireCompact {
		lines = append(lines, "请求类型: compact")
	}
	if event.AccountID > 0 {
		accountText := fmt.Sprintf("恢复账号: %d", event.AccountID)
		if strings.TrimSpace(event.AccountName) != "" {
			accountText += " (" + strings.TrimSpace(event.AccountName) + ")"
		}
		lines = append(lines, accountText)
	}
	if strings.TrimSpace(event.LastError) != "" {
		lines = append(lines, "最后错误: "+truncateForLog([]byte(event.LastError), 512))
	}
	return strings.Join(lines, "\n")
}

func resolveOpenAISchedulerProbeNotifyChannel(gateway config.GatewayConfig) string {
	channel := normalizeOpenAISchedulerProbeNotifyChannel(gateway.OpenAISchedulerProbeNotifyChannel)
	if channel != "" {
		return channel
	}
	if strings.TrimSpace(gateway.OpenAISchedulerProbeNotifyFeishuAppID) != "" ||
		strings.TrimSpace(gateway.OpenAISchedulerProbeNotifyFeishuAppSecret) != "" ||
		strings.TrimSpace(gateway.OpenAISchedulerProbeNotifyFeishuReceiveID) != "" {
		return openAISchedulerExhaustionNotifyChannelFeishuApp
	}
	return openAISchedulerExhaustionNotifyChannelFeishuWebhook
}

func normalizeOpenAISchedulerProbeNotifyChannel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case openAISchedulerExhaustionNotifyChannelFeishuWebhook, "webhook":
		return openAISchedulerExhaustionNotifyChannelFeishuWebhook
	case openAISchedulerExhaustionNotifyChannelFeishuApp, "app", "enterprise_app":
		return openAISchedulerExhaustionNotifyChannelFeishuApp
	default:
		return ""
	}
}

func normalizeOpenAISchedulerProbeNotifyFeishuDomain(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "feishu"
	}
	return trimmed
}

func normalizeOpenAISchedulerProbeNotifyFeishuReceiveIDType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "open_id", "user_id", "union_id", "email", "chat_id":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "chat_id"
	}
}

func openAISchedulerProbeFeishuOpenAPIBaseURL(domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" || strings.EqualFold(domain, "feishu") {
		return "https://open.feishu.cn", nil
	}
	if strings.EqualFold(domain, "lark") || strings.EqualFold(domain, "larksuite") {
		return "https://open.larksuite.com", nil
	}
	if strings.HasPrefix(strings.ToLower(domain), "http://") || strings.HasPrefix(strings.ToLower(domain), "https://") {
		parsed, err := url.Parse(domain)
		if err != nil {
			return "", fmt.Errorf("parse feishu openapi domain: %w", err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", fmt.Errorf("unsupported feishu openapi domain scheme: %s", parsed.Scheme)
		}
		if strings.TrimSpace(parsed.Host) == "" {
			return "", fmt.Errorf("feishu openapi domain missing host")
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return strings.TrimRight(parsed.String(), "/"), nil
	}
	return "https://open." + strings.Trim(domain, "/"), nil
}

func positiveIntOrDefault(value int, defaultValue int) int {
	if value > 0 {
		return value
	}
	return defaultValue
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
