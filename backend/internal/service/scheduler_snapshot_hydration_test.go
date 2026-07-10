//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

type snapshotHydrationCache struct {
	snapshot []*Account
	accounts map[int64]*Account
}

func (c *snapshotHydrationCache) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	return c.snapshot, true, nil
}

func (c *snapshotHydrationCache) SetSnapshot(ctx context.Context, bucket SchedulerBucket, accounts []Account) error {
	return nil
}

func (c *snapshotHydrationCache) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	if c.accounts == nil {
		return nil, nil
	}
	return c.accounts[accountID], nil
}

func (c *snapshotHydrationCache) SetAccount(ctx context.Context, account *Account) error {
	return nil
}

func (c *snapshotHydrationCache) DeleteAccount(ctx context.Context, accountID int64) error {
	return nil
}

func (c *snapshotHydrationCache) UpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}

func (c *snapshotHydrationCache) TryLockBucket(ctx context.Context, bucket SchedulerBucket, ttl time.Duration) (bool, error) {
	return true, nil
}

func (c *snapshotHydrationCache) UnlockBucket(ctx context.Context, bucket SchedulerBucket) error {
	return nil
}

func (c *snapshotHydrationCache) ListBuckets(ctx context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}

func (c *snapshotHydrationCache) GetOutboxWatermark(ctx context.Context) (int64, error) {
	return 0, nil
}

func (c *snapshotHydrationCache) SetOutboxWatermark(ctx context.Context, id int64) error {
	return nil
}

func TestSchedulerSnapshotDefaultBucketsIncludesOpenAIMixedBuckets(t *testing.T) {
	t.Parallel()

	svc := NewSchedulerSnapshotService(nil, nil, nil, snapshotBucketGroupRepo{
		groups: []Group{
			{ID: 12, Platform: PlatformOpenAI, Status: StatusActive},
		},
	}, nil)

	buckets, err := svc.defaultBuckets(context.Background())
	if err != nil {
		t.Fatalf("defaultBuckets error: %v", err)
	}

	if !snapshotBucketListContains(buckets, SchedulerBucket{GroupID: 0, Platform: PlatformOpenAI, Mode: SchedulerModeMixed}) {
		t.Fatalf("expected root OpenAI mixed bucket in %#v", buckets)
	}
	if !snapshotBucketListContains(buckets, SchedulerBucket{GroupID: 12, Platform: PlatformOpenAI, Mode: SchedulerModeMixed}) {
		t.Fatalf("expected group OpenAI mixed bucket in %#v", buckets)
	}
}

func TestSchedulerSnapshotRebuildBucketsForPlatformIncludesOpenAIMixedBucket(t *testing.T) {
	t.Parallel()

	cache := &snapshotBucketRecordingCache{}
	svc := NewSchedulerSnapshotService(cache, nil, snapshotBucketAccountRepo{}, nil, nil)

	if err := svc.rebuildBucketsForPlatform(context.Background(), PlatformOpenAI, []int64{12}, "test", nil); err != nil {
		t.Fatalf("rebuildBucketsForPlatform error: %v", err)
	}

	if !snapshotBucketListContains(cache.setBuckets, SchedulerBucket{GroupID: 12, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}) {
		t.Fatalf("expected OpenAI single bucket in %#v", cache.setBuckets)
	}
	if !snapshotBucketListContains(cache.setBuckets, SchedulerBucket{GroupID: 12, Platform: PlatformOpenAI, Mode: SchedulerModeForced}) {
		t.Fatalf("expected OpenAI forced bucket in %#v", cache.setBuckets)
	}
	if !snapshotBucketListContains(cache.setBuckets, SchedulerBucket{GroupID: 12, Platform: PlatformOpenAI, Mode: SchedulerModeMixed}) {
		t.Fatalf("expected OpenAI mixed bucket in %#v", cache.setBuckets)
	}
}

func TestOpenAISelectAccountWithLoadAwareness_HydratesSelectedAccountFromSchedulerSnapshot(t *testing.T) {
	cache := &snapshotHydrationCache{
		snapshot: []*Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"gpt-4": "gpt-4",
					},
				},
			},
		},
		accounts: map[int64]*Account{
			1: {
				ID:          1,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				Credentials: map[string]any{
					"api_key":       "sk-live",
					"model_mapping": map[string]any{"gpt-4": "gpt-4"},
				},
			},
		},
	}

	schedulerSnapshot := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)
	groupID := int64(2)
	svc := &OpenAIGatewayService{
		schedulerSnapshot: schedulerSnapshot,
		cache:             &stubGatewayCache{},
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil {
		t.Fatalf("expected selected account")
	}
	if got := selection.Account.GetOpenAIApiKey(); got != "sk-live" {
		t.Fatalf("expected hydrated api key, got %q", got)
	}
}

func TestOpenAINewAcquiredSelectionResult_ReleasesSlotWhenHydrationFails(t *testing.T) {
	cache := &snapshotHydrationCache{
		accounts: map[int64]*Account{},
	}
	schedulerSnapshot := NewSchedulerSnapshotService(cache, nil, stubOpenAIAccountRepo{}, nil, nil)
	svc := &OpenAIGatewayService{
		schedulerSnapshot: schedulerSnapshot,
	}
	releaseCalls := 0

	selection, err := svc.newAcquiredSelectionResult(context.Background(), &Account{ID: 1001}, func() {
		releaseCalls++
	})

	if err == nil {
		t.Fatalf("expected hydration error")
	}
	if selection != nil {
		t.Fatalf("expected nil selection on hydration error")
	}
	if releaseCalls != 1 {
		t.Fatalf("expected release to be called once, got %d", releaseCalls)
	}
}

func TestGatewaySelectAccountWithLoadAwareness_HydratesSelectedAccountFromSchedulerSnapshot(t *testing.T) {
	cache := &snapshotHydrationCache{
		snapshot: []*Account{
			{
				ID:          9,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
			},
		},
		accounts: map[int64]*Account{
			9: {
				ID:          9,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				Credentials: map[string]any{
					"api_key": "anthropic-live-key",
				},
			},
		},
	}

	schedulerSnapshot := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)
	svc := &GatewayService{
		schedulerSnapshot: schedulerSnapshot,
		cache:             &mockGatewayCacheForPlatform{},
		cfg:               testConfig(),
	}

	result, err := svc.SelectAccountWithLoadAwareness(context.Background(), nil, "", "claude-3-5-sonnet-20241022", nil, "", 0)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if result == nil || result.Account == nil {
		t.Fatalf("expected selected account")
	}
	if got := result.Account.GetCredential("api_key"); got != "anthropic-live-key" {
		t.Fatalf("expected hydrated api key, got %q", got)
	}
}

func TestGatewaySelectAccountWithLoadAwareness_HydratesRoutedStickyWaitPlan(t *testing.T) {
	groupID := int64(42)
	sessionHash := "sticky-session"
	model := "claude-3-5-sonnet-20241022"
	cache := &snapshotHydrationCache{
		snapshot: []*Account{
			{
				ID:          9,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				GroupIDs:    []int64{groupID},
			},
		},
		accounts: map[int64]*Account{
			9: {
				ID:          9,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				GroupIDs:    []int64{groupID},
				Credentials: map[string]any{
					"api_key": "anthropic-live-key",
				},
			},
		},
	}
	group := &Group{
		ID:                  groupID,
		Status:              StatusActive,
		Platform:            PlatformAnthropic,
		Hydrated:            true,
		ModelRoutingEnabled: true,
		ModelRouting:        map[string][]int64{model: []int64{9}},
	}
	svc := &GatewayService{
		schedulerSnapshot:  NewSchedulerSnapshotService(cache, nil, nil, nil, nil),
		cache:              &stubGatewayCache{sessionBindings: map[string]int64{sessionHash: 9}},
		cfg:                testConfig(),
		concurrencyService: NewConcurrencyService(&stubConcurrencyCacheForTest{acquireResult: false, waitCount: 0}),
	}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)

	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, model, nil, "", 0)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if result == nil || result.Account == nil || result.WaitPlan == nil {
		t.Fatalf("expected wait-plan selection, got %+v", result)
	}
	if got := result.Account.GetCredential("api_key"); got != "anthropic-live-key" {
		t.Fatalf("expected hydrated api key on sticky wait plan, got %q", got)
	}
}

type snapshotBucketRecordingCache struct {
	setBuckets []SchedulerBucket
}

func (c *snapshotBucketRecordingCache) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *snapshotBucketRecordingCache) SetSnapshot(ctx context.Context, bucket SchedulerBucket, accounts []Account) error {
	c.setBuckets = append(c.setBuckets, bucket)
	return nil
}

func (c *snapshotBucketRecordingCache) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	return nil, nil
}

func (c *snapshotBucketRecordingCache) SetAccount(ctx context.Context, account *Account) error {
	return nil
}

func (c *snapshotBucketRecordingCache) DeleteAccount(ctx context.Context, accountID int64) error {
	return nil
}

func (c *snapshotBucketRecordingCache) UpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}

func (c *snapshotBucketRecordingCache) TryLockBucket(ctx context.Context, bucket SchedulerBucket, ttl time.Duration) (bool, error) {
	return true, nil
}

func (c *snapshotBucketRecordingCache) UnlockBucket(ctx context.Context, bucket SchedulerBucket) error {
	return nil
}

func (c *snapshotBucketRecordingCache) ListBuckets(ctx context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}

func (c *snapshotBucketRecordingCache) GetOutboxWatermark(ctx context.Context) (int64, error) {
	return 0, nil
}

func (c *snapshotBucketRecordingCache) SetOutboxWatermark(ctx context.Context, id int64) error {
	return nil
}

type snapshotBucketAccountRepo struct {
	AccountRepository
}

func (snapshotBucketAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	return nil, nil
}

func (snapshotBucketAccountRepo) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	return nil, nil
}

type snapshotBucketGroupRepo struct {
	GroupRepository
	groups []Group
}

func (r snapshotBucketGroupRepo) ListActive(ctx context.Context) ([]Group, error) {
	return r.groups, nil
}

func snapshotBucketListContains(buckets []SchedulerBucket, want SchedulerBucket) bool {
	for _, bucket := range buckets {
		if bucket == want {
			return true
		}
	}
	return false
}
