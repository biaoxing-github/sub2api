package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// TempUnscheduler 用于 HandleFailoverError 中同账号重试耗尽后的临时封禁。
// GatewayService 隐式实现此接口。
type TempUnscheduler interface {
	TempUnscheduleRetryableError(ctx context.Context, accountID int64, failoverErr *service.UpstreamFailoverError)
}

// FailoverAction 表示 failover 错误处理后的下一步动作
type FailoverAction int

const (
	// FailoverContinue 继续循环（同账号重试或切换账号，调用方统一 continue）
	FailoverContinue FailoverAction = iota
	// FailoverExhausted 切换次数耗尽（调用方应返回错误响应）
	FailoverExhausted
	// FailoverCanceled context 已取消（调用方应直接 return）
	FailoverCanceled
)

const (
	// maxSameAccountRetries 同账号重试次数上限（针对 RetryableOnSameAccount 错误）
	maxSameAccountRetries = 3
	// sameAccountRetryDelay 同账号重试间隔
	sameAccountRetryDelay = 500 * time.Millisecond
	// singleAccountBackoffDelay 单账号分组 503 退避重试固定延时。
	// Service 层在 SingleAccountRetry 模式下已做充分原地重试（最多 3 次、总等待 30s），
	// Handler 层只需短暂间隔后重新进入 Service 层即可。
	singleAccountBackoffDelay = 2 * time.Second
)

// FailoverState 跨循环迭代共享的 failover 状态
type FailoverState struct {
	SwitchCount           int
	MaxSwitches           int
	FailedAccountIDs      map[int64]struct{}
	SameAccountRetryCount map[int64]int
	LastFailoverErr       *service.UpstreamFailoverError
	ForceCacheBilling     bool
	hasBoundSession       bool
}

// NewFailoverState 创建 failover 状态
func NewFailoverState(maxSwitches int, hasBoundSession bool) *FailoverState {
	return &FailoverState{
		MaxSwitches:           maxSwitches,
		FailedAccountIDs:      make(map[int64]struct{}),
		SameAccountRetryCount: make(map[int64]int),
		hasBoundSession:       hasBoundSession,
	}
}

// HandleFailoverError 处理 UpstreamFailoverError，返回下一步动作。
// 包含：缓存计费判断、智能重试策略、临时封禁、切换计数、Antigravity 延时。
func (s *FailoverState) HandleFailoverError(
	ctx context.Context,
	gatewayService TempUnscheduler,
	accountID int64,
	platform string,
	failoverErr *service.UpstreamFailoverError,
) FailoverAction {
	s.LastFailoverErr = failoverErr

	// 缓存计费判断
	if needForceCacheBilling(s.hasBoundSession, failoverErr) {
		s.ForceCacheBilling = true
	}

	// 智能重试策略：根据状态码差异化处理
	statusCode := failoverErr.StatusCode
	switch statusCode {
	case http.StatusTooManyRequests: // 429 限流
		// 429 限流：同账号重试，等待 RetryAfter 或固定延时
		if s.SameAccountRetryCount[accountID] < maxSameAccountRetries {
			s.SameAccountRetryCount[accountID]++
			logger.FromContext(ctx).Warn("gateway.failover_429_same_account_retry",
				zap.Int64("account_id", accountID),
				zap.Int("same_account_retry_count", s.SameAccountRetryCount[accountID]),
				zap.Int("same_account_retry_max", maxSameAccountRetries),
			)
			if !sleepWithContext(ctx, sameAccountRetryDelay) {
				return FailoverCanceled
			}
			return FailoverContinue
		}
		// 同账号重试用尽，执行临时封禁后切换
		gatewayService.TempUnscheduleRetryableError(ctx, accountID, failoverErr)

	case http.StatusServiceUnavailable: // 503 容量不足
		// 503 容量不足：立即切换下一账号，不冷却当前账号
		logger.FromContext(ctx).Warn("gateway.failover_503_immediate_switch",
			zap.Int64("account_id", accountID),
		)
		// 不执行 TempUnscheduleRetryableError，直接加入失败列表

	case http.StatusBadGateway, http.StatusGatewayTimeout: // 502/504 网关错误
		// 502/504 网关错误：同账号重试 1 次，等待 1s
		if s.SameAccountRetryCount[accountID] < 1 {
			s.SameAccountRetryCount[accountID]++
			logger.FromContext(ctx).Warn("gateway.failover_5xx_gateway_retry",
				zap.Int64("account_id", accountID),
				zap.Int("upstream_status", statusCode),
				zap.Int("same_account_retry_count", s.SameAccountRetryCount[accountID]),
			)
			if !sleepWithContext(ctx, time.Second) {
				return FailoverCanceled
			}
			return FailoverContinue
		}
		// 重试用尽，执行临时封禁后切换
		gatewayService.TempUnscheduleRetryableError(ctx, accountID, failoverErr)

	default:
		// 其他错误：原有逻辑，RetryableOnSameAccount 同账号重试
		if failoverErr.RetryableOnSameAccount && s.SameAccountRetryCount[accountID] < maxSameAccountRetries {
			s.SameAccountRetryCount[accountID]++
			logger.FromContext(ctx).Warn("gateway.failover_same_account_retry",
				zap.Int64("account_id", accountID),
				zap.Int("upstream_status", statusCode),
				zap.Int("same_account_retry_count", s.SameAccountRetryCount[accountID]),
				zap.Int("same_account_retry_max", maxSameAccountRetries),
			)
			if !sleepWithContext(ctx, sameAccountRetryDelay) {
				return FailoverCanceled
			}
			return FailoverContinue
		}
		// 同账号重试用尽，执行临时封禁
		if failoverErr.RetryableOnSameAccount {
			gatewayService.TempUnscheduleRetryableError(ctx, accountID, failoverErr)
		}
	}

	// 加入失败列表
	s.FailedAccountIDs[accountID] = struct{}{}

	// 检查是否耗尽
	if s.SwitchCount >= s.MaxSwitches {
		return FailoverExhausted
	}

	// 递增切换计数
	s.SwitchCount++
	logger.FromContext(ctx).Warn("gateway.failover_switch_account",
		zap.Int64("account_id", accountID),
		zap.Int("upstream_status", statusCode),
		zap.Int("switch_count", s.SwitchCount),
		zap.Int("max_switches", s.MaxSwitches),
	)

	// Antigravity 平台换号线性递增延时
	if platform == service.PlatformAntigravity {
		delay := time.Duration(s.SwitchCount-1) * time.Second
		if !sleepWithContext(ctx, delay) {
			return FailoverCanceled
		}
	}

	return FailoverContinue
}

// HandleSelectionExhausted 处理选号失败（所有候选账号都在排除列表中）时的退避重试决策。
// 针对 Anthropic 单账号分组的 503 (MODEL_CAPACITY_EXHAUSTED) 场景：
// infiniteWait=true 时无限等待重试，否则仍有切换额度时清除排除列表、等待退避后重新选号。
//
// 返回 FailoverContinue 时，调用方应设置 SingleAccountRetry context 并 continue。
// 返回 FailoverExhausted 时，调用方应返回错误响应。
// 返回 FailoverCanceled 时，调用方应直接 return。
func (s *FailoverState) HandleSelectionExhausted(ctx context.Context, infiniteWait bool) FailoverAction {
	if infiniteWait || (s.LastFailoverErr != nil &&
		s.LastFailoverErr.StatusCode == http.StatusServiceUnavailable &&
		s.SwitchCount < s.MaxSwitches) {

		logger.FromContext(ctx).Warn("gateway.failover_single_account_backoff",
			zap.Duration("backoff_delay", singleAccountBackoffDelay),
			zap.Int("switch_count", s.SwitchCount),
			zap.Int("max_switches", s.MaxSwitches),
			zap.Bool("infinite_wait", infiniteWait),
		)
		if !sleepWithContext(ctx, singleAccountBackoffDelay) {
			return FailoverCanceled
		}
		logger.FromContext(ctx).Warn("gateway.failover_single_account_retry",
			zap.Int("switch_count", s.SwitchCount),
			zap.Int("max_switches", s.MaxSwitches),
			zap.Bool("infinite_wait", infiniteWait),
		)
		s.FailedAccountIDs = make(map[int64]struct{})
		return FailoverContinue
	}
	return FailoverExhausted
}

// needForceCacheBilling 判断 failover 时是否需要强制缓存计费。
// 粘性会话切换账号、或上游明确标记时，将 input_tokens 转为 cache_read 计费。
func needForceCacheBilling(hasBoundSession bool, failoverErr *service.UpstreamFailoverError) bool {
	return hasBoundSession || (failoverErr != nil && failoverErr.ForceCacheBilling)
}

// sleepWithContext 等待指定时长，返回 false 表示 context 已取消。
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
