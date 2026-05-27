package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type upstreamBalanceRefreshOneRepo struct {
	account       *Account
	accounts      []Account
	updateExtraID int64
	updateExtra   map[string]any
	bulkUpdateIDs []int64
	bulkUpdate    AccountBulkUpdate
}

func (r *upstreamBalanceRefreshOneRepo) Create(context.Context, *Account) error { return nil }
func (r *upstreamBalanceRefreshOneRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, ErrAccountNotFound
}
func (r *upstreamBalanceRefreshOneRepo) GetByIDs(context.Context, []int64) ([]*Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ExistsByID(context.Context, int64) (bool, error) {
	return false, nil
}
func (r *upstreamBalanceRefreshOneRepo) GetByCRSAccountID(context.Context, string) (*Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) FindByExtraField(context.Context, string, any) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) Update(context.Context, *Account) error { return nil }
func (r *upstreamBalanceRefreshOneRepo) Delete(context.Context, int64) error    { return nil }
func (r *upstreamBalanceRefreshOneRepo) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string, string) ([]Account, *pagination.PaginationResult, error) {
	if len(r.accounts) > 0 {
		copied := append([]Account(nil), r.accounts...)
		return copied, &pagination.PaginationResult{Total: int64(len(copied))}, nil
	}
	return nil, nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListByGroup(context.Context, int64) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListActive(context.Context) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) UpdateLastUsed(context.Context, int64) error { return nil }
func (r *upstreamBalanceRefreshOneRepo) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) SetError(context.Context, int64, string) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) ClearError(context.Context, int64) error { return nil }
func (r *upstreamBalanceRefreshOneRepo) SetSchedulable(context.Context, int64, bool) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *upstreamBalanceRefreshOneRepo) BindGroups(context.Context, int64, []int64) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulable(context.Context) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulableByPlatforms(context.Context, []string) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	return nil, nil
}
func (r *upstreamBalanceRefreshOneRepo) SetRateLimited(context.Context, int64, time.Time) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) SetModelRateLimit(context.Context, int64, string, time.Time) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) SetOverloaded(context.Context, int64, time.Time) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) ClearTempUnschedulable(context.Context, int64) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) ClearRateLimit(context.Context, int64) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) ClearAntigravityQuotaScopes(context.Context, int64) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) ClearModelRateLimits(context.Context, int64) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.updateExtraID = id
	r.updateExtra = updates
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	r.bulkUpdateIDs = ids
	r.bulkUpdate = updates
	return int64(len(ids)), nil
}
func (r *upstreamBalanceRefreshOneRepo) IncrementQuotaUsed(context.Context, int64, float64) error {
	return nil
}
func (r *upstreamBalanceRefreshOneRepo) ResetQuotaUsed(context.Context, int64) error {
	return nil
}

type upstreamBalanceRefreshOneHTTP struct {
	requests  []*http.Request
	body      string
	responses map[string]string
}

func (h *upstreamBalanceRefreshOneHTTP) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	h.requests = append(h.requests, req)
	body := h.body
	if h.responses != nil {
		if matched, ok := h.responses[req.URL.Path]; ok {
			body = matched
		}
	}
	if body == "" {
		body = `{"total_granted":20,"total_used":7.5,"total_available":12.5}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (h *upstreamBalanceRefreshOneHTTP) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return h.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestUpstreamBalanceServiceRefreshOneRefreshesOnlyRequestedAccount(t *testing.T) {
	repo := &upstreamBalanceRefreshOneRepo{
		account: &Account{
			ID:          42,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk-test"},
		},
	}
	httpUpstream := &upstreamBalanceRefreshOneHTTP{}
	svc := NewUpstreamBalanceService(repo, httpUpstream, time.Minute)

	snapshot, err := svc.RefreshOne(context.Background(), 42)
	if err != nil {
		t.Fatalf("RefreshOne() error = %v", err)
	}
	if repo.updateExtraID != 42 {
		t.Fatalf("updated account id = %d, want 42", repo.updateExtraID)
	}
	if snapshot.Available != 12.5 || snapshot.Used != 7.5 || snapshot.Total != 20 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if repo.updateExtra[UpstreamBalanceAvailableKey] != 12.5 {
		t.Fatalf("updated extra = %+v", repo.updateExtra)
	}
	if len(httpUpstream.requests) == 0 || !strings.HasSuffix(httpUpstream.requests[0].URL.String(), "/v1/usage") {
		t.Fatalf("requests = %+v", httpUpstream.requests)
	}
}

func TestUpstreamBalanceBaseURLUsesDedicatedBalanceBaseURL(t *testing.T) {
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":          "https://request.example.com/v1",
			"request_base_urls": []string{"https://request.example.com/v1", "https://fast.example.com/v1"},
			"balance_base_url":  "https://balance.example.com/v1/",
		},
	}

	got := upstreamBalanceBaseURL(account)

	if got != "https://balance.example.com" {
		t.Fatalf("upstreamBalanceBaseURL() = %q, want %q", got, "https://balance.example.com")
	}
}

func TestUpstreamBalanceServiceRefreshAllHonorsActiveAccountLimit(t *testing.T) {
	repo := &upstreamBalanceRefreshOneRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-one"}},
			{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-two"}},
			{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-three"}},
		},
	}
	httpUpstream := &upstreamBalanceRefreshOneHTTP{}
	svc := NewUpstreamBalanceService(repo, httpUpstream, time.Minute)
	svc.activeAccountLimit = 2

	result, err := svc.RefreshAll(context.Background())
	if err != nil {
		t.Fatalf("RefreshAll() error = %v", err)
	}
	if result.MatchedAccounts != 3 {
		t.Fatalf("matched accounts = %d, want 3", result.MatchedAccounts)
	}
	if result.Refreshed != 2 {
		t.Fatalf("refreshed = %d, want 2", result.Refreshed)
	}
	if len(httpUpstream.requests) != 4 {
		t.Fatalf("upstream requests = %d, want 4 for two refreshed accounts", len(httpUpstream.requests))
	}
}

func TestUpstreamBalanceServiceRefreshOneUsesNewAPIUsageGroups(t *testing.T) {
	repo := &upstreamBalanceRefreshOneRepo{
		account: &Account{
			ID:       42,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key": "sk-test",
			},
		},
	}
	httpUpstream := &upstreamBalanceRefreshOneHTTP{
		body: `{"code":0,"data":{"user":{"balance":1001.9812742},"items":[{"api_key":{"key":"sk-test","name":"codex","group_id":2},"group":{"id":2,"name":"codex","rate_multiplier":0.7}}]}}`,
	}
	svc := NewUpstreamBalanceService(repo, httpUpstream, time.Minute)

	snapshot, err := svc.RefreshOne(context.Background(), 42)
	if err != nil {
		t.Fatalf("RefreshOne() error = %v", err)
	}
	if len(snapshot.Groups) != 1 || snapshot.Groups[0].Name != "codex" || snapshot.Groups[0].Ratio != 0.7 {
		t.Fatalf("snapshot groups = %+v", snapshot.Groups)
	}
	if len(snapshot.Keys) != 1 || len(snapshot.Keys[0].Groups) != 1 || snapshot.Keys[0].Groups[0].Name != "codex" {
		t.Fatalf("key groups = %+v", snapshot.Keys)
	}
	if snapshot.ConvertedAvailableByGroup["codex"] == 0 {
		t.Fatalf("converted groups = %+v", snapshot.ConvertedAvailableByGroup)
	}
}

func TestUpstreamBalanceServiceRefreshOneSyncsUpstreamAuthMeConcurrency(t *testing.T) {
	repo := &upstreamBalanceRefreshOneRepo{
		account: &Account{
			ID:          42,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Concurrency: 3,
			Credentials: map[string]any{
				"api_key":                         "sk-test",
				UpstreamAuthUsernameCredentialKey: "alice@example.com",
				UpstreamAuthPasswordCredentialKey: "secret",
				UpstreamBalanceEndpointPathsKey:   []any{"/api/v1/auth/me"},
			},
		},
	}
	httpUpstream := &upstreamBalanceRefreshOneHTTP{
		responses: map[string]string{
			"/api/user/login":        `{"success":true,"token":"login-token","data":{"id":99}}`,
			"/api/v1/auth/me":        `{"code":0,"message":"success","data":{"id":99,"balance":12.5,"concurrency":8}}`,
			"/api/subscription/self": `{"code":404}`,
		},
	}
	svc := NewUpstreamBalanceService(repo, httpUpstream, time.Minute)

	if _, err := svc.RefreshOne(context.Background(), 42); err != nil {
		t.Fatalf("RefreshOne() error = %v", err)
	}
	if len(repo.bulkUpdateIDs) != 1 || repo.bulkUpdateIDs[0] != 42 {
		t.Fatalf("bulk update ids = %+v", repo.bulkUpdateIDs)
	}
	if repo.bulkUpdate.Concurrency == nil || *repo.bulkUpdate.Concurrency != 8 {
		t.Fatalf("synced concurrency = %+v, want 8", repo.bulkUpdate.Concurrency)
	}
}

func TestUpstreamBalanceServiceRefreshOnePreservesManualGroupsWhenUpstreamOmitsGroups(t *testing.T) {
	repo := &upstreamBalanceRefreshOneRepo{
		account: &Account{
			ID:       42,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":                       "sk-test",
				UpstreamCommonRateMultiplierKey: 0.24,
				UpstreamCommonRateGroupNameKey:  "codex",
			},
			Extra: map[string]any{
				UpstreamCommonRateMultiplierKey: 0.24,
				UpstreamCommonRateGroupNameKey:  nil,
			},
		},
	}
	httpUpstream := &upstreamBalanceRefreshOneHTTP{
		body: `{"balance":48.95693364,"isValid":true,"mode":"unrestricted","planName":"钱包余额","remaining":48.95693364,"unit":"USD"}`,
	}
	svc := NewUpstreamBalanceService(repo, httpUpstream, time.Minute)

	snapshot, err := svc.RefreshOne(context.Background(), 42)
	if err != nil {
		t.Fatalf("RefreshOne() error = %v", err)
	}
	if len(snapshot.Groups) != 1 || snapshot.Groups[0].Name != "codex" || snapshot.Groups[0].Ratio != 0.24 {
		t.Fatalf("snapshot groups = %+v", snapshot.Groups)
	}
	if len(snapshot.Keys) != 1 || len(snapshot.Keys[0].Groups) != 1 || snapshot.Keys[0].Groups[0].Name != "codex" {
		t.Fatalf("key groups = %+v", snapshot.Keys)
	}
	if repo.updateExtra[UpstreamBalanceGroupsKey] == nil {
		t.Fatalf("updated extra did not preserve groups: %+v", repo.updateExtra)
	}
}

func TestGroupsForKeyPrefersFetchedGroupsOverManualRate(t *testing.T) {
	account := &Account{
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":                       "sk-test",
			UpstreamCommonRateMultiplierKey: 9.0,
			UpstreamCommonRateGroupNameKey:  "manual",
		},
	}
	auth := &upstreamAuthContext{
		groupsByKey: map[string][]UpstreamBalanceGroupSnapshot{
			FingerprintAPIKey("sk-test"): {{Name: "login-group", Ratio: 0.2}},
		},
		allGroups: []UpstreamBalanceGroupSnapshot{{Name: "login-group", Ratio: 0.2}},
	}

	got := groupsForKey(auth, account, "sk-test")
	if len(got) != 1 || got[0].Name != "login-group" || got[0].Ratio != 0.2 {
		t.Fatalf("groupsForKey() = %+v", got)
	}
}

func TestGroupsForKeyPrefersManualRateOverStaleFetchedGroups(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			"api_keys":                      []any{"sk-test", "sk-other"},
			UpstreamCommonRateMultiplierKey: 0.6,
			UpstreamCommonRateGroupNameKey:  "codex",
		},
		Extra: map[string]any{
			UpstreamCommonRateMultiplierKey: 0.6,
			UpstreamFetchedGroupsKey: []any{
				map[string]any{"name": "<nil>", "ratio": 0.6, "description": "manual common rate"},
			},
		},
	}

	got := groupsForKey(nil, account, "sk-test")
	if len(got) != 1 || got[0].Name != "codex" || got[0].Ratio != 0.6 {
		t.Fatalf("groupsForKey() = %+v", got)
	}
}

func TestManualRateGroupsPrefersExtraRatioAndCredentialName(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			UpstreamCommonRateMultiplierKey: 9.0,
			UpstreamCommonRateGroupNameKey:  "codex",
		},
		Extra: map[string]any{
			UpstreamCommonRateMultiplierKey: 0.2,
		},
	}

	got := manualRateGroups(account)
	if len(got) != 1 || got[0].Name != "codex" || got[0].Ratio != 0.2 {
		t.Fatalf("manualRateGroups() = %+v", got)
	}
}

func TestManualRateGroupsFallsBackWhenExtraGroupNameIsNil(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			UpstreamCommonRateMultiplierKey: 0.24,
			UpstreamCommonRateGroupNameKey:  "codex",
		},
		Extra: map[string]any{
			UpstreamCommonRateMultiplierKey: 0.24,
			UpstreamCommonRateGroupNameKey:  nil,
		},
	}

	got := manualRateGroups(account)
	if len(got) != 1 || got[0].Name != "codex" || got[0].Ratio != 0.24 {
		t.Fatalf("manualRateGroups() = %+v", got)
	}
}

func TestManualBalanceForAccountUsesQuotaLimitMinusQuotaUsed(t *testing.T) {
	account := &Account{Extra: map[string]any{"quota_limit": 100.0, "quota_used": 37.5}}

	got := manualBalanceForAccount(account)
	if got == nil {
		t.Fatal("manualBalanceForAccount() = nil")
	}
	assertFloatPtr(t, got.Available, 62.5)
	assertFloatPtr(t, got.Used, 37.5)
	assertFloatPtr(t, got.Total, 100)
}

func TestParseUpstreamBalanceResponse_OpenAICreditGrants(t *testing.T) {
	got, err := ParseUpstreamBalanceResponse([]byte(`{"total_granted":20,"total_used":7.5,"total_available":12.5}`))
	if err != nil {
		t.Fatalf("ParseUpstreamBalanceResponse() error = %v", err)
	}
	assertFloatPtr(t, got.Total, 20)
	assertFloatPtr(t, got.Used, 7.5)
	assertFloatPtr(t, got.Available, 12.5)
}

func TestParseUpstreamBalanceResponse_NewAPIUserSelf(t *testing.T) {
	got, err := ParseUpstreamBalanceResponse([]byte(`{"data":{"quota":100,"used":35}}`))
	if err != nil {
		t.Fatalf("ParseUpstreamBalanceResponse() error = %v", err)
	}
	assertFloatPtr(t, got.Total, 100)
	assertFloatPtr(t, got.Used, 35)
	assertFloatPtr(t, got.Available, 65)
}

func TestParseUpstreamBalanceResponse_Sub2APIUsage(t *testing.T) {
	got, err := ParseUpstreamBalanceResponse([]byte(`{"balance":48.95752932,"isValid":true,"mode":"unrestricted"}`))
	if err != nil {
		t.Fatalf("ParseUpstreamBalanceResponse() error = %v", err)
	}
	assertFloatPtr(t, got.Available, 48.95752932)
	if got.Used != nil || got.Total != nil {
		t.Fatalf("used/total = %v/%v, want nil/nil", got.Used, got.Total)
	}
}

func TestParseUpstreamBalanceResponse_NewAPITokenUsage(t *testing.T) {
	got, err := ParseUpstreamBalanceResponse([]byte(`{"code":true,"data":{"object":"token_usage","total_available":250000,"total_used":750000,"total_granted":1000000,"unlimited_quota":false}}`))
	if err != nil {
		t.Fatalf("ParseUpstreamBalanceResponse() error = %v", err)
	}
	assertFloatPtr(t, got.Available, 0.5)
	assertFloatPtr(t, got.Used, 1.5)
	assertFloatPtr(t, got.Total, 2)
}

func TestParseUpstreamBalanceResponse_NewAPIUnlimitedTokenUsageRejected(t *testing.T) {
	_, err := ParseUpstreamBalanceResponse([]byte(`{"code":true,"data":{"object":"token_usage","total_available":-250000,"total_used":250000,"total_granted":0,"unlimited_quota":true}}`))
	if err == nil {
		t.Fatal("ParseUpstreamBalanceResponse() error = nil, want error")
	}
}

func TestParseUpstreamBalanceResponse_BillingSubscriptionRejectsUnlimitedCompatibilityLimit(t *testing.T) {
	_, err := ParseUpstreamBalanceResponse([]byte(`{"object":"billing_subscription","soft_limit_usd":100000000,"hard_limit_usd":100000000}`))
	if err == nil {
		t.Fatal("ParseUpstreamBalanceResponse() error = nil, want error")
	}
}

func TestParseNewAPIUserSelfResponse(t *testing.T) {
	got, err := parseNewAPIUserSelfResponse([]byte(`{"success":true,"data":{"quota":4750000,"used_quota":250000}}`))
	if err != nil {
		t.Fatalf("parseNewAPIUserSelfResponse() error = %v", err)
	}
	assertFloatPtr(t, got.Available, 9.5)
	assertFloatPtr(t, got.Used, 0.5)
	assertFloatPtr(t, got.Total, 10)
}

func TestParseNewAPIUsageResponseUserBalance(t *testing.T) {
	got, err := parseNewAPIUsageResponse([]byte(`{"code":0,"data":{"user":{"balance":1001.9812742},"items":[{"group":{"name":"codex","rate_multiplier":0.7}}]}}`))
	if err != nil {
		t.Fatalf("parseNewAPIUsageResponse() error = %v", err)
	}
	assertFloatPtr(t, got.Available, 1001.9812742)
	if got.Used != nil || got.Total != nil {
		t.Fatalf("used/total = %v/%v, want nil/nil", got.Used, got.Total)
	}
}

func TestParseSubscriptionSelfResponse(t *testing.T) {
	got, err := parseSubscriptionSelfResponse([]byte(`{"success":true,"data":{"subscriptions":[{"subscription":{"status":"active","amount_total":248375000,"amount_used":0}}]}}`))
	if err != nil {
		t.Fatalf("parseSubscriptionSelfResponse() error = %v", err)
	}
	assertFloatPtr(t, got.Available, 496.75)
	assertFloatPtr(t, got.Total, 496.75)
}

func TestExtractNewAPIUserIDPrefersDataID(t *testing.T) {
	got := extractNewAPIUserID([]byte(`{"id":"outer-id","success":true,"data":{"id":127,"username":"ray"}}`))
	if got != "127" {
		t.Fatalf("extractNewAPIUserID() = %q, want 127", got)
	}
}

func TestParseUpstreamGroupsResponse_NewAPIPricing(t *testing.T) {
	got := ParseUpstreamGroupsResponse([]byte(`{"group_ratio":{"claude":0.2,"pro分组":2.5}}`))
	if len(got) != 2 {
		t.Fatalf("groups = %+v", got)
	}
	if got[0].Name != "claude" || got[0].Ratio != 0.2 || got[1].Name != "pro分组" || got[1].Ratio != 2.5 {
		t.Fatalf("groups = %+v", got)
	}
}

func TestParseUpstreamTokenListResponse(t *testing.T) {
	got := ParseUpstreamTokenListResponse([]byte(`{"data":{"items":[{"key":"sk-test","group":"claude"},{"key":"sk-other","group":{"name":"pro分组"}}]}}`))
	if len(got) != 2 {
		t.Fatalf("tokens = %+v", got)
	}
	if got[0].fullKey != "sk-test" || got[0].group != "claude" || got[1].group != "pro分组" {
		t.Fatalf("tokens = %+v", got)
	}
}

func TestParseUpstreamGroupsResponse_NewAPIUserGroups(t *testing.T) {
	got := ParseUpstreamGroupsResponse([]byte(`{"success":true,"data":{"claude":{"desc":"Claude group","ratio":0.2},"pro":{"desc":"Pro group","ratio":2.5}}}`))
	if len(got) != 2 {
		t.Fatalf("groups = %+v", got)
	}
	if got[0].Name != "claude" || got[0].Ratio != 0.2 || got[1].Name != "pro" || got[1].Ratio != 2.5 {
		t.Fatalf("groups = %+v", got)
	}
}

func TestParseUpstreamGroupsResponse_NewAPIUsageItems(t *testing.T) {
	got := ParseUpstreamGroupsResponse([]byte(`{"code":0,"data":{"items":[{"api_key":{"key":"sk-test","name":"codex","group_id":2},"group":{"id":2,"name":"codex","rate_multiplier":0.7}},{"api_key":{"key":"sk-other","name":"plus","group_id":3},"group":{"id":3,"name":"plus","rate_multiplier":1.2}}]}}`))
	if len(got) != 2 {
		t.Fatalf("groups = %+v", got)
	}
	if got[0].Name != "codex" || got[0].Ratio != 0.7 || got[1].Name != "plus" || got[1].Ratio != 1.2 {
		t.Fatalf("groups = %+v", got)
	}
}

func TestUpstreamBalanceEndpointsTriesSub2APIUsageFirst(t *testing.T) {
	got := upstreamBalanceEndpoints("https://example.com/v1")
	if len(got) == 0 || got[0] != "https://example.com/v1/usage" || got[1] != "https://example.com/v1/dashboard/billing/subscription" {
		t.Fatalf("first endpoint = %q", got)
	}
}

func TestUpstreamBalanceSnapshotFromExtra(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	keys, _ := json.Marshal([]UpstreamBalanceKeySnapshot{{Fingerprint: "sha256:x", Masked: "sk...1234", Status: "ok", Available: floatPtr(3)}})
	var keyRaw []any
	_ = json.Unmarshal(keys, &keyRaw)

	got := UpstreamBalanceSnapshotFromExtra(map[string]any{
		UpstreamBalanceAvailableKey:   3.0,
		UpstreamBalanceUsedKey:        2.0,
		UpstreamBalanceTotalKey:       5.0,
		UpstreamBalanceKeyCountKey:    1,
		UpstreamBalanceOKCountKey:     1,
		UpstreamBalanceFailedCountKey: 0,
		UpstreamBalanceUpdatedAtKey:   now.Format(time.RFC3339),
		UpstreamBalanceKeysKey:        keyRaw,
	})
	if got == nil {
		t.Fatal("snapshot is nil")
	}
	if got.Available != 3 || got.Used != 2 || got.Total != 5 || got.KeyCount != 1 || got.OKCount != 1 {
		t.Fatalf("snapshot = %+v", got)
	}
	if len(got.Keys) != 1 || got.Keys[0].Masked != "sk...1234" {
		t.Fatalf("keys = %+v", got.Keys)
	}
}

func TestUpstreamBalanceHighRiskDecision(t *testing.T) {
	now := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		opts RealtimeBalanceCheckOptions
		snap *UpstreamBalanceSnapshot
		want bool
	}{
		{
			name: "fresh healthy snapshot is low risk",
			snap: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1, UpdatedAt: &now},
			want: false,
		},
		{
			name: "low available balance is high risk",
			snap: &UpstreamBalanceSnapshot{Available: 0.25, OKCount: 1, UpdatedAt: &now},
			want: true,
		},
		{
			name: "stale snapshot is high risk",
			snap: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1, UpdatedAt: upstreamBalanceTimePtr(now.Add(-2 * time.Hour))},
			want: true,
		},
		{
			name: "last failure is high risk",
			snap: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1, FailedCount: 1, Error: "last refresh failed", UpdatedAt: &now},
			want: true,
		},
		{
			name: "codex long session start is high risk",
			opts: RealtimeBalanceCheckOptions{CodexLongSessionStart: true},
			snap: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1, UpdatedAt: &now},
			want: true,
		},
		{
			name: "missing snapshot is high risk",
			snap: nil,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.Now = now
			if got := isHighRiskRealtimeBalanceSnapshot(tt.snap, opts, 2); got != tt.want {
				t.Fatalf("isHighRiskRealtimeBalanceSnapshot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func assertFloatPtr(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("value = %v, want %v", got, want)
	}
}

func floatPtr(v float64) *float64                   { return &v }
func upstreamBalanceTimePtr(v time.Time) *time.Time { return &v }
