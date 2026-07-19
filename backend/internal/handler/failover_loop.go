package handler

import (
	"context"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// TempUnscheduler 用于 HandleFailoverError 中异常账号重试/切换前的临时冷却。
// GatewayService 隐式实现此接口。
type TempUnscheduler interface {
	TempUnscheduleRetryableError(ctx context.Context, accountID int64, failoverErr *service.UpstreamFailoverError)
}

// gatewayServiceKey 用于在 context 中传递 GatewayService，供 HandleSelectionExhausted 检查账号封禁状态
type gatewayServiceKey struct{}

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
	// maxSameAccountRetries 同账号重试次数上限（针对 RetryableOnSameAccount 错误）。
	// 首次请求之外只允许 1 次同账号重试，配合 30 秒间隔保证每分钟最多两次。
	maxSameAccountRetries = 1
	// sameAccountRetryDelay 同账号重试间隔。异常账号探测/重试必须保持 30 秒下限，
	// 确保单账号每分钟最多产生两次上游请求。
	sameAccountRetryDelay = 30 * time.Second
	// defaultSingleAccountBackoffDelay 单账号分组 503 退避重试默认延时。
	// 该值同时作为 Anthropic/Gemini 单账号容量耗尽后的最小重试间隔。
	defaultSingleAccountBackoffDelay = 30 * time.Second
	// maxSingleAccountExhaustionBackoffDelay 限制默认退避增长上界，避免单个请求长时间静默。
	maxSingleAccountExhaustionBackoffDelay = 30 * time.Second
)

var failoverSleepWithContext = defaultFailoverSleepWithContext

// FailoverState 跨循环迭代共享的 failover 状态
type FailoverState struct {
	SwitchCount                 int
	MaxSwitches                 int
	FailedAccountIDs            map[int64]struct{}
	SameAccountRetryCount       map[int64]int
	LastFailoverErr             *service.UpstreamFailoverError
	ForceCacheBilling           bool
	hasBoundSession             bool
	singleAccountBackoffTime    time.Duration // 单账号分组初始退避时间
	singleAccountBackoffAttempt int           // 当前请求内选号耗尽后已执行的退避次数
}

// NewFailoverState 创建 failover 状态
func NewFailoverState(maxSwitches int, hasBoundSession bool) *FailoverState {
	return &FailoverState{
		MaxSwitches:              maxSwitches,
		FailedAccountIDs:         make(map[int64]struct{}),
		SameAccountRetryCount:    make(map[int64]int),
		hasBoundSession:          hasBoundSession,
		singleAccountBackoffTime: defaultSingleAccountBackoffDelay,
	}
}

// NewFailoverStateWithBackoff 创建带自定义初始退避时间的 failover 状态
func NewFailoverStateWithBackoff(maxSwitches int, hasBoundSession bool, backoffSeconds int) *FailoverState {
	backoffTime := defaultSingleAccountBackoffDelay
	if backoffSeconds > 0 {
		backoffTime = time.Duration(backoffSeconds) * time.Second
		if backoffTime < defaultSingleAccountBackoffDelay {
			backoffTime = defaultSingleAccountBackoffDelay
		}
	}
	return &FailoverState{
		MaxSwitches:              maxSwitches,
		FailedAccountIDs:         make(map[int64]struct{}),
		SameAccountRetryCount:    make(map[int64]int),
		hasBoundSession:          hasBoundSession,
		singleAccountBackoffTime: backoffTime,
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
	// 请求已取消后继续重试只会把 context canceled 误报为账号耗尽。
	if ctx != nil && ctx.Err() != nil {
		return FailoverCanceled
	}
	s.LastFailoverErr = failoverErr

	// 缓存计费判断
	if needForceCacheBilling(s.hasBoundSession, failoverErr) {
		s.ForceCacheBilling = true
	}

	// 智能重试策略：根据状态码和统一错误分类差异化处理。
	statusCode := failoverErr.StatusCode
	switch {
	case platform == service.PlatformOpenAI && isOpenAIAccountInfrastructureFailover(failoverErr):
		// 账号级基础设施故障已经由 service 写入冷却；handler 只负责立即切号，不能再落入 502/504 同账号重试。
		tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)
		logger.FromContext(ctx).Warn("gateway.failover_openai_infrastructure_immediate_switch",
			zap.Int64("account_id", accountID),
			zap.Int("upstream_status", statusCode),
			zap.String("error_category", failoverErr.ActionMetadata["error_category"]),
		)

	case statusCode == http.StatusTooManyRequests: // 429 限流
		// 429 限流：同账号重试，等待 RetryAfter 或固定延时
		if s.SameAccountRetryCount[accountID] < maxSameAccountRetries {
			s.SameAccountRetryCount[accountID]++
			tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)
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
		tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)

	case statusCode == http.StatusServiceUnavailable: // 503 容量不足
		// 503 容量不足：立即切换下一账号，短暂冷却避免循环选中
		logger.FromContext(ctx).Warn("gateway.failover_503_immediate_switch",
			zap.Int64("account_id", accountID),
		)
		// 执行短时临时封禁，避免 HandleSelectionExhausted 清空失败列表后再次选中
		tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)

	case statusCode == 529: // 529 过载
		// 529 过载：立即切换下一账号，30s 冷却
		logger.FromContext(ctx).Warn("gateway.failover_529_immediate_switch",
			zap.Int64("account_id", accountID),
		)
		// 执行 30s 临时封禁
		tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)

	case statusCode == http.StatusBadGateway || statusCode == http.StatusGatewayTimeout: // 502/504 网关错误
		// 502/504 网关错误：同账号重试 1 次，等待 30s，避免异常账号快速探测。
		if s.SameAccountRetryCount[accountID] < 1 {
			s.SameAccountRetryCount[accountID]++
			tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)
			logger.FromContext(ctx).Warn("gateway.failover_5xx_gateway_retry",
				zap.Int64("account_id", accountID),
				zap.Int("upstream_status", statusCode),
				zap.Int("same_account_retry_count", s.SameAccountRetryCount[accountID]),
			)
			if !sleepWithContext(ctx, sameAccountRetryDelay) {
				return FailoverCanceled
			}
			return FailoverContinue
		}
		// 重试用尽，执行临时封禁后切换
		tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)

	default:
		// 其他错误：原有逻辑，RetryableOnSameAccount 同账号重试
		if failoverErr.RetryableOnSameAccount && s.SameAccountRetryCount[accountID] < maxSameAccountRetries {
			s.SameAccountRetryCount[accountID]++
			tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)
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
			tempUnscheduleFailoverAccount(ctx, gatewayService, accountID, failoverErr)
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
	service.RecordOpsSchedulingEvent(service.OpsSchedulingEventFailover)
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

// isOpenAIAccountInfrastructureFailover 使用统一分类元数据识别必须立即避让整个账号的上游故障。
func isOpenAIAccountInfrastructureFailover(failoverErr *service.UpstreamFailoverError) bool {
	if failoverErr == nil {
		return false
	}
	if service.IsOpenAIUpstreamInfrastructureFailure(failoverErr.StatusCode, "", failoverErr.ResponseBody) {
		return true
	}
	if failoverErr.StatusCode == 529 {
		return false
	}
	// 流内错误可能没有独立 HTTP 状态或完整响应体，保留统一策略元数据作为同一分类器的结果载体。
	category := failoverErr.ActionMetadata["error_category"]
	return category == service.UpstreamErrorCategoryInfrastructureFailure || category == service.UpstreamErrorCategoryUpstream5xx
}

func tempUnscheduleFailoverAccount(ctx context.Context, gatewayService TempUnscheduler, accountID int64, failoverErr *service.UpstreamFailoverError) {
	if gatewayService == nil || failoverErr == nil {
		return
	}
	gatewayService.TempUnscheduleRetryableError(ctx, accountID, failoverErr)
}

// HandleSelectionExhausted 处理选号失败（所有候选账号都在排除列表中）时的退避重试决策。
// 针对 Anthropic 单账号分组的 503 (MODEL_CAPACITY_EXHAUSTED) 场景：
// infiniteWait=true 时无限等待重试，否则仍有切换额度时清除排除列表、等待退避后重新选号。
//
// 返回 FailoverContinue 时，调用方应设置 SingleAccountRetry context 并 continue。
// 返回 FailoverExhausted 时，调用方应返回错误响应。
// 返回 FailoverCanceled 时，调用方应直接 return。
func (s *FailoverState) HandleSelectionExhausted(ctx context.Context, infiniteWait bool) FailoverAction {
	if ctx.Err() != nil {
		return FailoverCanceled
	}

	// 修复：退避前检查唯一候选账号是否仍在熔断期，避免空转
	if !infiniteWait && len(s.FailedAccountIDs) == 1 {
		for accountID := range s.FailedAccountIDs {
			// 如果唯一账号仍在封禁期（429/503），立即失败，不空转
			if gatewayService, ok := ctx.Value(gatewayServiceKey{}).(interface {
				IsAccountBlocked(int64) bool
			}); ok && gatewayService.IsAccountBlocked(accountID) {
				logger.FromContext(ctx).Warn("gateway.failover_blocked_account_skip",
					zap.Int64("account_id", accountID),
					zap.String("reason", "account still in cooldown"),
				)
				return FailoverExhausted
			}
		}
	}

	if infiniteWait || (s.LastFailoverErr != nil &&
		service.IsRetryableSchedulerExhaustionStatus(s.LastFailoverErr.StatusCode) &&
		s.SwitchCount < s.MaxSwitches) {

		backoffDelay := singleAccountExhaustionBackoffDelay(s.singleAccountBackoffTime, s.singleAccountBackoffAttempt)
		logger.FromContext(ctx).Warn("gateway.failover_single_account_backoff",
			zap.Duration("backoff_delay", backoffDelay),
			zap.Int("single_account_backoff_attempt", s.singleAccountBackoffAttempt),
			zap.Int("switch_count", s.SwitchCount),
			zap.Int("max_switches", s.MaxSwitches),
			zap.Bool("infinite_wait", infiniteWait),
		)
		if !sleepWithContext(ctx, backoffDelay) {
			return FailoverCanceled
		}
		s.singleAccountBackoffAttempt++
		logger.FromContext(ctx).Warn("gateway.failover_single_account_retry",
			zap.Int("single_account_backoff_attempt", s.singleAccountBackoffAttempt),
			zap.Int("switch_count", s.SwitchCount),
			zap.Int("max_switches", s.MaxSwitches),
			zap.Bool("infinite_wait", infiniteWait),
		)
		s.FailedAccountIDs = make(map[int64]struct{})
		return FailoverContinue
	}
	return FailoverExhausted
}

// singleAccountExhaustionBackoffDelay 计算选号耗尽后的退避延迟。
// 30 秒是异常账号探测下限；若配置值本身高于 30 秒，则尊重配置值不再压低。
func singleAccountExhaustionBackoffDelay(base time.Duration, retryCount int) time.Duration {
	if base <= 0 || base < defaultSingleAccountBackoffDelay {
		base = defaultSingleAccountBackoffDelay
	}
	maxDelay := maxSingleAccountExhaustionBackoffDelay
	if base > maxDelay {
		maxDelay = base
	}
	delay := base
	for i := 0; i < retryCount && delay < maxDelay; i++ {
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	jitterMax := delay * 3 / 10
	if jitterMax > 0 && delay < maxDelay {
		delay += time.Duration(rand.Int64N(int64(jitterMax) + 1))
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	return delay
}

// needForceCacheBilling 判断 failover 时是否需要强制缓存计费。
// 粘性会话切换账号、或上游明确标记时，将 input_tokens 转为 cache_read 计费。
func needForceCacheBilling(hasBoundSession bool, failoverErr *service.UpstreamFailoverError) bool {
	return hasBoundSession || (failoverErr != nil && failoverErr.ForceCacheBilling)
}

// failoverClientGone 在客户端断开后停止新的选号和上游重试。
func failoverClientGone(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.Context().Err() == nil {
		return false
	}
	if service.StopOpenAICompactSSEKeepaliveCommitted(c) {
		return true
	}
	if !c.Writer.Written() {
		c.Status(statusClientClosedRequest)
	}
	return true
}

// sleepWithContext 等待指定时长，返回 false 表示 context 已取消。
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	return failoverSleepWithContext(ctx, d)
}

func sameAccountRetryLimit(retryLimit int) int {
	if retryLimit <= 0 {
		return 0
	}
	if retryLimit > maxSameAccountRetries {
		return maxSameAccountRetries
	}
	return retryLimit
}

func defaultFailoverSleepWithContext(ctx context.Context, d time.Duration) bool {
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
