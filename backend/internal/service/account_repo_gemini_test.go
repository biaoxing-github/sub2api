package service

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// mockAccountRepoForGemini 是 Gemini/OpenAI 调度测试共用的账号仓储桩。
type mockAccountRepoForGemini struct {
	accounts           []Account
	accountsByID       map[int64]*Account
	listByGroupFunc    func(ctx context.Context, groupID int64, platforms []string) ([]Account, error)
	listByPlatformFunc func(ctx context.Context, platforms []string) ([]Account, error)
}

func (m *mockAccountRepoForGemini) GetByID(ctx context.Context, id int64) (*Account, error) {
	if acc, ok := m.accountsByID[id]; ok {
		return acc, nil
	}
	return nil, errors.New("account not found")
}

func (m *mockAccountRepoForGemini) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	var result []*Account
	for _, id := range ids {
		if acc, ok := m.accountsByID[id]; ok {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockAccountRepoForGemini) ExistsByID(ctx context.Context, id int64) (bool, error) {
	if m.accountsByID == nil {
		return false, nil
	}
	_, ok := m.accountsByID[id]
	return ok, nil
}

func (m *mockAccountRepoForGemini) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range m.accounts {
		if acc.Platform == platform && acc.IsSchedulable() {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockAccountRepoForGemini) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	// 测试桩不关心分组隔离，调用方按平台断言调度行为即可。
	return m.ListSchedulableByPlatform(ctx, platform)
}

func (m *mockAccountRepoForGemini) Create(ctx context.Context, account *Account) error { return nil }

func (m *mockAccountRepoForGemini) GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) FindByExtraField(ctx context.Context, key string, value any) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) ListCRSAccountIDs(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) Update(ctx context.Context, account *Account) error { return nil }

func (m *mockAccountRepoForGemini) Delete(ctx context.Context, id int64) error { return nil }

func (m *mockAccountRepoForGemini) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (m *mockAccountRepoForGemini) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode, planType string) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (m *mockAccountRepoForGemini) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) ListActive(ctx context.Context) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) UpdateLastUsed(ctx context.Context, id int64) error { return nil }

func (m *mockAccountRepoForGemini) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}

func (m *mockAccountRepoForGemini) SetError(ctx context.Context, id int64, errorMsg string) error {
	return nil
}

func (m *mockAccountRepoForGemini) ClearError(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAccountRepoForGemini) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	return nil
}

func (m *mockAccountRepoForGemini) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}

func (m *mockAccountRepoForGemini) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	return nil
}

func (m *mockAccountRepoForGemini) ListSchedulable(ctx context.Context) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	if m.listByPlatformFunc != nil {
		return m.listByPlatformFunc(ctx, platforms)
	}
	var result []Account
	platformSet := make(map[string]bool)
	for _, p := range platforms {
		platformSet[p] = true
	}
	for _, acc := range m.accounts {
		if platformSet[acc.Platform] && acc.IsSchedulable() {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockAccountRepoForGemini) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	if m.listByGroupFunc != nil {
		return m.listByGroupFunc(ctx, groupID, platforms)
	}
	return m.ListSchedulableByPlatforms(ctx, platforms)
}

func (m *mockAccountRepoForGemini) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return m.ListSchedulableByPlatform(ctx, platform)
}

func (m *mockAccountRepoForGemini) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return m.ListSchedulableByPlatforms(ctx, platforms)
}

func (m *mockAccountRepoForGemini) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	return nil
}

func (m *mockAccountRepoForGemini) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time) error {
	return nil
}

func (m *mockAccountRepoForGemini) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	return nil
}

func (m *mockAccountRepoForGemini) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	return nil
}

func (m *mockAccountRepoForGemini) ClearTempUnschedulable(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAccountRepoForGemini) ClearRateLimit(ctx context.Context, id int64) error { return nil }

func (m *mockAccountRepoForGemini) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAccountRepoForGemini) ClearModelRateLimits(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAccountRepoForGemini) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	return nil
}

func (m *mockAccountRepoForGemini) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	return nil
}

func (m *mockAccountRepoForGemini) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	return 0, nil
}

func (m *mockAccountRepoForGemini) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	return nil
}

func (m *mockAccountRepoForGemini) ResetQuotaUsed(ctx context.Context, id int64) error {
	return nil
}

var _ AccountRepository = (*mockAccountRepoForGemini)(nil)
