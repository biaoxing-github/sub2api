package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// OpenAIAccountSchedulingPoolStatusSchedulable 表示账号当前可被调度器正常选择。
	OpenAIAccountSchedulingPoolStatusSchedulable = "schedulable"
	// OpenAIAccountSchedulingPoolStatusDegraded 表示账号仍在池内，但线路健康已降级。
	OpenAIAccountSchedulingPoolStatusDegraded = "degraded"
	// OpenAIAccountSchedulingPoolStatusBlocked 表示账号被运行时阻断或熔断器拦截。
	OpenAIAccountSchedulingPoolStatusBlocked = "blocked"
	// OpenAIAccountSchedulingPoolStatusFiltered 表示账号不满足当前模型、接口或能力筛选。
	OpenAIAccountSchedulingPoolStatusFiltered = "filtered"
)

// OpenAIAccountSchedulingPoolFilter 描述管理端查看调度池时使用的筛选条件。
type OpenAIAccountSchedulingPoolFilter struct {
	GroupID         *int64
	Model           string
	Endpoint        OpenAIEndpointCapability
	Transport       OpenAIUpstreamTransport
	ImageCapability OpenAIImagesCapability
	Search          string
}

// OpenAIAccountSchedulingPoolSnapshot 表示某一时刻调度池中账号的健康快照。
type OpenAIAccountSchedulingPoolSnapshot struct {
	Items            []OpenAIAccountSchedulingPoolItem `json:"items"`
	Total            int                               `json:"total"`
	SchedulableCount int                               `json:"schedulable_count"`
	DegradedCount    int                               `json:"degraded_count"`
	BlockedCount     int                               `json:"blocked_count"`
	FilteredCount    int                               `json:"filtered_count"`
	GeneratedAt      time.Time                         `json:"generated_at"`
	GroupID          *int64                            `json:"group_id,omitempty"`
	Model            string                            `json:"model,omitempty"`
	Endpoint         string                            `json:"endpoint,omitempty"`
	Transport        string                            `json:"transport,omitempty"`
	ImageCapability  string                            `json:"image_capability,omitempty"`
}

// OpenAIAccountSchedulingPoolItem 表示单个账号在当前筛选条件下的调度状态。
type OpenAIAccountSchedulingPoolItem struct {
	Account             Account                            `json:"account"`
	PoolStatus          string                             `json:"pool_status"`
	PoolReasons         []string                           `json:"pool_reasons"`
	RuntimeBlock        *OpenAIAccountRuntimeBlockSnapshot `json:"runtime_block,omitempty"`
	PathHealth          OpenAIPathHealthRecord             `json:"path_health"`
	PathHealthAvailable bool                               `json:"path_health_available"`
	DerivedHealth       AccountDerivedHealthState          `json:"derived_health"`
	EffectiveLoadFactor int                                `json:"effective_load_factor"`
}

// ListOpenAIAccountSchedulingPool 返回 OpenAI 调度热路径看到的账号池快照。
func (s *OpenAIGatewayService) ListOpenAIAccountSchedulingPool(ctx context.Context, filter OpenAIAccountSchedulingPoolFilter, now time.Time) (OpenAIAccountSchedulingPoolSnapshot, error) {
	if now.IsZero() {
		now = time.Now()
	}
	filter = normalizeOpenAIAccountSchedulingPoolFilter(filter)
	accounts, err := s.listSchedulableAccounts(ctx, filter.GroupID)
	if err != nil {
		return OpenAIAccountSchedulingPoolSnapshot{}, err
	}

	items := make([]OpenAIAccountSchedulingPoolItem, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		if !account.IsOpenAI() || !account.IsSchedulableAt(now) {
			continue
		}
		if !openAIAccountSchedulingPoolMatchesSearch(&account, filter.Search) {
			continue
		}
		items = append(items, s.buildOpenAIAccountSchedulingPoolItem(account, filter, now))
	}
	sort.SliceStable(items, func(i, j int) bool {
		leftRank := openAIAccountSchedulingPoolStatusRank(items[i].PoolStatus)
		rightRank := openAIAccountSchedulingPoolStatusRank(items[j].PoolStatus)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if items[i].Account.Priority != items[j].Account.Priority {
			return items[i].Account.Priority > items[j].Account.Priority
		}
		return items[i].Account.ID < items[j].Account.ID
	})

	snapshot := OpenAIAccountSchedulingPoolSnapshot{
		Items:           items,
		Total:           len(items),
		GeneratedAt:     now,
		GroupID:         cloneInt64Ptr(filter.GroupID),
		Model:           filter.Model,
		Endpoint:        string(filter.Endpoint),
		Transport:       string(filter.Transport),
		ImageCapability: string(filter.ImageCapability),
	}
	for _, item := range items {
		switch item.PoolStatus {
		case OpenAIAccountSchedulingPoolStatusSchedulable:
			snapshot.SchedulableCount++
		case OpenAIAccountSchedulingPoolStatusDegraded:
			snapshot.DegradedCount++
		case OpenAIAccountSchedulingPoolStatusBlocked:
			snapshot.BlockedCount++
		case OpenAIAccountSchedulingPoolStatusFiltered:
			snapshot.FilteredCount++
		}
	}
	return snapshot, nil
}

func normalizeOpenAIAccountSchedulingPoolFilter(filter OpenAIAccountSchedulingPoolFilter) OpenAIAccountSchedulingPoolFilter {
	filter.Model = strings.TrimSpace(filter.Model)
	filter.Search = strings.ToLower(strings.TrimSpace(filter.Search))
	if filter.Transport == "" {
		filter.Transport = OpenAIUpstreamTransportHTTPSSE
	}
	return filter
}

func (s *OpenAIGatewayService) buildOpenAIAccountSchedulingPoolItem(account Account, filter OpenAIAccountSchedulingPoolFilter, now time.Time) OpenAIAccountSchedulingPoolItem {
	reasons := make([]string, 0, 4)
	status := OpenAIAccountSchedulingPoolStatusSchedulable

	runtimeBlock, runtimeBlocked := s.SnapshotOpenAIAccountRuntimeBlock(&account, now)
	if runtimeBlocked {
		status = OpenAIAccountSchedulingPoolStatusBlocked
		reasons = append(reasons, "runtime_block:"+firstNonEmptyString(runtimeBlock.Reason, "active"))
	}
	if account.IsOpenAIApiKey() && len(account.GetAPIKeys()) == 0 {
		status = OpenAIAccountSchedulingPoolStatusBlocked
		reasons = append(reasons, "api_key_missing")
	}

	filtered := false
	if filter.Model != "" && !account.IsModelSupported(filter.Model) {
		filtered = true
		reasons = append(reasons, "model_unsupported:"+filter.Model)
	}
	if !account.SupportsOpenAIEndpointCapability(filter.Endpoint) {
		filtered = true
		reasons = append(reasons, "endpoint_unsupported:"+string(filter.Endpoint))
	}
	if !s.isOpenAIAccountTransportCompatible(&account, filter.Transport) {
		filtered = true
		reasons = append(reasons, "transport_unsupported:"+string(filter.Transport))
	}
	if !account.SupportsOpenAIImageCapability(filter.ImageCapability) {
		filtered = true
		reasons = append(reasons, "image_capability_unsupported:"+string(filter.ImageCapability))
	}
	if filtered && status != OpenAIAccountSchedulingPoolStatusBlocked {
		status = OpenAIAccountSchedulingPoolStatusFiltered
	}

	pathHealth, pathHealthAvailable := s.snapshotOpenAIPathHealthForSchedulingPool(&account, filter.Transport)
	pathReason := openAIAccountSchedulingPoolPathReason(pathHealth)
	if pathHealth.State == OpenAIPathHealthStateDegraded {
		reasons = append(reasons, pathReason)
		if status == OpenAIAccountSchedulingPoolStatusSchedulable {
			status = OpenAIAccountSchedulingPoolStatusDegraded
		}
	}
	if s.openAIPathHealthCircuitBreakerEnabled() &&
		(pathHealth.State == OpenAIPathHealthStateOpenCircuit || pathHealth.State == OpenAIPathHealthStateHalfOpen) {
		status = OpenAIAccountSchedulingPoolStatusBlocked
		reasons = append(reasons, pathReason)
	}

	var blockPtr *OpenAIAccountRuntimeBlockSnapshot
	if runtimeBlocked {
		snapshot := runtimeBlock
		blockPtr = &snapshot
	}
	return OpenAIAccountSchedulingPoolItem{
		Account:             account,
		PoolStatus:          status,
		PoolReasons:         uniqueNonEmptyStrings(reasons),
		RuntimeBlock:        blockPtr,
		PathHealth:          pathHealth,
		PathHealthAvailable: pathHealthAvailable,
		DerivedHealth:       DeriveAccountHealthState(&account, pathHealth, now),
		EffectiveLoadFactor: account.EffectiveLoadFactor(),
	}
}

func (s *OpenAIGatewayService) snapshotOpenAIPathHealthForSchedulingPool(account *Account, transport OpenAIUpstreamTransport) (OpenAIPathHealthRecord, bool) {
	if s == nil || s.openaiPathHealth == nil || account == nil {
		return OpenAIPathHealthRecord{Key: OpenAIPathHealthKeyForAccount(account, string(transport)), State: OpenAIPathHealthStateHealthy}, false
	}
	accountKey := OpenAIPathHealthKeyForAccount(account, string(transport))
	accountSnapshot := s.openaiPathHealth.Snapshot(accountKey)
	bucketSnapshot := s.openaiPathHealth.Snapshot(OpenAIPathHealthBucketKeyForAccount(account, string(transport)))
	worseState := openAIPathHealthWorseState(accountSnapshot.State, bucketSnapshot.State)
	if worseState == accountSnapshot.State {
		return accountSnapshot, true
	}
	accountSnapshot.State = worseState
	if accountSnapshot.LastFailureReason == "" {
		accountSnapshot.LastFailureReason = bucketSnapshot.LastFailureReason
	}
	if accountSnapshot.CooldownUntil == nil || (bucketSnapshot.CooldownUntil != nil && bucketSnapshot.CooldownUntil.After(*accountSnapshot.CooldownUntil)) {
		accountSnapshot.CooldownUntil = bucketSnapshot.CooldownUntil
	}
	return accountSnapshot, true
}

func openAIAccountSchedulingPoolPathReason(health OpenAIPathHealthRecord) string {
	state := strings.TrimSpace(health.State)
	if state == "" {
		state = OpenAIPathHealthStateHealthy
	}
	reason := "path_health:" + state
	if health.LastFailureReason != "" {
		reason += ":" + strings.TrimSpace(health.LastFailureReason)
	}
	return reason
}

func openAIAccountSchedulingPoolMatchesSearch(account *Account, search string) bool {
	if search == "" {
		return true
	}
	if account == nil {
		return false
	}
	parts := []string{
		strconv.FormatInt(account.ID, 10),
		account.Name,
		account.Type,
		account.Status,
		account.ErrorMessage,
	}
	for _, group := range account.Groups {
		if group != nil {
			parts = append(parts, group.Name, strconv.FormatInt(group.ID, 10))
		}
	}
	for _, accountGroup := range account.AccountGroups {
		parts = append(parts, strconv.FormatInt(accountGroup.GroupID, 10))
	}
	for _, part := range parts {
		if strings.Contains(strings.ToLower(strings.TrimSpace(part)), search) {
			return true
		}
	}
	return false
}

func openAIAccountSchedulingPoolStatusRank(status string) int {
	switch status {
	case OpenAIAccountSchedulingPoolStatusBlocked:
		return 0
	case OpenAIAccountSchedulingPoolStatusFiltered:
		return 1
	case OpenAIAccountSchedulingPoolStatusDegraded:
		return 2
	case OpenAIAccountSchedulingPoolStatusSchedulable:
		return 3
	default:
		return 4
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
