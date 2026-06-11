package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	openAISchedulerExhaustionProbeAttemptsPerAccount = 6
	openAISchedulerExhaustionProbeLoopDelay          = 2 * time.Second
	openAISchedulerExhaustionProbeBodyReadLimit      = 1 << 20
)

// OpenAISchedulerExhaustionProbeOptions 描述调度耗尽后的小请求探测范围。
type OpenAISchedulerExhaustionProbeOptions struct {
	GroupID        *int64
	RequestedModel string
	RequireCompact bool
	Infinite       bool
}

// RecoverOpenAISchedulerExhaustion 在 OpenAI 调度池暂无可调度账号时，用最小
// Responses 请求主动探测同一调度范围内的账号。探测成功会解除运行时调度屏蔽并
// 清理 rate-limit/temp-unsched 等状态，使 handler 可以重新进入真实调度。
func (s *OpenAIGatewayService) RecoverOpenAISchedulerExhaustion(ctx context.Context, opts OpenAISchedulerExhaustionProbeOptions) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil {
		return false, errors.New("openai gateway service is nil")
	}
	accounts, err := s.listOpenAISchedulerExhaustionProbeCandidates(ctx, opts)
	if err != nil {
		return false, err
	}
	if len(accounts) == 0 {
		return false, ErrNoAvailableAccounts
	}

	startedAt := s.nowOpenAISchedulerExhaustionProbe()
	notifyEnabled, notifyAfter, notifyRepeat, notifyRecovered := s.openAISchedulerExhaustionProbeNotifySettings()
	nextNotifyAt := startedAt.Add(notifyAfter)
	waitingNotificationSent := false
	rounds := 0
	attempts := 0
	var lastErr error
	for {
		rounds++
		for i := range accounts {
			account := &accounts[i]
			attemptLimit := openAISchedulerExhaustionProbeAttemptsPerAccount
			if opts.Infinite {
				attemptLimit = 1
			}
			for attempt := 0; attempt < attemptLimit; attempt++ {
				if err := ctx.Err(); err != nil {
					return false, err
				}
				attempts++
				if err := s.probeOpenAISchedulerExhaustionAccount(ctx, account, opts.RequestedModel, opts.RequireCompact); err != nil {
					lastErr = err
					if opts.Infinite && notifyEnabled {
						now := s.nowOpenAISchedulerExhaustionProbe()
						if !now.Before(nextNotifyAt) {
							waitingNotificationSent = true
							s.notifyOpenAISchedulerExhaustionProbe(ctx, openAISchedulerExhaustionProbeNotifyEvent{
								Phase:          openAISchedulerExhaustionNotifyPhaseWaiting,
								StartedAt:      startedAt,
								Elapsed:        now.Sub(startedAt),
								Rounds:         rounds,
								Attempts:       attempts,
								CandidateCount: len(accounts),
								RequestedModel: opts.RequestedModel,
								RequireCompact: opts.RequireCompact,
								LastError:      err.Error(),
							})
							for !nextNotifyAt.After(now) {
								nextNotifyAt = nextNotifyAt.Add(notifyRepeat)
							}
						}
					}
					continue
				}
				if err := s.recoverOpenAISchedulerExhaustionAccount(ctx, account); err != nil {
					return false, err
				}
				if opts.Infinite && waitingNotificationSent && notifyRecovered {
					now := s.nowOpenAISchedulerExhaustionProbe()
					s.notifyOpenAISchedulerExhaustionProbe(ctx, openAISchedulerExhaustionProbeNotifyEvent{
						Phase:          openAISchedulerExhaustionNotifyPhaseRecovered,
						StartedAt:      startedAt,
						Elapsed:        now.Sub(startedAt),
						Rounds:         rounds,
						Attempts:       attempts,
						CandidateCount: len(accounts),
						RequestedModel: opts.RequestedModel,
						RequireCompact: opts.RequireCompact,
						AccountID:      account.ID,
						AccountName:    account.Name,
						LastError:      errorString(lastErr),
					})
				}
				return true, nil
			}
		}
		if !opts.Infinite {
			break
		}
		delay := roundDelayFromFailureCounts(s.collectOpenAISchedulerExhaustionFailureCounts(accounts))
		if err := s.sleepOpenAISchedulerExhaustionProbe(ctx, delay); err != nil {
			return false, err
		}
	}
	if lastErr != nil {
		return false, lastErr
	}
	return false, ErrNoAvailableAccounts
}

func (s *OpenAIGatewayService) listOpenAISchedulerExhaustionProbeCandidates(ctx context.Context, opts OpenAISchedulerExhaustionProbeOptions) ([]Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, errors.New("openai account repository is nil")
	}
	var accounts []Account
	var err error
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	} else if opts.GroupID != nil {
		accounts, err = s.accountRepo.ListByGroup(ctx, *opts.GroupID)
	} else {
		accounts, err = s.accountRepo.ListByGroup(ctx, AccountListGroupUngrouped)
	}
	if err != nil {
		return nil, fmt.Errorf("query openai scheduler probe candidates: %w", err)
	}

	candidates := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if isOpenAISchedulerExhaustionProbeCandidate(ctx, &account, opts.RequestedModel, opts.RequireCompact) {
			candidates = append(candidates, account)
		}
	}
	return candidates, nil
}

func isOpenAISchedulerExhaustionProbeCandidate(ctx context.Context, account *Account, requestedModel string, requireCompact bool) bool {
	if account == nil || !account.IsOpenAI() || !account.IsActive() || !account.Schedulable {
		return false
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !time.Now().Before(*account.ExpiresAt) {
		return false
	}
	if requestedModel != "" && !account.IsModelSupported(requestedModel) {
		return false
	}
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions) {
		return false
	}
	if requireCompact && openAICompactSupportTier(account) == 0 {
		return false
	}
	return true
}

func (s *OpenAIGatewayService) probeOpenAISchedulerExhaustionAccount(ctx context.Context, account *Account, requestedModel string, requireCompact bool) error {
	if s.openAISchedulerExhaustionProbeFunc != nil {
		return s.openAISchedulerExhaustionProbeFunc(ctx, account, requestedModel, requireCompact)
	}
	return s.sendOpenAISchedulerExhaustionProbe(ctx, account, requestedModel, requireCompact)
}

func (s *OpenAIGatewayService) sendOpenAISchedulerExhaustionProbe(ctx context.Context, account *Account, requestedModel string, requireCompact bool) error {
	if s == nil || s.httpUpstream == nil {
		return errors.New("openai scheduler exhaustion probe upstream is nil")
	}
	if account == nil {
		return errors.New("openai scheduler exhaustion probe account is nil")
	}

	probeCtx, cancel := context.WithTimeout(ctx, openaiResponsesProbeTimeout)
	defer cancel()

	token, _, err := s.GetAccessToken(probeCtx, account)
	if err != nil {
		return fmt.Errorf("get openai probe token for account %d: %w", account.ID, err)
	}

	probeModel := requestedModel
	if requireCompact {
		probeModel = resolveOpenAICompactForwardModel(account, requestedModel)
	}
	targetURL, err := s.openAISchedulerExhaustionProbeURL(account, requireCompact)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, targetURL, bytes.NewReader(openaiResponsesProbePayload(probeModel)))
	if err != nil {
		return fmt.Errorf("build openai scheduler exhaustion probe request: %w", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	if account.Type == AccountTypeOAuth {
		req.Host = "chatgpt.com"
		req.Header.Set("OpenAI-Beta", "responses=experimental")
		req.Header.Set("originator", "codex_cli_rs")
		req.Header.Set("user-agent", codexCLIUserAgent)
		req.Header.Set("version", codexCLIVersion)
		sessionID := fmt.Sprintf("sub2api-scheduler-probe-%d", account.ID)
		req.Header.Set("session_id", sessionID)
		req.Header.Set("conversation_id", sessionID)
		if chatgptAccountID := strings.TrimSpace(account.GetChatGPTAccountID()); chatgptAccountID != "" {
			req.Header.Set("chatgpt-account-id", chatgptAccountID)
		}
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	var resp *http.Response
	if profile := s.openAIUpstreamTLSProfile(account); profile != nil {
		resp, err = s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, profile)
	} else {
		resp, err = s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	}
	if err != nil {
		return fmt.Errorf("openai scheduler exhaustion probe request failed for account %d: %w", account.ID, err)
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, openAISchedulerExhaustionProbeBodyReadLimit))
	if readErr != nil {
		return fmt.Errorf("read openai scheduler exhaustion probe response for account %d: %w", account.ID, readErr)
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	s.handleOpenAIAccountUpstreamError(probeCtx, account, resp.StatusCode, resp.Header, responseBody, requestedModel)
	bodyText := strings.TrimSpace(truncateForLog(responseBody, 512))
	if bodyText == "" {
		return fmt.Errorf("openai scheduler exhaustion probe failed for account %d: status %d", account.ID, resp.StatusCode)
	}
	return fmt.Errorf("openai scheduler exhaustion probe failed for account %d: status %d body %s", account.ID, resp.StatusCode, bodyText)
}

func (s *OpenAIGatewayService) openAISchedulerExhaustionProbeURL(account *Account, requireCompact bool) (string, error) {
	targetURL := openaiPlatformAPIURL
	switch account.Type {
	case AccountTypeOAuth:
		targetURL = chatgptCodexURL
	case AccountTypeAPIKey:
		baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		validatedURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return "", err
		}
		targetURL = buildOpenAIAccountResponsesURL(validatedURL, account)
	default:
		return "", fmt.Errorf("unsupported openai probe account type: %s", account.Type)
	}
	if requireCompact {
		targetURL = appendOpenAIResponsesRequestPathSuffix(targetURL, "/compact")
	}
	return targetURL, nil
}

func (s *OpenAIGatewayService) recoverOpenAISchedulerExhaustionAccount(ctx context.Context, account *Account) error {
	if account == nil {
		return nil
	}
	s.ClearAccountSchedulingBlock(account.ID)
	if s.rateLimitService != nil {
		if _, err := s.rateLimitService.RecoverAccountAfterSuccessfulTest(ctx, account.ID); err != nil {
			return fmt.Errorf("recover openai scheduler probe account %d: %w", account.ID, err)
		}
	}
	if s.openaiPathHealth != nil {
		s.openaiPathHealth.RecordSuccess(OpenAIPathHealthKeyForAccount(account, string(OpenAIUpstreamTransportHTTPSSE)), nil, nil)
	}
	s.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
	return nil
}

func (s *OpenAIGatewayService) sleepOpenAISchedulerExhaustionProbe(ctx context.Context, d time.Duration) error {
	if s.openAISchedulerExhaustionProbeSleep != nil {
		return s.openAISchedulerExhaustionProbeSleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// roundDelayFromFailureCounts 取候选账号里最大的连续失败次数，按分级阶梯算出
// 本轮探测循环的等待时间。无候选时回退到基础轮询间隔。
func roundDelayFromFailureCounts(counts []int64) time.Duration {
	if len(counts) == 0 {
		return openAISchedulerExhaustionProbeLoopDelay
	}
	maxCount := int64(0)
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	return probeIntervalFromErrorCount(int(maxCount))
}

// collectOpenAISchedulerExhaustionFailureCounts 从候选账号的 path health 里收集
// ConsecutiveFailures 计数，供分级 sleep 使用。nil-safe。
func (s *OpenAIGatewayService) collectOpenAISchedulerExhaustionFailureCounts(accounts []Account) []int64 {
	if s == nil || s.openaiPathHealth == nil || len(accounts) == 0 {
		return nil
	}
	counts := make([]int64, 0, len(accounts))
	for i := range accounts {
		key := OpenAIPathHealthKeyForAccount(&accounts[i], string(OpenAIUpstreamTransportHTTPSSE))
		snapshot := s.openaiPathHealth.Snapshot(key)
		counts = append(counts, snapshot.ConsecutiveFailures)
	}
	return counts
}

func probeIntervalFromErrorCount(errorCount int) time.Duration {
	switch {
	case errorCount <= 1:
		return 1 * time.Second
	case errorCount == 2:
		return 3 * time.Second
	case errorCount == 3:
		return 10 * time.Second
	case errorCount == 4:
		return 30 * time.Second
	case errorCount == 5:
		return 1 * time.Minute
	case errorCount == 6:
		return 5 * time.Minute
	case errorCount == 7:
		return 30 * time.Minute
	default: // >= 8
		return 60 * time.Minute
	}
}

// ProbeIntervalFromErrorCount 按探测失败次数返回分级退避间隔，供 handler 层使用。
func (s *OpenAIGatewayService) ProbeIntervalFromErrorCount(errorCount int) time.Duration {
	return probeIntervalFromErrorCount(errorCount)
}
