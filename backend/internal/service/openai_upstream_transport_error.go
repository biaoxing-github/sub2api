package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const openAITransportErrorTempUnschedDuration = 10 * time.Minute

var openAITransportFailoverBody = []byte(`{"error":{"type":"upstream_error","message":"Upstream request failed"}}`)

// openAITransportErrorClass 描述 OpenAI 上游传输层错误的调度影响。
type openAITransportErrorClass struct {
	// Persistent 表示同一账号/代理短时间内继续重试没有意义，需要临时摘除。
	Persistent bool
}

var openAIPersistentTransportErrorMarkers = []string{
	"authentication failed",
	"proxy authentication required",
	"connection refused",
	"no route to host",
	"network is unreachable",
	"no such host",
}

// classifyOpenAITransportError 判断 Do/DoWithTLS 返回的非 HTTP 错误是否属于持久故障。
func classifyOpenAITransportError(err error) openAITransportErrorClass {
	if err == nil {
		return openAITransportErrorClass{}
	}
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return openAITransportErrorClass{Persistent: true}
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return openAITransportErrorClass{Persistent: true}
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range openAIPersistentTransportErrorMarkers {
		if strings.Contains(msg, marker) {
			return openAITransportErrorClass{Persistent: true}
		}
	}
	return openAITransportErrorClass{}
}

// handleOpenAIUpstreamTransportError 处理 OpenAI 上游传输层失败：
// 记录运维错误；持久故障临时摘除账号；非客户端取消场景返回 failover 错误。
func (s *OpenAIGatewayService) handleOpenAIUpstreamTransportError(ctx context.Context, c *gin.Context, account *Account, err error, passthrough bool) error {
	if err == nil {
		err = errors.New("openai upstream transport error")
	}
	safeErr := sanitizeUpstreamErrorMessage(err.Error())
	if safeErr == "" {
		safeErr = "Upstream request failed"
	}
	event := OpsUpstreamErrorEvent{
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: 0,
		Passthrough:        passthrough,
		Kind:               "request_error",
		Message:            safeErr,
	}
	if account != nil {
		event.Platform = account.Platform
		event.AccountID = account.ID
		event.AccountName = account.Name
	}
	setOpsUpstreamError(c, 0, safeErr, "")
	appendOpsUpstreamError(c, event)

	if errors.Is(err, context.Canceled) {
		return err
	}
	if classifyOpenAITransportError(err).Persistent {
		s.tempUnscheduleOpenAITransportError(ctx, account, safeErr)
	}

	return &UpstreamFailoverError{
		StatusCode:   http.StatusBadGateway,
		ResponseBody: openAITransportFailoverBody,
		ActionLabel:  OpenAIStreamActionRetryNextAccount,
		ActionMetadata: map[string]string{
			"phase":  "request",
			"reason": "transport_error",
		},
	}
}

// tempUnscheduleOpenAITransportError 将持久传输故障账号写入临时不可调度状态。
func (s *OpenAIGatewayService) tempUnscheduleOpenAITransportError(ctx context.Context, account *Account, safeErr string) {
	if s == nil || account == nil {
		return
	}
	until := time.Now().Add(openAITransportErrorTempUnschedDuration)
	reason := "upstream transport error (proxy/network): " + safeErr
	s.BlockAccountScheduling(account, until, "transport_error")

	log := logger.L().With(zap.String("component", "service.openai_gateway"))
	if s.accountRepo == nil {
		log.Warn("openai.account_temp_unscheduled_transport_memory_only",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.String("platform", account.Platform),
			zap.Time("until", until),
			zap.String("reason", reason),
		)
		return
	}

	bgCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	if err := s.accountRepo.SetTempUnschedulable(bgCtx, account.ID, until, reason); err != nil {
		log.Warn("openai.account_temp_unscheduled_transport_failed",
			zap.Int64("account_id", account.ID),
			zap.Error(err),
		)
		return
	}

	log.Warn("openai.account_temp_unscheduled_transport",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.String("platform", account.Platform),
		zap.Time("until", until),
		zap.String("reason", reason),
	)
}
