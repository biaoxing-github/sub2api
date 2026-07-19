package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolShowsHealthAndReasons(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	loadFactor := 3
	accounts := []Account{
		{
			ID:            101,
			Name:          "ready-api-key",
			Platform:      PlatformOpenAI,
			Type:          AccountTypeAPIKey,
			Status:        StatusActive,
			Schedulable:   true,
			Concurrency:   5,
			Priority:      20,
			LoadFactor:    &loadFactor,
			Credentials:   map[string]any{"api_key": "sk-ready"},
			AccountGroups: []AccountGroup{{GroupID: 7}},
		},
		{
			ID:            102,
			Name:          "line-degraded",
			Platform:      PlatformOpenAI,
			Type:          AccountTypeOAuth,
			Status:        StatusActive,
			Schedulable:   true,
			Concurrency:   2,
			AccountGroups: []AccountGroup{{GroupID: 7}},
		},
		{
			ID:            103,
			Name:          "runtime-blocked",
			Platform:      PlatformOpenAI,
			Type:          AccountTypeOAuth,
			Status:        StatusActive,
			Schedulable:   true,
			Concurrency:   1,
			AccountGroups: []AccountGroup{{GroupID: 7}},
		},
		{
			ID:          104,
			Name:        "model-filtered",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"api_key":       "sk-filtered",
				"model_mapping": map[string]any{"gpt-4.1": "gpt-4.1"},
			},
			AccountGroups: []AccountGroup{{GroupID: 7}},
		},
		{
			ID:            105,
			Name:          "manual-disabled",
			Platform:      PlatformOpenAI,
			Type:          AccountTypeOAuth,
			Status:        StatusActive,
			Schedulable:   false,
			AccountGroups: []AccountGroup{{GroupID: 7}},
		},
	}
	pathHealth := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		DegradedFailureThreshold: 1,
		OpenFailureThreshold:     4,
		Cooldown:                 time.Minute,
	})
	pathHealth.RecordFailure(OpenAIPathHealthKeyForAccount(&accounts[1], string(OpenAIUpstreamTransportHTTPSSE)), OpenAIPathFailureEOF, nil)
	svc := &OpenAIGatewayService{
		accountRepo:      schedulingPoolCompleteAccountRepo{accounts: accounts},
		openaiPathHealth: pathHealth,
	}
	svc.BlockAccountScheduling(&accounts[2], now.Add(10*time.Minute), "429")

	snapshot, err := svc.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		GroupID:   schedulingPoolInt64Ptr(7),
		Model:     "gpt-5.5",
		Endpoint:  OpenAIEndpointCapabilityResponses,
		Transport: OpenAIUpstreamTransportHTTPSSE,
	}, now)

	require.NoError(t, err)
	require.Equal(t, 4, snapshot.Total)
	require.Equal(t, 1, snapshot.SchedulableCount)
	require.Equal(t, 1, snapshot.DegradedCount)
	require.Equal(t, 1, snapshot.BlockedCount)
	require.Equal(t, 1, snapshot.FilteredCount)
	require.Equal(t, int64(7), *snapshot.GroupID)
	require.Equal(t, "gpt-5.5", snapshot.Model)
	require.NotContains(t, openAISchedulingPoolItemIDs(snapshot.Items), int64(105))

	items := openAISchedulingPoolItemsByID(snapshot.Items)
	require.Equal(t, OpenAIAccountSchedulingPoolStatusSchedulable, items[101].PoolStatus)
	require.Equal(t, 3, items[101].EffectiveLoadFactor)
	require.Empty(t, items[101].PoolReasons)

	require.Equal(t, OpenAIAccountSchedulingPoolStatusDegraded, items[102].PoolStatus)
	require.Equal(t, AccountDerivedHealthLineDegraded, items[102].DerivedHealth.State)
	require.Contains(t, items[102].PoolReasons, "path_health:degraded:unexpected_eof")

	require.Equal(t, OpenAIAccountSchedulingPoolStatusBlocked, items[103].PoolStatus)
	require.NotNil(t, items[103].RuntimeBlock)
	require.Equal(t, "429", items[103].RuntimeBlock.Reason)
	require.Contains(t, items[103].PoolReasons, "runtime_block:429")
	require.NotNil(t, items[103].NextScheduledAt)
	require.WithinDuration(t, now.Add(10*time.Minute), *items[103].NextScheduledAt, time.Second)
	require.Equal(t, "runtime_block", items[103].NextScheduledReason)

	require.Equal(t, OpenAIAccountSchedulingPoolStatusFiltered, items[104].PoolStatus)
	require.Contains(t, items[104].PoolReasons, "model_unsupported:gpt-5.5")
}

func TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolKeepsEnabledBlockedAccountsAndReportsRecovery(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keyCooldownUntil := now.Add(2 * time.Minute)
	rateLimitUntil := now.Add(7 * time.Minute)
	overloadUntil := now.Add(5 * time.Minute)
	tempUnschedulableUntil := now.Add(10 * time.Minute)

	coolingKey := "sk-cooling"
	accounts := []Account{
		{
			ID:          701,
			Name:        "ready",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Priority:    20,
			Credentials: map[string]any{"api_key": "sk-ready"},
		},
		{
			ID:          702,
			Name:        "single-key-cooling",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Priority:    10,
			Credentials: map[string]any{
				"api_keys": []any{coolingKey},
				CredentialAPIKeysDisabled: map[string]any{
					FingerprintAPIKey(coolingKey): map[string]any{
						"reason":         "service_unavailable",
						"disabled_at":    now.UTC().Format(time.RFC3339),
						"disabled_until": keyCooldownUntil.UTC().Format(time.RFC3339),
						"disabled_count": 1,
					},
				},
			},
		},
		{
			ID:                      703,
			Name:                    "multiple-windows",
			Platform:                PlatformOpenAI,
			Type:                    AccountTypeOAuth,
			Status:                  StatusActive,
			Schedulable:             true,
			Priority:                2,
			RateLimitResetAt:        &rateLimitUntil,
			OverloadUntil:           &overloadUntil,
			TempUnschedulableUntil:  &tempUnschedulableUntil,
			TempUnschedulableReason: "temporary upstream failure",
		},
		{
			ID:          704,
			Name:        "manual-disabled",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: false,
		},
		{
			ID:          705,
			Name:        "inactive",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusDisabled,
			Schedulable: true,
		},
	}
	repo := schedulingPoolCompleteAccountRepo{accounts: accounts}
	svc := &OpenAIGatewayService{accountRepo: repo}

	snapshot, err := svc.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		Platform: PlatformOpenAI,
	}, now)

	require.NoError(t, err)
	require.Equal(t, 3, snapshot.Total)
	require.Equal(t, 1, snapshot.SchedulableCount)
	require.Equal(t, 2, snapshot.BlockedCount)
	require.Equal(t, []int64{703, 702, 701}, openAISchedulingPoolItemIDs(snapshot.Items))

	items := openAISchedulingPoolItemsByID(snapshot.Items)
	require.Equal(t, OpenAIAccountSchedulingPoolStatusBlocked, items[702].PoolStatus)
	require.Contains(t, items[702].PoolReasons, "api_keys_cooling_down")
	require.NotContains(t, items[702].PoolReasons, "api_key_missing")
	require.NotNil(t, items[702].NextScheduledAt)
	require.WithinDuration(t, keyCooldownUntil, *items[702].NextScheduledAt, time.Second)
	require.Equal(t, "api_key_cooldown", items[702].NextScheduledReason)

	require.Equal(t, OpenAIAccountSchedulingPoolStatusBlocked, items[703].PoolStatus)
	require.Contains(t, items[703].PoolReasons, "rate_limited")
	require.Contains(t, items[703].PoolReasons, "overloaded")
	require.Contains(t, items[703].PoolReasons, "temp_unschedulable")
	require.NotNil(t, items[703].NextScheduledAt)
	require.WithinDuration(t, tempUnschedulableUntil, *items[703].NextScheduledAt, time.Second)
	require.Equal(t, "temp_unschedulable", items[703].NextScheduledReason)
}

func TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolReadsAllPages(t *testing.T) {
	accountCount := openAIAccountSchedulingPoolPageSize + 1
	accounts := make([]Account, 0, accountCount)
	for i := 0; i < accountCount; i++ {
		accounts = append(accounts, Account{
			ID:          int64(i + 1),
			Name:        fmt.Sprintf("paged-account-%04d", i+1),
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
		})
	}

	svc := &OpenAIGatewayService{accountRepo: schedulingPoolCompleteAccountRepo{accounts: accounts}}
	snapshot, err := svc.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		Platform: PlatformOpenAI,
	}, time.Now())

	require.NoError(t, err)
	require.Equal(t, accountCount, snapshot.Total)
	require.Equal(t, accountCount, snapshot.SchedulableCount)
	require.Len(t, snapshot.Items, accountCount)
}

func TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolSupportsAnthropicGroup(t *testing.T) {
	now := time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)
	accounts := []Account{
		{
			ID:            201,
			Name:          "anthropic-self-use",
			Platform:      PlatformAnthropic,
			Type:          AccountTypeOAuth,
			Status:        StatusActive,
			Schedulable:   true,
			Concurrency:   3,
			Priority:      10,
			AccountGroups: []AccountGroup{{GroupID: 2}},
		},
		{
			ID:            202,
			Name:          "openai-self-use",
			Platform:      PlatformOpenAI,
			Type:          AccountTypeAPIKey,
			Status:        StatusActive,
			Schedulable:   true,
			Credentials:   map[string]any{"api_key": "sk-openai"},
			AccountGroups: []AccountGroup{{GroupID: 2}},
		},
		{
			ID:            203,
			Name:          "anthropic-other-group",
			Platform:      PlatformAnthropic,
			Type:          AccountTypeOAuth,
			Status:        StatusActive,
			Schedulable:   true,
			AccountGroups: []AccountGroup{{GroupID: 3}},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo: schedulingPoolGroupAwareAccountRepo{
			schedulingPoolCompleteAccountRepo: schedulingPoolCompleteAccountRepo{accounts: accounts},
		},
	}

	snapshot, err := svc.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		GroupID:  schedulingPoolInt64Ptr(2),
		Platform: PlatformAnthropic,
	}, now)

	require.NoError(t, err)
	require.Equal(t, PlatformAnthropic, snapshot.Platform)
	require.Equal(t, 1, snapshot.Total)
	require.Equal(t, []int64{201}, openAISchedulingPoolItemIDs(snapshot.Items))
	require.Equal(t, OpenAIAccountSchedulingPoolStatusSchedulable, snapshot.Items[0].PoolStatus)
	require.False(t, snapshot.Items[0].PathHealthAvailable)
}

func TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolUsesListedAPIKeyBaseURLWithoutRefetch(t *testing.T) {
	now := time.Date(2026, 6, 9, 22, 10, 0, 0, time.UTC)
	listedAccount := Account{
		ID:          301,
		Name:        "listed-with-base-url",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_key":           "sk-listed",
			"base_url":          "https://custom-upstream.example/v1",
			"request_base_urls": []any{"https://custom-upstream.example/v1"},
		},
		AccountGroups: []AccountGroup{{GroupID: 2}},
	}
	firstTokenMs := 3456
	pathHealth := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{Enabled: true})
	pathHealth.RecordSuccess(OpenAIPathHealthKeyForAccount(&listedAccount, string(OpenAIUpstreamTransportHTTPSSE)), &firstTokenMs, nil)
	getByIDCalls := 0

	svc := &OpenAIGatewayService{
		accountRepo: schedulingPoolNoRefetchAccountRepo{
			schedulingPoolCompleteAccountRepo: schedulingPoolCompleteAccountRepo{accounts: []Account{listedAccount}},
			getByIDCalls:                      &getByIDCalls,
		},
		openaiPathHealth: pathHealth,
	}

	snapshot, err := svc.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		GroupID:   schedulingPoolInt64Ptr(2),
		Platform:  PlatformOpenAI,
		Transport: OpenAIUpstreamTransportHTTPSSE,
	}, now)

	require.NoError(t, err)
	require.Equal(t, 1, snapshot.Total)
	require.Equal(t, int64(1), snapshot.Items[0].PathHealth.Samples)
	require.Equal(t, int64(1), snapshot.Items[0].PathHealth.SuccessCount)
	require.Equal(t, "https://custom-upstream.example/v1", snapshot.Items[0].PathHealth.Key.Upstream)
	require.InDelta(t, firstTokenMs, snapshot.Items[0].PathHealth.TTFTEWMAMs, 0.01)
	require.Zero(t, getByIDCalls)
}

func TestOpenAIGatewayService_ManualProbeSuccessRestoresSchedulingPoolHealth(t *testing.T) {
	now := time.Date(2026, 6, 16, 18, 0, 0, 0, time.UTC)
	account := Account{
		ID:          62034,
		Name:        "manual-probe-restore",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_key": "sk-restore"},
		Extra: map[string]any{
			AccountProbeHealthExtraKey: map[string]any{
				"level":         AccountProbeHealthLineDegraded,
				"failure_count": 2,
				"last_error":    "unexpected EOF",
			},
		},
		AccountGroups: []AccountGroup{{GroupID: 2}},
	}
	repo := &manualProbeSchedulingPoolRepo{account: &account}
	pathHealth := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		DegradedFailureThreshold: 1,
		OpenFailureThreshold:     2,
	})
	accountKey := OpenAIPathHealthKeyForAccount(&account, string(OpenAIUpstreamTransportHTTPSSE))
	bucketKey := OpenAIPathHealthBucketKeyForAccount(&account, string(OpenAIUpstreamTransportHTTPSSE))
	pathHealth.RecordFailure(accountKey, OpenAIPathFailureEOF, nil)
	pathHealth.RecordFailure(bucketKey, OpenAIPathFailureEOF, nil)
	gateway := &OpenAIGatewayService{
		accountRepo:      repo,
		openaiPathHealth: pathHealth,
	}
	rateLimitService := NewRateLimitService(repo, nil, nil, nil, nil)
	rateLimitService.SetAccountRuntimeBlocker(gateway)

	before, err := gateway.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		GroupID:   schedulingPoolInt64Ptr(2),
		Platform:  PlatformOpenAI,
		Transport: OpenAIUpstreamTransportHTTPSSE,
	}, now)
	require.NoError(t, err)
	require.Equal(t, OpenAIAccountSchedulingPoolStatusDegraded, before.Items[0].PoolStatus)
	require.Equal(t, AccountDerivedHealthLineDegraded, before.Items[0].DerivedHealth.State)

	_, err = rateLimitService.RecordAccountProbeOutcome(context.Background(), AccountProbeOutcome{
		AccountID: account.ID,
		Account:   &account,
		Source:    AccountProbeOutcomeSourceManualTest,
		Success:   true,
		Reason:    "probe_success",
	})
	require.NoError(t, err)

	after, err := gateway.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		GroupID:   schedulingPoolInt64Ptr(2),
		Platform:  PlatformOpenAI,
		Transport: OpenAIUpstreamTransportHTTPSSE,
	}, now)
	require.NoError(t, err)
	require.Equal(t, OpenAIAccountSchedulingPoolStatusSchedulable, after.Items[0].PoolStatus)
	require.Equal(t, AccountDerivedHealthNormal, after.Items[0].DerivedHealth.State)
	require.Equal(t, OpenAIPathHealthStateHealthy, after.Items[0].PathHealth.State)
	require.Empty(t, after.Items[0].PoolReasons)
}

type manualProbeSchedulingPoolRepo struct {
	mockAccountRepoForGemini
	account *Account
}

func (r *manualProbeSchedulingPoolRepo) ListWithFilters(
	ctx context.Context,
	params pagination.PaginationParams,
	platform, accountType, status, search string,
	groupID int64,
	privacyMode, planType string,
) ([]Account, *pagination.PaginationResult, error) {
	if r.account == nil {
		return []Account{}, &pagination.PaginationResult{Page: params.Page, PageSize: params.Limit()}, nil
	}
	return schedulingPoolCompleteAccountRepo{accounts: []Account{*r.account}}.ListWithFilters(
		ctx, params, platform, accountType, status, search, groupID, privacyMode, planType,
	)
}

func (r *manualProbeSchedulingPoolRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return r.mockAccountRepoForGemini.GetByID(ctx, id)
}

func (r *manualProbeSchedulingPoolRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	return r.ListSchedulableByGroupIDAndPlatforms(ctx, groupID, []string{platform})
}

func (r *manualProbeSchedulingPoolRepo) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	if r.account == nil || !schedulingPoolPlatformIn(r.account.Platform, platforms) || !r.account.IsSchedulable() {
		return nil, nil
	}
	account := *r.account
	return []Account{schedulerTestEnsureOpenAIAPIKeyCredentials(account)}, nil
}

func (r *manualProbeSchedulingPoolRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.ListSchedulableByGroupIDAndPlatform(ctx, 0, platform)
}

func (r *manualProbeSchedulingPoolRepo) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return r.ListSchedulableByGroupIDAndPlatforms(ctx, 0, platforms)
}

func (r *manualProbeSchedulingPoolRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.ListSchedulableByGroupIDAndPlatform(ctx, 0, platform)
}

func (r *manualProbeSchedulingPoolRepo) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return r.ListSchedulableByGroupIDAndPlatforms(ctx, 0, platforms)
}

func (r *manualProbeSchedulingPoolRepo) ClearError(ctx context.Context, id int64) error {
	if r.account != nil && r.account.ID == id {
		r.account.Status = StatusActive
		r.account.ErrorMessage = ""
	}
	return nil
}

func (r *manualProbeSchedulingPoolRepo) ClearRateLimit(ctx context.Context, id int64) error {
	if r.account != nil && r.account.ID == id {
		r.account.RateLimitedAt = nil
		r.account.RateLimitResetAt = nil
	}
	return nil
}

func (r *manualProbeSchedulingPoolRepo) ClearTempUnschedulable(ctx context.Context, id int64) error {
	if r.account != nil && r.account.ID == id {
		r.account.TempUnschedulableUntil = nil
		r.account.TempUnschedulableReason = ""
	}
	return nil
}

func (r *manualProbeSchedulingPoolRepo) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	if r.account != nil && r.account.ID == id {
		r.account.Schedulable = schedulable
	}
	return nil
}

func (r *manualProbeSchedulingPoolRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	if r.account == nil || r.account.ID != id {
		return nil
	}
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for key, value := range updates {
		r.account.Extra[key] = value
	}
	return nil
}

func openAISchedulingPoolItemsByID(items []OpenAIAccountSchedulingPoolItem) map[int64]OpenAIAccountSchedulingPoolItem {
	out := make(map[int64]OpenAIAccountSchedulingPoolItem, len(items))
	for _, item := range items {
		out[item.Account.ID] = item
	}
	return out
}

func openAISchedulingPoolItemIDs(items []OpenAIAccountSchedulingPoolItem) []int64 {
	out := make([]int64, 0, len(items))
	for _, item := range items {
		out = append(out, item.Account.ID)
	}
	return out
}

func schedulingPoolInt64Ptr(v int64) *int64 {
	return &v
}

type schedulingPoolCompleteAccountRepo struct {
	AccountRepository
	accounts []Account
}

func (r schedulingPoolCompleteAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (r schedulingPoolCompleteAccountRepo) ListWithFilters(
	ctx context.Context,
	params pagination.PaginationParams,
	platform, accountType, status, search string,
	groupID int64,
	privacyMode, planType string,
) ([]Account, *pagination.PaginationResult, error) {
	filtered := make([]Account, 0, len(r.accounts))
	for i := range r.accounts {
		account := r.accounts[i]
		if platform != "" && account.Platform != platform {
			continue
		}
		if accountType != "" && account.Type != accountType {
			continue
		}
		if status != "" && account.Status != status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(account.Name), strings.ToLower(search)) {
			continue
		}
		if groupID == AccountListGroupUngrouped && hasAccountGroupMetadata(&account) {
			continue
		}
		if groupID > 0 && !isAccountInRequestedGroup(&account, &groupID) {
			continue
		}
		filtered = append(filtered, account)
	}
	total := int64(len(filtered))
	pageSize := params.Limit()
	pages := 0
	if pageSize > 0 && total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	start := params.Offset()
	if start >= len(filtered) {
		return []Account{}, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: pageSize, Pages: pages}, nil
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: pageSize, Pages: pages}, nil
}

type schedulingPoolGroupAwareAccountRepo struct {
	schedulingPoolCompleteAccountRepo
}

func (r schedulingPoolGroupAwareAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	return r.listByGroupAndPlatforms(groupID, []string{platform}), nil
}

func (r schedulingPoolGroupAwareAccountRepo) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	return r.listByGroupAndPlatforms(groupID, platforms), nil
}

func (r schedulingPoolGroupAwareAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.listUngroupedByPlatforms([]string{platform}), nil
}

func (r schedulingPoolGroupAwareAccountRepo) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return r.listUngroupedByPlatforms(platforms), nil
}

func (r schedulingPoolGroupAwareAccountRepo) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return r.listByPlatforms(platforms), nil
}

func (r schedulingPoolGroupAwareAccountRepo) listByGroupAndPlatforms(groupID int64, platforms []string) []Account {
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		account := acc
		if schedulingPoolPlatformIn(acc.Platform, platforms) && isAccountInRequestedGroup(&account, &groupID) {
			result = append(result, schedulerTestEnsureOpenAIAPIKeyCredentials(acc))
		}
	}
	return result
}

func (r schedulingPoolGroupAwareAccountRepo) listUngroupedByPlatforms(platforms []string) []Account {
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		account := acc
		if schedulingPoolPlatformIn(acc.Platform, platforms) && isAccountInRequestedGroup(&account, nil) {
			result = append(result, schedulerTestEnsureOpenAIAPIKeyCredentials(acc))
		}
	}
	return result
}

func (r schedulingPoolGroupAwareAccountRepo) listByPlatforms(platforms []string) []Account {
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		if schedulingPoolPlatformIn(acc.Platform, platforms) {
			result = append(result, schedulerTestEnsureOpenAIAPIKeyCredentials(acc))
		}
	}
	return result
}

func schedulingPoolPlatformIn(platform string, platforms []string) bool {
	for _, item := range platforms {
		if platform == item {
			return true
		}
	}
	return false
}

type schedulingPoolNoRefetchAccountRepo struct {
	schedulingPoolCompleteAccountRepo
	getByIDCalls *int
}

func (r schedulingPoolNoRefetchAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	if r.getByIDCalls != nil {
		*r.getByIDCalls = *r.getByIDCalls + 1
	}
	return nil, ErrAccountNotFound
}
