package service

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// newAPIRedeemWorkItem 是只会由一个账号 worker 消费的唯一兑换码任务。
type newAPIRedeemWorkItem struct {
	File newAPIRedeemStoredFile
	Code string
}

// newAPIRedeemWorkResult 把账号处理结果反馈给文件调度器，用于决定账号退出和兑换码优先回投。
type newAPIRedeemWorkResult struct {
	Item    newAPIRedeemWorkItem
	Outcome string
	Err     error
}

// newAPIRedeemRunNetwork 协调所有账号 worker 对同一 Mihomo 出口的访问。
// 普通请求持有读锁；首次观察到某一代出口限流的 worker 独占切换，其他 worker 复用新出口。
type newAPIRedeemRunNetwork struct {
	service        *NewAPIRedeemService
	runID          string
	gate           sync.RWMutex
	state          *newAPIRedeemNetworkState
	generation     uint64
	paceMu         sync.Mutex
	nextRequestAt  time.Time
	requestSpacing time.Duration
}

func newNewAPIRedeemRunNetwork(service *NewAPIRedeemService, runID string, accountCount int) *newAPIRedeemRunNetwork {
	return &newAPIRedeemRunNetwork{
		service: service, runID: runID, state: newNewAPIRedeemNetworkState(),
		requestSpacing: newAPIRedeemRequestSpacing(service.requestInterval, accountCount),
	}
}

// newAPIRedeemRequestSpacing 将账号并发均匀分布在单账号请求间隔内，不提高单账号请求频率。
func newAPIRedeemRequestSpacing(accountInterval time.Duration, accountCount int) time.Duration {
	if accountInterval <= 0 {
		return 0
	}
	if accountCount <= 1 {
		return accountInterval
	}
	return accountInterval / time.Duration(accountCount)
}

// waitRequestSlot 串行分配请求起始时刻，消除所有账号同一瞬间发出的突发流量。
func (network *newAPIRedeemRunNetwork) waitRequestSlot(ctx context.Context) bool {
	network.paceMu.Lock()
	defer network.paceMu.Unlock()
	if network.requestSpacing <= 0 {
		return true
	}
	if wait := time.Until(network.nextRequestAt); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
		}
	}
	network.nextRequestAt = time.Now().Add(network.requestSpacing)
	return true
}

// inspect 在 worker 启动前读取当前节点和出口 IP；失败只影响状态展示，不阻断普通请求。
func (network *newAPIRedeemRunNetwork) inspect(ctx context.Context) {
	if network.service.mihomoClient == nil {
		network.service.recordNewAPIRedeemNetwork(network.runID, newAPIRedeemNetworkIdentity{}, 0, "未配置 Mihomo Controller")
		return
	}
	identity, err := network.service.mihomoClient.inspect(ctx)
	if err != nil {
		network.service.recordNewAPIRedeemNetwork(network.runID, newAPIRedeemNetworkIdentity{}, 0, "读取当前节点和出口 IP 失败: "+err.Error())
		return
	}
	network.state.accept(identity)
	network.service.recordNewAPIRedeemNetwork(network.runID, identity, 0, "已读取当前节点和出口 IP")
}

// doRequest 把出口代次和请求时网络身份绑定到一次上游请求。
func (network *newAPIRedeemRunNetwork) doRequest(ctx context.Context, account newAPIRedeemStoredAccount, code string) (newAPIRedeemUpstreamResponse, error, uint64, newAPIRedeemNetworkIdentity) {
	if !network.waitRequestSlot(ctx) {
		return newAPIRedeemUpstreamResponse{}, ctx.Err(), 0, newAPIRedeemNetworkIdentity{}
	}
	network.gate.RLock()
	defer network.gate.RUnlock()
	generation := network.generation
	identity := network.state.Current
	result, err := network.service.callNewAPI(ctx, http.MethodPost, "/api/user/topup", account, map[string]any{"key": code})
	return result, err, generation, identity
}

// switchExit 只为仍处于 observedGeneration 的限流执行一次切换。
func (network *newAPIRedeemRunNetwork) switchExit(ctx context.Context, observedGeneration uint64) (*newAPIRedeemNetworkSwitch, error) {
	network.gate.Lock()
	defer network.gate.Unlock()
	if network.generation != observedGeneration {
		return nil, nil
	}
	if network.service.mihomoClient == nil {
		err := fmt.Errorf("未配置 Mihomo Controller")
		network.service.recordNewAPIRedeemNetwork(network.runID, network.state.Current, 0, err.Error())
		return nil, err
	}
	switched, err := network.service.mihomoClient.switchExit(ctx, network.state)
	if err != nil {
		network.service.recordNewAPIRedeemNetwork(network.runID, network.state.Current, 0, err.Error())
		return nil, err
	}
	network.generation++
	network.service.closeNewAPIRedeemAccountIdleConnections()
	network.service.recordNewAPIRedeemNetwork(network.runID, network.state.Current, 1, switched.Message)
	return &switched, nil
}

// executeNewAPIRedeemWorker 使用一个账号及其固定浏览器指纹顺序消费共享任务队列，并反馈每次处理结果。
func (s *NewAPIRedeemService) executeNewAPIRedeemWorker(ctx context.Context, runID string, account newAPIRedeemStoredAccount, jobs <-chan newAPIRedeemWorkItem, results chan<- newAPIRedeemWorkResult, network *newAPIRedeemRunNetwork) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-jobs:
			if !ok {
				return
			}
			if s.hasNewAPIRedeemSuccess(item.File.ID, account.ID) {
				return
			}
			outcome, err := s.executeNewAPIRedeemWorkItem(ctx, runID, account, item, network)
			select {
			case results <- newAPIRedeemWorkResult{Item: item, Outcome: outcome, Err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil || outcome == "redeemed" || outcome == "already_redeemed" {
				return
			}
		}
	}
}

// executeNewAPIRedeemWorkItem 完成一个码的验证；限流时切换出口并由同一账号重试原码。
func (s *NewAPIRedeemService) executeNewAPIRedeemWorkItem(ctx context.Context, runID string, account newAPIRedeemStoredAccount, item newAPIRedeemWorkItem, network *newAPIRedeemRunNetwork) (string, error) {
	for {
		if ctx.Err() != nil {
			return "", nil
		}
		s.recordNewAPIRedeemCurrentRequest(runID, item.File)
		result, requestErr, generation, requestIdentity := network.doRequest(ctx, account, item.Code)
		outcome := "request_error"
		statusCode := 0
		message := ""
		if requestErr != nil {
			message = requestErr.Error()
		} else {
			statusCode = result.StatusCode
			message = firstNewAPIRedeemText(result.Message, "上游拒绝请求")
			switch {
			case result.OK:
				outcome = "redeemed"
			case isNewAPIRedeemAlreadyRedeemed(result.Message):
				outcome = "already_redeemed"
			case isNewAPIRedeemRateLimited(result.StatusCode, result.Message):
				outcome = "rate_limited"
			case isNewAPIRedeemInvalidCode(result.Message):
				outcome = "invalid_code"
			default:
				outcome = "rejected"
			}
		}

		removedFromFile := false
		pendingRemovalCount := 0
		if outcome == "invalid_code" || outcome == "redeemed" {
			removed, pendingCount, removeErr := s.stageNewAPIRedeemCodeRemoval(item.File.ID, item.Code)
			if removeErr != nil {
				message = fmt.Sprintf("%s；从文件移除已用兑换码失败: %v", message, removeErr)
			} else {
				removedFromFile = removed
				pendingRemovalCount = pendingCount
			}
		}

		var switchResult *newAPIRedeemNetworkSwitch
		if outcome == "rate_limited" {
			switched, switchErr := network.switchExit(ctx, generation)
			if switchErr != nil {
				message = fmt.Sprintf("%s；自动切换节点失败: %v；任务将暂停后重试当前兑换码", message, switchErr)
				_ = s.recordNewAPIRedeemRequest(runID, item.File, account, item.Code, outcome, message, statusCode, false, requestIdentity, nil, "")
				return outcome, fmt.Errorf("%w: %v", errNewAPIRedeemNoAvailableExit, switchErr)
			}
			switchResult = switched
			if switched != nil {
				message = fmt.Sprintf("%s；%s", message, switched.Message)
			} else {
				message = message + "；已复用其他账号刚切换的新出口"
			}
		}

		completedMessage := ""
		if outcome != "rate_limited" && outcome != "already_redeemed" {
			completedMessage = fmt.Sprintf("已验证 %s", item.File.Name)
		}
		if err := s.recordNewAPIRedeemRequest(runID, item.File, account, item.Code, outcome, message, statusCode, removedFromFile, requestIdentity, switchResult, completedMessage); err != nil {
			return outcome, err
		}
		if outcome == "redeemed" && removedFromFile {
			// 真正兑换成功后立即压缩源文件，避免后续账号再次拿到已消费兑换码。
			if err := s.flushNewAPIRedeemPendingRemovals(item.File.ID); err != nil {
				return outcome, err
			}
		} else if pendingRemovalCount >= newAPIRedeemFileCompactBatch {
			if err := s.flushNewAPIRedeemPendingRemovals(item.File.ID); err != nil {
				return outcome, err
			}
		}
		if outcome == "rate_limited" {
			if !s.waitNewAPIRedeemInterval(ctx) {
				return outcome, nil
			}
			continue
		}
		if !s.waitNewAPIRedeemInterval(ctx) {
			return outcome, nil
		}
		return outcome, nil
	}
}
