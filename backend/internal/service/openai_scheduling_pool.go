package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const openAIAccountSchedulingPoolPageSize = 1000

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
	Platform        string
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
	Platform         string                            `json:"platform,omitempty"`
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
	NextScheduledAt     *time.Time                         `json:"next_scheduled_at,omitempty"`
	NextScheduledReason string                             `json:"next_scheduled_reason,omitempty"`
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
	accounts, useMixed, err := s.listSchedulingPoolAccounts(ctx, filter.GroupID, filter.Platform)
	if err != nil {
		return OpenAIAccountSchedulingPoolSnapshot{}, err
	}

	items := make([]OpenAIAccountSchedulingPoolItem, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		if !openAIAccountSchedulingPoolPlatformAllowed(&account, filter.Platform, useMixed) {
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
			return items[i].Account.Priority < items[j].Account.Priority
		}
		return items[i].Account.ID < items[j].Account.ID
	})

	snapshot := OpenAIAccountSchedulingPoolSnapshot{
		Items:           items,
		Total:           len(items),
		GeneratedAt:     now,
		GroupID:         cloneInt64Ptr(filter.GroupID),
		Platform:        filter.Platform,
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
	filter.Platform = strings.ToLower(strings.TrimSpace(filter.Platform))
	if filter.Platform == "" {
		filter.Platform = PlatformOpenAI
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.Search = strings.ToLower(strings.TrimSpace(filter.Search))
	if filter.Transport == "" {
		filter.Transport = OpenAIUpstreamTransportHTTPSSE
	}
	return filter
}

func (s *OpenAIGatewayService) listSchedulingPoolAccounts(ctx context.Context, groupID *int64, platform string) ([]Account, bool, error) {
	if s == nil || s.accountRepo == nil {
		return nil, false, fmt.Errorf("account repository is not configured")
	}

	useMixed := platform == PlatformAnthropic
	platforms := []string{platform}
	switch platform {
	case PlatformOpenAI:
		platforms = openAICompatibleAccountPlatforms(platform)
	case PlatformAnthropic:
		platforms = []string{platform, PlatformAntigravity}
	}

	groupFilter := AccountListGroupUngrouped
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		groupFilter = 0
	} else if groupID != nil {
		groupFilter = *groupID
	}

	accounts := make([]Account, 0)
	for _, accountPlatform := range platforms {
		for page := 1; ; page++ {
			params := pagination.PaginationParams{
				Page:      page,
				PageSize:  openAIAccountSchedulingPoolPageSize,
				SortBy:    "priority",
				SortOrder: pagination.SortOrderAsc,
			}
			pageAccounts, pageResult, err := s.accountRepo.ListWithFilters(
				ctx,
				params,
				accountPlatform,
				"",
				"",
				"",
				groupFilter,
				"",
				"",
			)
			if err != nil {
				return nil, useMixed, fmt.Errorf("query accounts failed: %w", err)
			}
			accounts = append(accounts, pageAccounts...)
			if pageResult == nil || page >= pageResult.Pages || len(pageAccounts) == 0 {
				break
			}
		}
	}

	enabled := make([]Account, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		if !account.IsActive() || !account.Schedulable {
			continue
		}
		if !openAIAccountSchedulingPoolPlatformAllowed(&account, platform, useMixed) {
			continue
		}
		enabled = append(enabled, account)
	}
	return enabled, useMixed, nil
}

func openAIAccountSchedulingPoolPlatformAllowed(account *Account, platform string, useMixed bool) bool {
	if account == nil {
		return false
	}
	if platform == PlatformOpenAI {
		return isOpenAICompatibleAccountForPlatform(account, platform)
	}
	if useMixed {
		if account.Platform == platform {
			return true
		}
		return account.Platform == PlatformAntigravity && account.IsMixedSchedulingEnabled()
	}
	return account.Platform == platform
}

type accountSchedulingRecovery struct {
	blocked bool
	unknown bool
	at      *time.Time
	reason  string
}

func (r *accountSchedulingRecovery) addTimedBlock(until *time.Time, reason string, now time.Time) {
	r.blocked = true
	if until == nil || !now.Before(*until) {
		r.unknown = true
		return
	}
	if r.at == nil || until.After(*r.at) {
		copied := *until
		r.at = &copied
		r.reason = reason
	}
}

func (r *accountSchedulingRecovery) addUnknownBlock() {
	r.blocked = true
	r.unknown = true
}

func (r accountSchedulingRecovery) nextScheduled() (*time.Time, string) {
	if !r.blocked || r.unknown || r.at == nil {
		return nil, ""
	}
	copied := *r.at
	return &copied, r.reason
}

func accountSchedulingAvailability(account Account, now time.Time) ([]string, accountSchedulingRecovery) {
	reasons := make([]string, 0, 6)
	recovery := accountSchedulingRecovery{}

	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		reasons = append(reasons, "expired")
		recovery.addUnknownBlock()
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		reasons = append(reasons, "overloaded")
		recovery.addTimedBlock(account.OverloadUntil, "overloaded", now)
	}
	if resetAt := account.effectiveRateLimitResetAt(now); resetAt != nil {
		reasons = append(reasons, "rate_limited")
		recovery.addTimedBlock(resetAt, "rate_limited", now)
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		reasons = append(reasons, "temp_unschedulable")
		recovery.addTimedBlock(account.TempUnschedulableUntil, "temp_unschedulable", now)
	}
	if account.IsAPIKeyOrBedrock() && account.IsQuotaExceeded() {
		reasons = append(reasons, "quota_exhausted")
		recovery.addUnknownBlock()
	}
	if !account.IsAvailabilityScheduleAllowedAt(now) {
		reasons = append(reasons, "availability_schedule")
		recovery.addUnknownBlock()
	}

	if account.Type == AccountTypeAPIKey {
		accountAt := account
		accountAt.nowForTest = &now
		savedKeyCount := accountAt.SavedAPIKeyCount()
		switch {
		case savedKeyCount == 0:
			reasons = append(reasons, "api_key_missing")
			recovery.addUnknownBlock()
		case len(accountAt.GetAPIKeys()) == 0:
			reasons = append(reasons, "api_keys_cooling_down")
			if nextKeyAt := earliestDisabledAPIKeyRecoveryAt(&accountAt, now); nextKeyAt != nil {
				recovery.addTimedBlock(nextKeyAt, "api_key_cooldown", now)
			} else {
				recovery.addUnknownBlock()
			}
		}
	}

	return uniqueNonEmptyStrings(reasons), recovery
}

func earliestDisabledAPIKeyRecoveryAt(account *Account, now time.Time) *time.Time {
	if account == nil {
		return nil
	}
	details := DisabledAPIKeyDetails(account.Credentials, now)
	var earliest *time.Time
	for _, detail := range details {
		if !detail.Disabled || strings.TrimSpace(detail.DisabledUntil) == "" {
			continue
		}
		until, err := time.Parse(time.RFC3339, detail.DisabledUntil)
		if err != nil || !now.Before(until) {
			continue
		}
		if earliest == nil || until.Before(*earliest) {
			copied := until
			earliest = &copied
		}
	}
	return earliest
}

func (s *OpenAIGatewayService) buildOpenAIAccountSchedulingPoolItem(account Account, filter OpenAIAccountSchedulingPoolFilter, now time.Time) OpenAIAccountSchedulingPoolItem {
	if filter.Platform != PlatformOpenAI || !account.IsOpenAI() {
		return buildGenericAccountSchedulingPoolItem(account, filter, now)
	}
	evalAccount := account

	reasons, recovery := accountSchedulingAvailability(account, now)
	status := OpenAIAccountSchedulingPoolStatusSchedulable
	if recovery.blocked {
		status = OpenAIAccountSchedulingPoolStatusBlocked
	}

	runtimeBlock, runtimeBlocked := s.SnapshotOpenAIAccountRuntimeBlock(&account, now)
	if runtimeBlocked {
		status = OpenAIAccountSchedulingPoolStatusBlocked
		reasons = append(reasons, "runtime_block:"+firstNonEmptyString(runtimeBlock.Reason, "active"))
		recovery.addTimedBlock(runtimeBlock.Until, "runtime_block", now)
	}

	filtered := false
	if filter.Model != "" && !account.IsModelSupported(filter.Model) {
		filtered = true
		reasons = append(reasons, "model_unsupported:"+filter.Model)
	}
	if !evalAccount.SupportsOpenAIEndpointCapability(filter.Endpoint) {
		filtered = true
		reasons = append(reasons, "endpoint_unsupported:"+string(filter.Endpoint))
	}
	if !s.isOpenAIAccountTransportCompatible(&evalAccount, filter.Transport) {
		filtered = true
		reasons = append(reasons, "transport_unsupported:"+string(filter.Transport))
	}
	if !evalAccount.SupportsOpenAIImageCapability(filter.ImageCapability) {
		filtered = true
		reasons = append(reasons, "image_capability_unsupported:"+string(filter.ImageCapability))
	}
	if filtered && status != OpenAIAccountSchedulingPoolStatusBlocked {
		status = OpenAIAccountSchedulingPoolStatusFiltered
	}

	pathHealth, pathHealthAvailable := s.snapshotOpenAIPathHealthForSchedulingPool(&evalAccount, filter.Transport)
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
		recovery.addTimedBlock(pathHealth.CooldownUntil, "path_cooldown", now)
	}

	var blockPtr *OpenAIAccountRuntimeBlockSnapshot
	if runtimeBlocked {
		snapshot := runtimeBlock
		blockPtr = &snapshot
	}
	nextScheduledAt, nextScheduledReason := recovery.nextScheduled()
	return OpenAIAccountSchedulingPoolItem{
		Account:             account,
		PoolStatus:          status,
		PoolReasons:         uniqueNonEmptyStrings(reasons),
		NextScheduledAt:     nextScheduledAt,
		NextScheduledReason: nextScheduledReason,
		RuntimeBlock:        blockPtr,
		PathHealth:          pathHealth,
		PathHealthAvailable: pathHealthAvailable,
		DerivedHealth:       DeriveAccountHealthState(&account, pathHealth, now),
		EffectiveLoadFactor: account.EffectiveLoadFactor(),
	}
}

func buildGenericAccountSchedulingPoolItem(account Account, filter OpenAIAccountSchedulingPoolFilter, now time.Time) OpenAIAccountSchedulingPoolItem {
	reasons, recovery := accountSchedulingAvailability(account, now)
	status := OpenAIAccountSchedulingPoolStatusSchedulable
	if recovery.blocked {
		status = OpenAIAccountSchedulingPoolStatusBlocked
	}
	if filter.Model != "" && !account.IsModelSupported(filter.Model) {
		if status != OpenAIAccountSchedulingPoolStatusBlocked {
			status = OpenAIAccountSchedulingPoolStatusFiltered
		}
		reasons = append(reasons, "model_unsupported:"+filter.Model)
	}
	health := OpenAIPathHealthRecord{Key: OpenAIPathHealthKeyForAccount(&account, ""), State: OpenAIPathHealthStateHealthy}
	nextScheduledAt, nextScheduledReason := recovery.nextScheduled()
	return OpenAIAccountSchedulingPoolItem{
		Account:             account,
		PoolStatus:          status,
		PoolReasons:         uniqueNonEmptyStrings(reasons),
		NextScheduledAt:     nextScheduledAt,
		NextScheduledReason: nextScheduledReason,
		PathHealth:          health,
		PathHealthAvailable: false,
		DerivedHealth:       DeriveAccountHealthState(&account, health, now),
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
