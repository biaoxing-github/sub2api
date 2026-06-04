package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"golang.org/x/sync/errgroup"
)

const (
	UpstreamBalanceAvailableKey   = "upstream_balance_available"
	UpstreamBalanceUsedKey        = "upstream_balance_used"
	UpstreamBalanceTotalKey       = "upstream_balance_total"
	UpstreamBalanceUpdatedAtKey   = "upstream_balance_updated_at"
	UpstreamBalanceKeyCountKey    = "upstream_balance_key_count"
	UpstreamBalanceOKCountKey     = "upstream_balance_ok_count"
	UpstreamBalanceFailedCountKey = "upstream_balance_failed_count"
	UpstreamBalanceErrorKey       = "upstream_balance_error"
	UpstreamBalanceKeysKey        = "upstream_balance_keys"
	UpstreamBalanceGroupsKey      = "upstream_balance_groups"
	UpstreamBalanceConvertedKey   = "upstream_balance_converted_available_by_group"

	newAPIQuotaPerUSD = 500000.0

	UpstreamAuthUsernameCredentialKey       = "upstream_auth_username"
	UpstreamAuthPasswordCredentialKey       = "upstream_auth_password"
	UpstreamAuthTokenCredentialKey          = "upstream_auth_token"
	UpstreamManualRateMultiplierKey         = "upstream_manual_rate_multiplier"
	UpstreamManualRateGroupNameKey          = "upstream_manual_rate_group_name"
	UpstreamCommonRateMultiplierKey         = "upstream_common_rate_multiplier"
	UpstreamCommonRateGroupNameKey          = "upstream_common_rate_group_name"
	UpstreamManualBalanceTotalKey           = "upstream_manual_balance_total"
	UpstreamBalanceEndpointPathsKey         = "upstream_balance_endpoint_paths"
	UpstreamFetchedGroupsKey                = "upstream_fetched_groups"
	UpstreamBalanceAccountEndpoint          = "upstream-login:/api/user/self"
	UpstreamBalanceSubscriptionEndpoint     = "upstream-login:/api/subscription/self"
	UpstreamBalanceSubscriptionItemEndpoint = "upstream-login:/api/subscription/self#subscription"

	defaultUpstreamAuthCacheTTL = 15 * time.Minute
)

type UpstreamBalanceGroupSnapshot struct {
	Name               string   `json:"name"`
	Ratio              float64  `json:"ratio"`
	Description        string   `json:"description,omitempty"`
	ConvertedAvailable *float64 `json:"converted_available,omitempty"`
	ConvertedTotal     *float64 `json:"converted_total,omitempty"`
	ConvertedUsed      *float64 `json:"converted_used,omitempty"`
}

type UpstreamBalanceKeySnapshot struct {
	Fingerprint string                         `json:"fingerprint"`
	Masked      string                         `json:"masked"`
	Available   *float64                       `json:"available,omitempty"`
	Used        *float64                       `json:"used,omitempty"`
	Total       *float64                       `json:"total,omitempty"`
	Status      string                         `json:"status"`
	Error       string                         `json:"error,omitempty"`
	Endpoint    string                         `json:"endpoint,omitempty"`
	UpdatedAt   *time.Time                     `json:"updated_at,omitempty"`
	Groups      []UpstreamBalanceGroupSnapshot `json:"groups,omitempty"`
}

type UpstreamBalanceSnapshot struct {
	Available                 float64                        `json:"available"`
	Used                      float64                        `json:"used"`
	Total                     float64                        `json:"total"`
	KeyCount                  int                            `json:"key_count"`
	OKCount                   int                            `json:"ok_count"`
	FailedCount               int                            `json:"failed_count"`
	UpdatedAt                 *time.Time                     `json:"updated_at,omitempty"`
	Error                     string                         `json:"error,omitempty"`
	Keys                      []UpstreamBalanceKeySnapshot   `json:"keys,omitempty"`
	Groups                    []UpstreamBalanceGroupSnapshot `json:"groups,omitempty"`
	ConvertedAvailableByGroup map[string]float64             `json:"converted_available_by_group,omitempty"`
}

type UpstreamBalanceSummary struct {
	Available                 float64            `json:"available"`
	Used                      float64            `json:"used"`
	Total                     float64            `json:"total"`
	AccountCount              int                `json:"account_count"`
	KeyCount                  int                `json:"key_count"`
	OKKeyCount                int                `json:"ok_key_count"`
	FailedKeyCount            int                `json:"failed_key_count"`
	MissingAccounts           int                `json:"missing_accounts"`
	LatestUpdatedAt           *time.Time         `json:"latest_updated_at,omitempty"`
	OldestUpdatedAt           *time.Time         `json:"oldest_updated_at,omitempty"`
	ConvertedAvailableByGroup map[string]float64 `json:"converted_available_by_group,omitempty"`
}

type UpstreamBalanceRefreshResult struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	MatchedAccounts int                    `json:"matched_accounts"`
	Refreshed       int                    `json:"refreshed"`
	Failed          int                    `json:"failed"`
	Summary         UpstreamBalanceSummary `json:"summary"`
}

type UpstreamBalanceService struct {
	accountRepo  AccountRepository
	httpUpstream HTTPUpstream
	interval     time.Duration
	// activeAccountLimit 限制定时刷新每轮最多触达的上游账号数，0 表示不限制。
	activeAccountLimit int
	// autoRefreshEnabled 控制后台定时刷新是否启动，手动刷新接口不受影响。
	autoRefreshEnabled bool
	// initialRefreshDelay 避免服务启动后立刻打满上游余额接口。
	initialRefreshDelay time.Duration
	stopCh              chan struct{}
	stopOnce            sync.Once
	runMu               sync.Mutex
	// authCacheTTL 控制上游后台登录态的进程内复用时长，避免连续余额刷新反复打登录接口。
	authCacheTTL time.Duration
	authCacheMu  sync.Mutex
	// authCache 只保留运行期登录态；进程重启后重新登录，避免把 cookie 写入数据库。
	authCache map[string]upstreamAuthSession
}

type upstreamAuthContext struct {
	token       string
	cookie      string
	newAPIUser  string
	userBalance *parsedUpstreamBalance
	concurrency *int
	groupsByKey map[string][]UpstreamBalanceGroupSnapshot
	allGroups   []UpstreamBalanceGroupSnapshot
}

type upstreamTokenInfo struct {
	fullKey string
	masked  string
	group   string
}

type upstreamAuthSession struct {
	token      string
	cookie     string
	newAPIUser string
	expiresAt  time.Time
}

func NewUpstreamBalanceService(accountRepo AccountRepository, httpUpstream HTTPUpstream, interval time.Duration) *UpstreamBalanceService {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &UpstreamBalanceService{
		accountRepo:         accountRepo,
		httpUpstream:        httpUpstream,
		interval:            interval,
		autoRefreshEnabled:  true,
		initialRefreshDelay: 90 * time.Second,
		stopCh:              make(chan struct{}),
		authCacheTTL:        defaultUpstreamAuthCacheTTL,
		authCache:           make(map[string]upstreamAuthSession),
	}
}

func ProvideUpstreamBalanceService(accountRepo AccountRepository, httpUpstream HTTPUpstream, cfg *config.Config) *UpstreamBalanceService {
	interval := 30 * time.Minute
	autoRefreshEnabled := true
	activeAccountLimit := 0
	if cfg != nil {
		autoRefreshEnabled = cfg.Gateway.RealtimeBalancePrewarm.Enabled
		if cfg.Gateway.RealtimeBalancePrewarm.IntervalSeconds > 0 {
			interval = time.Duration(cfg.Gateway.RealtimeBalancePrewarm.IntervalSeconds) * time.Second
		}
		activeAccountLimit = cfg.Gateway.RealtimeBalancePrewarm.ActiveAccountLimit
	}
	svc := NewUpstreamBalanceService(accountRepo, httpUpstream, interval)
	svc.autoRefreshEnabled = autoRefreshEnabled
	svc.activeAccountLimit = activeAccountLimit
	svc.Start()
	return svc
}

func (s *UpstreamBalanceService) Start() {
	if s == nil || s.accountRepo == nil || s.httpUpstream == nil || !s.autoRefreshEnabled {
		return
	}
	go func() {
		delay := s.initialRefreshDelay
		if delay <= 0 {
			delay = s.interval
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				if _, err := s.RefreshAll(ctx); err != nil {
					slog.Warn("upstream_balance.refresh_all_failed", "error", err)
				}
				cancel()
				timer.Reset(s.interval)
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *UpstreamBalanceService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
}

func (s *UpstreamBalanceService) RefreshAll(ctx context.Context) (*UpstreamBalanceRefreshResult, error) {
	if s == nil || s.accountRepo == nil {
		return nil, errors.New("upstream balance service is not configured")
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()

	accounts, _, err := s.accountRepo.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 1000, SortBy: "id", SortOrder: "asc"}, PlatformOpenAI, AccountTypeAPIKey, "", "", 0, "", "")
	if err != nil {
		return nil, err
	}
	matchedAccounts := len(accounts)
	if s.activeAccountLimit > 0 && len(accounts) > s.activeAccountLimit {
		accounts = accounts[:s.activeAccountLimit]
	}

	now := time.Now().UTC()
	result := &UpstreamBalanceRefreshResult{GeneratedAt: now, MatchedAccounts: matchedAccounts}
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for i := range accounts {
		account := accounts[i]
		g.Go(func() error {
			snapshot, err := s.RefreshAccount(gctx, &account)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Failed++
				return nil
			}
			result.Refreshed++
			applySnapshotToSummary(&result.Summary, snapshot)
			return nil
		})
	}
	_ = g.Wait()
	return result, nil
}

func (s *UpstreamBalanceService) RefreshOne(ctx context.Context, accountID int64) (*UpstreamBalanceSnapshot, error) {
	if s == nil || s.accountRepo == nil {
		return nil, errors.New("upstream balance service is not configured")
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.RefreshAccount(ctx, account)
}

func (s *UpstreamBalanceService) RefreshAccount(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
	if s == nil || s.accountRepo == nil || s.httpUpstream == nil {
		return nil, errors.New("upstream balance service is not configured")
	}
	if account == nil || account.Type != AccountTypeAPIKey || !account.IsOpenAI() {
		return nil, errors.New("account is not an OpenAI API-key account")
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	keys := allAccountAPIKeys(account)
	if len(keys) == 0 {
		snapshot := UpstreamBalanceSnapshot{Error: "missing api_key"}
		_ = s.accountRepo.UpdateExtra(ctx, account.ID, snapshot.toExtraUpdates(time.Now().UTC()))
		return &snapshot, nil
	}

	now := time.Now().UTC()
	snapshot := &UpstreamBalanceSnapshot{KeyCount: len(keys), Keys: make([]UpstreamBalanceKeySnapshot, 0, len(keys))}
	authCtx := s.fetchUpstreamAuthContext(ctx, account, keys)
	if authCtx != nil && len(authCtx.allGroups) > 0 {
		account.Extra[UpstreamFetchedGroupsKey] = authCtx.allGroups
	}
	for _, key := range keys {
		item := s.fetchKeyBalance(ctx, account, key, authCtx)
		item.UpdatedAt = &now
		snapshot.Keys = append(snapshot.Keys, item)
		if item.Status == "ok" {
			snapshot.OKCount++
			if item.Available != nil {
				snapshot.Available += *item.Available
			}
			if item.Used != nil {
				snapshot.Used += *item.Used
			}
			if item.Total != nil {
				snapshot.Total += *item.Total
			}
			mergeKeyGroupsIntoSnapshot(snapshot, item)
		} else {
			snapshot.FailedCount++
			if snapshot.Error == "" {
				snapshot.Error = item.Error
			}
		}
	}
	snapshot.UpdatedAt = &now
	if snapshot.OKCount == 0 {
		if manual := manualBalanceForAccount(account); manual != nil {
			item := UpstreamBalanceKeySnapshot{
				Fingerprint: "manual:" + strconv.FormatInt(account.ID, 10),
				Masked:      "manual balance",
				Status:      "ok",
				Endpoint:    "manual",
				UpdatedAt:   &now,
				Available:   manual.Available,
				Used:        manual.Used,
				Total:       manual.Total,
				Groups:      groupsForAccount(authCtx, account),
			}
			applyConvertedBalancesToGroups(&item)
			snapshot.Keys = append(snapshot.Keys, item)
			snapshot.OKCount = 1
			snapshot.FailedCount = len(keys)
			if item.Available != nil {
				snapshot.Available = *item.Available
			}
			if item.Used != nil {
				snapshot.Used = *item.Used
			}
			if item.Total != nil {
				snapshot.Total = *item.Total
			}
			mergeKeyGroupsIntoSnapshot(snapshot, item)
		}
	}
	if snapshot.OKCount == 0 && authCtx != nil && authCtx.userBalance != nil {
		item := UpstreamBalanceKeySnapshot{
			Fingerprint: "account:" + strconv.FormatInt(account.ID, 10),
			Masked:      "upstream account",
			Status:      "ok",
			Endpoint:    UpstreamBalanceAccountEndpoint,
			UpdatedAt:   &now,
			Available:   authCtx.userBalance.Available,
			Used:        authCtx.userBalance.Used,
			Total:       authCtx.userBalance.Total,
			Groups:      groupsForAccount(authCtx, account),
		}
		applyConvertedBalancesToGroups(&item)
		snapshot.Keys = append(snapshot.Keys, item)
		snapshot.OKCount = 1
		snapshot.FailedCount = len(keys)
		if item.Available != nil {
			snapshot.Available = *item.Available
		}
		if item.Used != nil {
			snapshot.Used = *item.Used
		}
		if item.Total != nil {
			snapshot.Total = *item.Total
		}
		mergeKeyGroupsIntoSnapshot(snapshot, item)
	}
	if snapshot.OKCount > 0 {
		snapshot.Error = ""
	}
	preserveManualGroups(snapshot, account)
	updates := snapshot.toExtraUpdates(now)
	if authCtx != nil && len(authCtx.allGroups) > 0 {
		setFetchedRateGroupUpdates(updates, authCtx.allGroups)
	} else if len(snapshot.Groups) > 0 && len(manualRateGroups(account)) == 0 {
		setFetchedRateGroupUpdates(updates, snapshot.Groups)
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
		return nil, err
	}
	if authCtx != nil && authCtx.concurrency != nil && *authCtx.concurrency != account.Concurrency {
		if _, err := s.accountRepo.BulkUpdate(ctx, []int64{account.ID}, AccountBulkUpdate{Concurrency: authCtx.concurrency}); err != nil {
			return nil, err
		}
		account.Concurrency = *authCtx.concurrency
	}
	return snapshot, nil
}

func (s *UpstreamBalanceService) fetchKeyBalance(ctx context.Context, account *Account, apiKey string, authCtx *upstreamAuthContext) UpstreamBalanceKeySnapshot {
	item := UpstreamBalanceKeySnapshot{Fingerprint: FingerprintAPIKey(apiKey), Masked: maskUpstreamAPIKey(apiKey), Status: "error"}
	if balance := balanceForKeyFromAuthContext(authCtx, len(allAccountAPIKeys(account))); balance != nil {
		item.Available = balance.Available
		item.Used = balance.Used
		item.Total = balance.Total
		item.Endpoint = UpstreamBalanceAccountEndpoint
		item.Groups = groupsForKey(authCtx, account, apiKey)
		applyConvertedBalancesToGroups(&item)
		item.Status = "ok"
		item.Error = ""
		return item
	}
	baseURL := upstreamBalanceBaseURL(account)
	endpoints := upstreamBalanceEndpointsForAccount(account, baseURL)
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	var lastErr string
	for _, endpoint := range endpoints {
		reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			lastErr = err.Error()
			continue
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "sub2api-upstream-balance/1.0")
		resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
		if err != nil {
			cancel()
			lastErr = err.Error()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			lastErr = readErr.Error()
			continue
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			lastErr = fmt.Sprintf("%s returned %d", endpoint, resp.StatusCode)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Sprintf("%s returned %d: %s", endpoint, resp.StatusCode, trimErrorBody(body))
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				break
			}
			continue
		}
		balance, err := ParseUpstreamBalanceResponse(body)
		if err != nil {
			lastErr = fmt.Sprintf("%s parse failed: %s", endpoint, err.Error())
			continue
		}
		item.Available = balance.Available
		item.Used = balance.Used
		item.Total = balance.Total
		item.Endpoint = endpoint
		item.Groups = manualRateGroups(account)
		if len(item.Groups) == 0 {
			item.Groups = groupsFromBalanceResponse(body, apiKey, len(allAccountAPIKeys(account)) == 1)
			if len(item.Groups) == 0 {
				item.Groups = balance.Groups
			}
			if len(item.Groups) == 0 {
				item.Groups = groupsForKey(authCtx, account, apiKey)
			}
			if len(item.Groups) == 0 {
				item.Groups = s.fetchKeyGroups(ctx, account, apiKey, baseURL)
			}
		}
		applyConvertedBalancesToGroups(&item)
		item.Status = "ok"
		item.Error = ""
		return item
	}
	if authCtx != nil {
		if balance := balanceForKeyFromAuthContext(authCtx, len(allAccountAPIKeys(account))); balance != nil {
			item.Available = balance.Available
			item.Used = balance.Used
			item.Total = balance.Total
			item.Endpoint = UpstreamBalanceAccountEndpoint
			item.Groups = groupsForKey(authCtx, account, apiKey)
			applyConvertedBalancesToGroups(&item)
			item.Status = "ok"
			item.Error = ""
			return item
		}
		if groups := groupsForKey(authCtx, account, apiKey); len(groups) > 0 {
			item.Groups = groups
		}
	}
	if lastErr == "" {
		lastErr = "no balance endpoint matched"
	}
	item.Error = lastErr
	return item
}

type parsedUpstreamBalance struct {
	Available *float64
	Used      *float64
	Total     *float64
	Groups    []UpstreamBalanceGroupSnapshot
}

func (s *UpstreamBalanceService) fetchUpstreamAuthContext(ctx context.Context, account *Account, keys []string) *upstreamAuthContext {
	if s == nil || s.httpUpstream == nil || account == nil {
		return nil
	}
	username := strings.TrimSpace(account.GetCredential(UpstreamAuthUsernameCredentialKey))
	password := strings.TrimSpace(account.GetCredential(UpstreamAuthPasswordCredentialKey))
	if username == "" || password == "" {
		return nil
	}
	baseURL := upstreamBalanceBaseURL(account)
	cacheKey := upstreamAuthCacheKey(account, baseURL, username)
	session, fromCache := s.cachedUpstreamAuthSession(cacheKey)
	if !fromCache {
		var err error
		session, err = s.loginUpstreamSession(ctx, account, baseURL, username, password)
		if err != nil {
			slog.Warn("upstream_balance.login_failed", "account_id", account.ID, "error", err)
			return nil
		}
		s.storeUpstreamAuthSession(cacheKey, session)
	}
	auth := s.fetchUpstreamAuthContextWithSession(ctx, account, keys, baseURL, session)
	if fromCache && auth.userBalance == nil {
		s.deleteUpstreamAuthSession(cacheKey)
		freshSession, err := s.loginUpstreamSession(ctx, account, baseURL, username, password)
		if err != nil {
			slog.Warn("upstream_balance.login_failed", "account_id", account.ID, "error", err)
			return auth
		}
		s.storeUpstreamAuthSession(cacheKey, freshSession)
		auth = s.fetchUpstreamAuthContextWithSession(ctx, account, keys, baseURL, freshSession)
	}
	return auth
}

func (s *UpstreamBalanceService) fetchUpstreamAuthContextWithSession(ctx context.Context, account *Account, keys []string, baseURL string, session upstreamAuthSession) *upstreamAuthContext {
	auth := &upstreamAuthContext{token: session.token, cookie: session.cookie, newAPIUser: session.newAPIUser}
	auth.userBalance, auth.concurrency = s.fetchAuthenticatedAuthMe(ctx, account, baseURL, auth)
	if auth.userBalance == nil {
		if balance := s.fetchAuthenticatedAccountBalance(ctx, account, baseURL, auth); balance != nil {
			auth.userBalance = balance
		}
	}
	auth.allGroups = s.fetchAuthenticatedGroups(ctx, account, baseURL, auth)
	auth.groupsByKey = s.fetchAuthenticatedKeyGroups(ctx, account, baseURL, auth, keys, auth.allGroups)
	return auth
}

func upstreamAuthCacheKey(account *Account, baseURL, username string) string {
	if account == nil {
		return ""
	}
	return fmt.Sprintf("%d|%s|%s|%d", account.ID, baseURL, username, account.UpdatedAt.UnixNano())
}

func (s *UpstreamBalanceService) cachedUpstreamAuthSession(key string) (upstreamAuthSession, bool) {
	if s == nil || key == "" {
		return upstreamAuthSession{}, false
	}
	now := time.Now()
	s.authCacheMu.Lock()
	defer s.authCacheMu.Unlock()
	session, ok := s.authCache[key]
	if !ok || !session.expiresAt.After(now) {
		if ok {
			delete(s.authCache, key)
		}
		return upstreamAuthSession{}, false
	}
	return session, true
}

func (s *UpstreamBalanceService) storeUpstreamAuthSession(key string, session upstreamAuthSession) {
	if s == nil || key == "" {
		return
	}
	ttl := s.authCacheTTL
	if ttl <= 0 {
		ttl = defaultUpstreamAuthCacheTTL
	}
	session.expiresAt = time.Now().Add(ttl)
	s.authCacheMu.Lock()
	defer s.authCacheMu.Unlock()
	if s.authCache == nil {
		s.authCache = make(map[string]upstreamAuthSession)
	}
	s.authCache[key] = session
}

func (s *UpstreamBalanceService) deleteUpstreamAuthSession(key string) {
	if s == nil || key == "" {
		return
	}
	s.authCacheMu.Lock()
	defer s.authCacheMu.Unlock()
	delete(s.authCache, key)
}

func (s *UpstreamBalanceService) loginUpstreamSession(ctx context.Context, account *Account, baseURL, username, password string) (upstreamAuthSession, error) {
	token, cookie, newAPIUser, err := s.loginUpstream(ctx, account, baseURL, username, password)
	if err != nil {
		return upstreamAuthSession{}, err
	}
	return upstreamAuthSession{token: token, cookie: cookie, newAPIUser: newAPIUser}, nil
}

func (s *UpstreamBalanceService) loginUpstream(ctx context.Context, account *Account, baseURL, username, password string) (token string, cookie string, newAPIUser string, err error) {
	payloads := []struct {
		endpoint string
		body     map[string]string
	}{
		{baseURL + "/api/user/login", map[string]string{"username": username, "password": password}},
		{baseURL + "/api/v1/auth/login", map[string]string{"email": username, "password": password}},
	}
	var lastErr string
	for _, payload := range payloads {
		body, _ := json.Marshal(payload.body)
		status, respBody, headers, err := s.doUpstreamJSON(ctx, account, http.MethodPost, payload.endpoint, body, "", "", "")
		if err != nil {
			lastErr = err.Error()
			continue
		}
		if status < 200 || status >= 300 {
			lastErr = fmt.Sprintf("%s returned %d: %s", payload.endpoint, status, trimErrorBody(respBody))
			if shouldStopUpstreamLoginFallback(status) {
				break
			}
			continue
		}
		token = extractAuthToken(respBody)
		cookie = joinSetCookies(headers.Values("Set-Cookie"))
		newAPIUser = extractNewAPIUserID(respBody)
		if token != "" || cookie != "" {
			return token, cookie, newAPIUser, nil
		}
		lastErr = payload.endpoint + " did not return token or cookie"
	}
	if lastErr == "" {
		lastErr = "no upstream login endpoint matched"
	}
	return "", "", "", errors.New(lastErr)
}

func shouldStopUpstreamLoginFallback(status int) bool {
	return status != http.StatusNotFound && status != http.StatusMethodNotAllowed
}

func (s *UpstreamBalanceService) fetchAuthenticatedAuthMe(ctx context.Context, account *Account, baseURL string, auth *upstreamAuthContext) (*parsedUpstreamBalance, *int) {
	status, body, _, err := s.doUpstreamJSON(ctx, account, http.MethodGet, baseURL+"/api/v1/auth/me", nil, auth.token, auth.cookie, auth.newAPIUser)
	if err != nil || status < 200 || status >= 300 {
		return nil, nil
	}
	balance, _ := parseNewAPIUserSelfResponse(body)
	concurrency := parseAuthMeConcurrency(body)
	return balance, concurrency
}

func (s *UpstreamBalanceService) fetchAuthenticatedAccountBalance(ctx context.Context, account *Account, baseURL string, auth *upstreamAuthContext) *parsedUpstreamBalance {
	var out *parsedUpstreamBalance
	var lastErr string
	endpoints := []string{baseURL + "/api/user/self"}
	for _, path := range upstreamBalanceEndpointPaths(account) {
		if isAuthenticatedGroupEndpointPath(path) {
			endpoints = append(endpoints, joinUpstreamEndpoint(baseURL, path))
		}
	}
	for _, endpoint := range uniqueStrings(endpoints) {
		status, body, _, err := s.doUpstreamJSON(ctx, account, http.MethodGet, endpoint, nil, auth.token, auth.cookie, auth.newAPIUser)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		if status < 200 || status >= 300 {
			lastErr = fmt.Sprintf("%s returned %d: %s", endpoint, status, trimErrorBody(body))
			continue
		}
		balance, err := parseNewAPIUsageResponse(body)
		if err != nil {
			balance, err = parseNewAPIUserSelfResponse(body)
		}
		if err != nil {
			balance, err = ParseUpstreamBalanceResponse(body)
		}
		if err != nil {
			lastErr = err.Error()
			continue
		}
		out = balance
		break
	}
	if sub := s.fetchAuthenticatedSubscriptionBalance(ctx, account, baseURL, auth); sub != nil {
		out = addParsedBalances(out, sub)
	}
	if out == nil && lastErr != "" {
		slog.Warn("upstream_balance.authenticated_balance_failed", "account_id", account.ID, "error", lastErr)
	}
	return out
}

func (s *UpstreamBalanceService) fetchAuthenticatedSubscriptionBalance(ctx context.Context, account *Account, baseURL string, auth *upstreamAuthContext) *parsedUpstreamBalance {
	status, body, _, err := s.doUpstreamJSON(ctx, account, http.MethodGet, baseURL+"/api/subscription/self", nil, auth.token, auth.cookie, auth.newAPIUser)
	if err != nil || status < 200 || status >= 300 {
		return nil
	}
	balance, err := parseSubscriptionSelfResponse(body)
	if err != nil {
		return nil
	}
	return balance
}

func (s *UpstreamBalanceService) fetchAuthenticatedGroups(ctx context.Context, account *Account, baseURL string, auth *upstreamAuthContext) []UpstreamBalanceGroupSnapshot {
	endpoints := []string{
		baseURL + "/api/v1/groups/available",
		baseURL + "/api/v1/groups/rates",
		baseURL + "/api/user/groups",
		baseURL + "/api/pricing",
	}
	for _, path := range upstreamBalanceEndpointPaths(account) {
		if isAuthenticatedGroupEndpointPath(path) {
			endpoints = append(endpoints, joinUpstreamEndpoint(baseURL, path))
		}
	}
	var groups []UpstreamBalanceGroupSnapshot
	seen := make(map[string]struct{})
	for _, endpoint := range uniqueStrings(endpoints) {
		status, body, _, err := s.doUpstreamJSON(ctx, account, http.MethodGet, endpoint, nil, auth.token, auth.cookie, auth.newAPIUser)
		if err != nil || status < 200 || status >= 300 {
			continue
		}
		for _, group := range ParseUpstreamGroupsResponse(body) {
			if group.Name == "" || group.Ratio <= 0 {
				continue
			}
			if _, ok := seen[group.Name]; ok {
				continue
			}
			seen[group.Name] = struct{}{}
			groups = append(groups, group)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func (s *UpstreamBalanceService) fetchAuthenticatedKeyGroups(ctx context.Context, account *Account, baseURL string, auth *upstreamAuthContext, keys []string, allGroups []UpstreamBalanceGroupSnapshot) map[string][]UpstreamBalanceGroupSnapshot {
	if len(keys) == 0 {
		return nil
	}
	groupRatio := make(map[string]UpstreamBalanceGroupSnapshot, len(allGroups))
	for _, group := range allGroups {
		groupRatio[group.Name] = group
	}
	out := make(map[string][]UpstreamBalanceGroupSnapshot)
	for _, endpoint := range uniqueStrings([]string{baseURL + "/api/v1/keys?page=1&page_size=1000", baseURL + "/api/token/?page=1&page_size=1000"}) {
		status, body, _, err := s.doUpstreamJSON(ctx, account, http.MethodGet, endpoint, nil, auth.token, auth.cookie, auth.newAPIUser)
		if err != nil || status < 200 || status >= 300 {
			continue
		}
		for _, tokenInfo := range ParseUpstreamTokenListResponse(body) {
			if tokenInfo.group == "" {
				continue
			}
			group, ok := groupRatio[tokenInfo.group]
			if !ok {
				group = UpstreamBalanceGroupSnapshot{Name: tokenInfo.group, Ratio: 1}
			}
			for _, key := range keys {
				if upstreamTokenMatchesKey(tokenInfo, key) {
					out[FingerprintAPIKey(key)] = []UpstreamBalanceGroupSnapshot{group}
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *UpstreamBalanceService) doUpstreamJSON(ctx context.Context, account *Account, method, endpoint string, body []byte, token, cookie, newAPIUser string) (int, []byte, http.Header, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, endpoint, reader)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-upstream-balance/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if newAPIUser != "" {
		req.Header.Set("New-Api-User", newAPIUser)
	}
	proxyURL := ""
	if account != nil && account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return 0, nil, nil, err
	}
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return resp.StatusCode, nil, resp.Header, readErr
	}
	return resp.StatusCode, respBody, resp.Header, nil
}

func (s *UpstreamBalanceService) fetchKeyGroups(ctx context.Context, account *Account, apiKey string, baseURL string) []UpstreamBalanceGroupSnapshot {
	if s == nil || s.httpUpstream == nil {
		return nil
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	for _, endpoint := range upstreamGroupEndpoints(baseURL) {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "sub2api-upstream-balance/1.0")
		resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
		if err != nil {
			cancel()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		cancel()
		if readErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		groups := ParseUpstreamGroupsResponse(body)
		if len(groups) > 0 {
			return groups
		}
	}
	return nil
}

func ParseUpstreamBalanceResponse(body []byte) (*parsedUpstreamBalance, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("response is not an object")
	}
	if isNewAPITokenUsageResponse(obj) {
		return parseNewAPITokenUsageResponse(obj)
	}
	if balance, err := parseNewAPIUsageResponse(body); err == nil {
		balance.Groups = parseUsageItemGroups(obj)
		return balance, nil
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(obj["object"])), "billing_subscription") {
		return parseBillingSubscriptionResponse(obj)
	}
	candidates := []map[string]any{obj}
	for _, key := range []string{"data", "quota", "balance", "result"} {
		if nested, ok := obj[key].(map[string]any); ok {
			candidates = append(candidates, nested)
		}
	}
	out := &parsedUpstreamBalance{}
	for _, c := range candidates {
		if out.Available == nil {
			out.Available = firstFloatPtr(c, "total_available", "available", "available_balance", "remain", "remaining", "balance", "credit")
		}
		if out.Used == nil {
			out.Used = firstFloatPtr(c, "total_used", "used", "used_amount", "consumed", "usage")
		}
		if out.Total == nil {
			out.Total = firstFloatPtr(c, "total_granted", "total", "limit", "quota_limit", "quota", "granted", "amount")
		}
	}
	if out.Total == nil && out.Available != nil && out.Used != nil {
		total := *out.Available + *out.Used
		out.Total = &total
	}
	if out.Available == nil && out.Total != nil && out.Used != nil {
		available := *out.Total - *out.Used
		out.Available = &available
	}
	if out.Available == nil && out.Used == nil && out.Total == nil {
		return nil, errors.New("no balance fields found")
	}
	return out, nil
}

func parseNewAPIUserSelfResponse(body []byte) (*parsedUpstreamBalance, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("response is not an object")
	}
	data, ok := obj["data"].(map[string]any)
	if !ok {
		data = obj
	}
	candidates := []map[string]any{data}
	if user, ok := data["user"].(map[string]any); ok {
		candidates = append([]map[string]any{user}, candidates...)
	}
	for _, candidate := range candidates {
		quota, ok := parseAnyFloat(candidate["quota"])
		if !ok {
			continue
		}
		used, _ := parseAnyFloat(candidate["used_quota"])
		available := quota / newAPIQuotaPerUSD
		usedUSD := used / newAPIQuotaPerUSD
		total := available + usedUSD
		return &parsedUpstreamBalance{Available: &available, Used: &usedUSD, Total: &total}, nil
	}
	for _, candidate := range candidates {
		available := firstFloatPtr(candidate, "balance", "available_balance", "available", "remain", "remaining")
		if available == nil {
			continue
		}
		out := &parsedUpstreamBalance{Available: available}
		if used := firstFloatPtr(candidate, "used", "used_amount", "usage"); used != nil {
			out.Used = used
		}
		if total := firstFloatPtr(candidate, "total", "quota_limit", "amount"); total != nil {
			out.Total = total
		}
		if out.Total == nil && out.Used != nil {
			total := *out.Available + *out.Used
			out.Total = &total
		}
		if out.Total == nil {
			total := *out.Available
			out.Total = &total
		}
		return out, nil
	}
	return nil, errors.New("no user balance field found")
}

func parseAuthMeConcurrency(body []byte) *int {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	data, ok := obj["data"].(map[string]any)
	if !ok {
		data = obj
	}
	value, ok := parseAnyFloat(data["concurrency"])
	if !ok || value < 0 || value != float64(int(value)) {
		return nil
	}
	concurrency := int(value)
	return &concurrency
}

func parseNewAPIUsageResponse(body []byte) (*parsedUpstreamBalance, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("response is not an object")
	}
	user := firstObjectAtPath(obj, []string{"data", "user"}, []string{"user"})
	if user == nil {
		return nil, errors.New("no usage user object found")
	}
	out := &parsedUpstreamBalance{}
	if v := firstFloatPtr(user, "balance", "available_balance", "available", "remain", "remaining"); v != nil {
		out.Available = v
	}
	if v := firstFloatPtr(user, "used", "used_amount", "usage"); v != nil {
		out.Used = v
	}
	if v := firstFloatPtr(user, "total", "quota", "quota_limit", "amount"); v != nil {
		out.Total = v
	}
	if out.Total == nil && out.Available != nil && out.Used != nil {
		total := *out.Available + *out.Used
		out.Total = &total
	}
	if out.Available == nil && out.Total != nil && out.Used != nil {
		available := *out.Total - *out.Used
		out.Available = &available
	}
	if out.Available == nil && out.Used == nil && out.Total == nil {
		return nil, errors.New("no usage balance fields found")
	}
	return out, nil
}

func parseBillingSubscriptionResponse(obj map[string]any) (*parsedUpstreamBalance, error) {
	limit := firstFloatPtr(obj, "hard_limit_usd", "soft_limit_usd", "system_hard_limit_usd")
	if limit == nil {
		return nil, errors.New("no billing subscription limit fields found")
	}
	if *limit >= 100000000 {
		return nil, errors.New("billing subscription exposes an unlimited compatibility limit")
	}
	return &parsedUpstreamBalance{Available: limit, Total: limit}, nil
}

func isNewAPITokenUsageResponse(obj map[string]any) bool {
	data, ok := obj["data"].(map[string]any)
	if !ok {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(data["object"])), "token_usage") {
		return true
	}
	_, hasUnlimited := data["unlimited_quota"]
	_, hasAvailable := data["total_available"]
	return hasUnlimited && hasAvailable
}

func parseNewAPITokenUsageResponse(obj map[string]any) (*parsedUpstreamBalance, error) {
	data, _ := obj["data"].(map[string]any)
	if unlimited, ok := data["unlimited_quota"].(bool); ok && unlimited {
		available, _ := parseAnyFloat(data["total_available"])
		total, _ := parseAnyFloat(data["total_granted"])
		if available <= 0 && total <= 0 {
			return nil, errors.New("NewAPI token usage is unlimited and does not expose a finite balance")
		}
	}
	out := &parsedUpstreamBalance{}
	if v := firstFloatPtr(data, "total_available", "remain_quota"); v != nil {
		value := *v / newAPIQuotaPerUSD
		out.Available = &value
	}
	if v := firstFloatPtr(data, "total_used", "used_quota"); v != nil {
		value := *v / newAPIQuotaPerUSD
		out.Used = &value
	}
	if v := firstFloatPtr(data, "total_granted", "total", "quota"); v != nil {
		value := *v / newAPIQuotaPerUSD
		out.Total = &value
	}
	if out.Available == nil && out.Used == nil && out.Total == nil {
		return nil, errors.New("no NewAPI token usage fields found")
	}
	return out, nil
}

func ParseUpstreamGroupsResponse(body []byte) []UpstreamBalanceGroupSnapshot {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	if groups := parseUsageItemGroups(raw); len(groups) > 0 {
		return groups
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	data := firstObjectAtPath(obj, []string{"data", "group_ratio"}, []string{"data"}, []string{"group_ratio"})
	if data == nil {
		data = obj
	}
	groups := make([]UpstreamBalanceGroupSnapshot, 0, len(data))
	for name, value := range data {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if ratio, ok := parseAnyFloat(value); ok && ratio > 0 {
			groups = append(groups, UpstreamBalanceGroupSnapshot{Name: name, Ratio: ratio})
			continue
		}
		m, ok := value.(map[string]any)
		if !ok {
			continue
		}
		ratio, ok := parseAnyFloat(m["ratio"])
		if !ok || ratio <= 0 {
			continue
		}
		groups = append(groups, UpstreamBalanceGroupSnapshot{
			Name:        name,
			Ratio:       ratio,
			Description: strings.TrimSpace(fmt.Sprint(m["desc"])),
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func parseUsageItemGroups(raw any) []UpstreamBalanceGroupSnapshot {
	items := findObjectListAtKeys(raw, "items")
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]UpstreamBalanceGroupSnapshot)
	for _, item := range items {
		group, ok := usageItemGroup(item)
		if !ok {
			continue
		}
		if existing, ok := seen[group.Name]; ok && existing.Ratio > 0 {
			continue
		}
		seen[group.Name] = group
	}
	if len(seen) == 0 {
		return nil
	}
	groups := make([]UpstreamBalanceGroupSnapshot, 0, len(seen))
	for _, group := range seen {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func groupsFromBalanceResponse(body []byte, apiKey string, singleKey bool) []UpstreamBalanceGroupSnapshot {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	if groups := usageItemGroupsForKey(raw, apiKey); len(groups) > 0 {
		return groups
	}
	if singleKey {
		return parseUsageItemGroups(raw)
	}
	return nil
}

func usageItemGroupsForKey(raw any, apiKey string) []UpstreamBalanceGroupSnapshot {
	items := findObjectListAtKeys(raw, "items")
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]UpstreamBalanceGroupSnapshot)
	for _, item := range items {
		if !usageItemMatchesAPIKey(item, apiKey) {
			continue
		}
		group, ok := usageItemGroup(item)
		if !ok {
			continue
		}
		seen[group.Name] = group
	}
	if len(seen) == 0 {
		return nil
	}
	groups := make([]UpstreamBalanceGroupSnapshot, 0, len(seen))
	for _, group := range seen {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func usageItemMatchesAPIKey(item map[string]any, apiKey string) bool {
	candidates := tokenCandidatesFromObject(item)
	if apiKeyObj, ok := item["api_key"].(map[string]any); ok {
		candidates = append(candidates, tokenCandidatesFromObject(apiKeyObj)...)
	}
	if tokenObj, ok := item["token"].(map[string]any); ok {
		candidates = append(candidates, tokenCandidatesFromObject(tokenObj)...)
	}
	for _, candidate := range candidates {
		if upstreamTokenMatchesKey(upstreamTokenInfo{fullKey: candidate, masked: candidate}, apiKey) {
			return true
		}
	}
	return false
}

func tokenCandidatesFromObject(item map[string]any) []string {
	fields := []string{
		"key", "token", "api_key", "apiKey", "apikey",
		"masked_key", "masked", "key_masked", "token_key",
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if value := firstString(item, field); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func usageItemGroup(item map[string]any) (UpstreamBalanceGroupSnapshot, bool) {
	groupObj, _ := item["group"].(map[string]any)
	name := ""
	if groupObj != nil {
		name = strings.TrimSpace(firstString(groupObj, "name", "group_name"))
	}
	if name == "" {
		name = strings.TrimSpace(firstString(item, "group_name", "group"))
	}
	if name == "" {
		return UpstreamBalanceGroupSnapshot{}, false
	}
	var ratio float64
	var ok bool
	if groupObj != nil {
		ratio, ok = parseAnyFloat(groupObj["rate_multiplier"])
		if !ok || ratio <= 0 {
			ratio, ok = parseAnyFloat(groupObj["ratio"])
		}
	}
	if !ok || ratio <= 0 {
		ratio, ok = parseAnyFloat(item["rate_multiplier"])
	}
	if !ok || ratio <= 0 {
		ratio = 1
	}
	description := ""
	if groupObj != nil {
		description = strings.TrimSpace(firstString(groupObj, "description", "desc"))
	}
	return UpstreamBalanceGroupSnapshot{Name: name, Ratio: ratio, Description: description}, true
}

func ParseUpstreamTokenListResponse(body []byte) []upstreamTokenInfo {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	items := findObjectList(raw)
	out := make([]upstreamTokenInfo, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(firstString(item, "key", "token", "api_key", "apiKey", "apikey", "token_key"))
		masked := strings.TrimSpace(firstString(item, "masked_key", "masked", "key_masked"))
		group := strings.TrimSpace(firstString(item, "group", "group_name"))
		if key == "" || group == "" {
			if apiKeyObj, ok := item["api_key"].(map[string]any); ok {
				if key == "" {
					key = strings.TrimSpace(firstString(apiKeyObj, "key", "token", "api_key", "apiKey", "apikey", "token_key"))
				}
				if masked == "" {
					masked = strings.TrimSpace(firstString(apiKeyObj, "masked_key", "masked", "key_masked"))
				}
				if group == "" {
					group = strings.TrimSpace(firstString(apiKeyObj, "group", "group_name"))
				}
			}
		}
		if group == "" {
			if g, ok := item["group"].(map[string]any); ok {
				group = strings.TrimSpace(firstString(g, "name"))
			}
		}
		if key == "" && masked == "" {
			continue
		}
		out = append(out, upstreamTokenInfo{fullKey: key, masked: masked, group: group})
	}
	return out
}

func UpstreamBalanceSnapshotFromExtra(extra map[string]any) *UpstreamBalanceSnapshot {
	if len(extra) == 0 {
		return nil
	}
	updatedRaw := strings.TrimSpace(fmt.Sprint(extra[UpstreamBalanceUpdatedAtKey]))
	if updatedRaw == "" || updatedRaw == "<nil>" {
		return nil
	}
	updatedAt, _ := parseTime(updatedRaw)
	updatedUTC := updatedAt.UTC()
	s := &UpstreamBalanceSnapshot{
		Available:   parseExtraFloat64(extra[UpstreamBalanceAvailableKey]),
		Used:        parseExtraFloat64(extra[UpstreamBalanceUsedKey]),
		Total:       parseExtraFloat64(extra[UpstreamBalanceTotalKey]),
		KeyCount:    int(parseExtraFloat64(extra[UpstreamBalanceKeyCountKey])),
		OKCount:     int(parseExtraFloat64(extra[UpstreamBalanceOKCountKey])),
		FailedCount: int(parseExtraFloat64(extra[UpstreamBalanceFailedCountKey])),
		Error:       strings.TrimSpace(fmt.Sprint(extra[UpstreamBalanceErrorKey])),
		UpdatedAt:   &updatedUTC,
	}
	if s.Error == "<nil>" {
		s.Error = ""
	}
	if rawKeys, ok := extra[UpstreamBalanceKeysKey].([]any); ok {
		s.Keys = parseUpstreamBalanceKeySnapshots(rawKeys)
	}
	if rawGroups, ok := extra[UpstreamBalanceGroupsKey].([]any); ok {
		s.Groups = parseUpstreamBalanceGroups(rawGroups)
	}
	if len(s.Groups) == 0 {
		if rawGroups, ok := extra[UpstreamFetchedGroupsKey].([]any); ok {
			s.Groups = parseUpstreamBalanceGroups(rawGroups)
		}
	}
	if rawConverted, ok := extra[UpstreamBalanceConvertedKey].(map[string]any); ok {
		s.ConvertedAvailableByGroup = parseStringFloatMap(rawConverted)
	}
	return s
}

func allAccountAPIKeys(account *Account) []string {
	if account == nil {
		return nil
	}
	keys := account.GetAPIKeys()
	if len(keys) == 0 {
		if key := strings.TrimSpace(account.GetCredential("api_key")); key != "" {
			keys = []string{key}
		}
	}
	return keys
}

func upstreamBalanceBaseURL(account *Account) string {
	baseURL := strings.TrimSpace(account.GetOpenAIBalanceBaseURL())
	if baseURL == "" {
		baseURL = strings.TrimSpace(account.GetCredential("base_url"))
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return baseURL
}

func upstreamBalanceEndpoints(baseURL string) []string {
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return upstreamBalanceEndpointsFromPaths(baseURL, defaultUpstreamBalanceEndpointPaths())
}

func upstreamBalanceEndpointsForAccount(account *Account, baseURL string) []string {
	return upstreamBalanceEndpointsFromPaths(baseURL, upstreamBalanceEndpointPaths(account))
}

func upstreamBalanceEndpointsFromPaths(baseURL string, paths []string) []string {
	endpoints := make([]string, 0, len(paths))
	for _, path := range paths {
		endpoints = append(endpoints, joinUpstreamEndpoint(baseURL, path))
	}
	return uniqueStrings(endpoints)
}

func defaultUpstreamBalanceEndpointPaths() []string {
	return []string{
		"/v1/usage",
		"/v1/dashboard/billing/subscription",
		"/dashboard/billing/subscription",
		"/api/usage/token/",
		"/dashboard/billing/credit_grants",
		"/v1/dashboard/billing/credit_grants",
		"/api/user/self",
		"/api/user/self/stat",
		"/api/user/self/usage",
		"/api/user/balance",
		"/user/balance",
		"/balance",
		"/api/v1/usage",
	}
}

func upstreamBalanceEndpointPaths(account *Account) []string {
	defaults := defaultUpstreamBalanceEndpointPaths()
	var paths []string
	if account != nil {
		if account.Credentials != nil {
			paths = append(paths, parseStringList(account.Credentials[UpstreamBalanceEndpointPathsKey])...)
		}
		if account.Extra != nil {
			paths = append(paths, parseStringList(account.Extra[UpstreamBalanceEndpointPathsKey])...)
		}
	}
	if len(paths) == 0 {
		paths = defaults
	}
	return uniqueStrings(normalizeEndpointPaths(paths))
}

func parseStringList(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var parsed []string
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parsed
		}
		parts := strings.FieldsFunc(v, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' })
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeEndpointPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			if u, err := url.Parse(path); err == nil {
				path = u.EscapedPath()
				if u.RawQuery != "" {
					path += "?" + u.RawQuery
				}
			}
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		out = append(out, path)
	}
	return out
}

func joinUpstreamEndpoint(baseURL, path string) string {
	baseURL = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1"), "/")
	path = strings.TrimSpace(path)
	if path == "" {
		return baseURL
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

func isAuthenticatedGroupEndpointPath(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if u, err := url.Parse(path); err == nil {
			path = strings.ToLower(u.EscapedPath())
		}
	}
	return path == "/api/v1/usage"
}

func upstreamGroupEndpoints(baseURL string) []string {
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return uniqueStrings([]string{
		baseURL + "/api/user/groups",
		baseURL + "/api/user/self/groups",
		baseURL + "/api/ratio_config",
	})
}

func extractAuthToken(body []byte) string {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	return strings.TrimSpace(findStringByKeys(raw, "access_token", "token", "jwt"))
}

func extractNewAPIUserID(body []byte) string {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	if root, ok := raw.(map[string]any); ok {
		if data, ok := root["data"].(map[string]any); ok {
			for _, key := range []string{"id", "user_id"} {
				if s := firstString(data, key); s != "" {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	return strings.TrimSpace(findStringByKeys(raw, "id", "user_id"))
}

func joinSetCookies(values []string) string {
	if len(values) == 0 {
		return ""
	}
	cookies := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		cookies = append(cookies, strings.Split(value, ";")[0])
	}
	return strings.Join(cookies, "; ")
}

func parseSubscriptionSelfResponse(body []byte) (*parsedUpstreamBalance, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	items := findObjectListAtKeys(raw, "subscriptions", "all_subscriptions")
	var out *parsedUpstreamBalance
	for _, item := range items {
		sub, ok := item["subscription"].(map[string]any)
		if !ok {
			sub = item
		}
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(sub["status"])))
		if status != "" && status != "active" {
			continue
		}
		totalUnits, hasTotal := parseAnyFloat(sub["amount_total"])
		usedUnits, _ := parseAnyFloat(sub["amount_used"])
		if !hasTotal || totalUnits <= 0 {
			continue
		}
		total := totalUnits / newAPIQuotaPerUSD
		used := usedUnits / newAPIQuotaPerUSD
		available := total - used
		out = addParsedBalances(out, &parsedUpstreamBalance{Available: &available, Used: &used, Total: &total})
	}
	if out == nil {
		return nil, errors.New("no active subscription balance found")
	}
	return out, nil
}

func addParsedBalances(a, b *parsedUpstreamBalance) *parsedUpstreamBalance {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := &parsedUpstreamBalance{}
	if a.Available != nil || b.Available != nil {
		v := floatPtrValueOrZero(a.Available) + floatPtrValueOrZero(b.Available)
		out.Available = &v
	}
	if a.Used != nil || b.Used != nil {
		v := floatPtrValueOrZero(a.Used) + floatPtrValueOrZero(b.Used)
		out.Used = &v
	}
	if a.Total != nil || b.Total != nil {
		v := floatPtrValueOrZero(a.Total) + floatPtrValueOrZero(b.Total)
		out.Total = &v
	}
	return out
}

func balanceForKeyFromAuthContext(auth *upstreamAuthContext, keyCount int) *parsedUpstreamBalance {
	if auth == nil || auth.userBalance == nil || keyCount != 1 {
		return nil
	}
	return auth.userBalance
}

func manualBalanceForAccount(account *Account) *parsedUpstreamBalance {
	if account == nil {
		return nil
	}
	total := account.GetQuotaLimit()
	if total <= 0 {
		total = account.getExtraFloat64(UpstreamManualBalanceTotalKey)
	}
	if total <= 0 {
		return nil
	}
	used := account.GetQuotaUsed()
	available := total - used
	return &parsedUpstreamBalance{
		Available: &available,
		Used:      &used,
		Total:     &total,
	}
}

func groupsForAccount(auth *upstreamAuthContext, account *Account) []UpstreamBalanceGroupSnapshot {
	if groups := manualRateGroups(account); len(groups) > 0 {
		return groups
	}
	if auth != nil && len(auth.allGroups) > 0 {
		return auth.allGroups
	}
	if groups := fetchedRateGroups(account); len(groups) > 0 {
		return groups
	}
	return nil
}

func groupsForKey(auth *upstreamAuthContext, account *Account, apiKey string) []UpstreamBalanceGroupSnapshot {
	if groups := manualRateGroups(account); len(groups) > 0 {
		return groups
	}
	if auth != nil {
		if groups := auth.groupsByKey[FingerprintAPIKey(apiKey)]; len(groups) > 0 {
			return groups
		}
		if len(auth.allGroups) == 1 && len(allAccountAPIKeys(account)) == 1 {
			return auth.allGroups
		}
	}
	if groups := fetchedRateGroups(account); len(groups) > 0 && len(allAccountAPIKeys(account)) == 1 {
		return groups
	}
	return nil
}

func fetchedRateGroups(account *Account) []UpstreamBalanceGroupSnapshot {
	if account == nil || account.Extra == nil {
		return nil
	}
	raw, ok := account.Extra[UpstreamFetchedGroupsKey].([]any)
	if !ok {
		return nil
	}
	return parseUpstreamBalanceGroups(raw)
}

func manualRateGroups(account *Account) []UpstreamBalanceGroupSnapshot {
	if account == nil {
		return nil
	}
	var ratio float64
	var ok bool
	if account.Extra != nil {
		ratio, ok = parseAnyFloat(account.Extra[UpstreamManualRateMultiplierKey])
	}
	if !ok || ratio <= 0 {
		ratio, ok = parseAnyFloat(account.Credentials[UpstreamManualRateMultiplierKey])
	}
	if !ok || ratio <= 0 {
		// 旧版前端曾把人工倍率写入 credentials 的 common 字段，这里只兼容旧人工配置。
		ratio, ok = parseAnyFloat(account.Credentials[UpstreamCommonRateMultiplierKey])
	}
	if !ok || ratio <= 0 {
		return nil
	}
	name := ""
	if account.Extra != nil {
		name = stringValue(account.Extra[UpstreamManualRateGroupNameKey])
	}
	if name == "" {
		name = stringValue(account.Credentials[UpstreamManualRateGroupNameKey])
	}
	if name == "" {
		name = stringValue(account.Credentials[UpstreamCommonRateGroupNameKey])
	}
	if name == "" {
		name = "manual"
	}
	return []UpstreamBalanceGroupSnapshot{{Name: name, Ratio: ratio, Description: "manual common rate"}}
}

func preserveManualGroups(snapshot *UpstreamBalanceSnapshot, account *Account) {
	if snapshot == nil || len(snapshot.Groups) > 0 {
		return
	}
	groups := groupsForAccount(nil, account)
	if len(groups) == 0 {
		return
	}
	for i := range snapshot.Keys {
		if snapshot.Keys[i].Status != "ok" || len(snapshot.Keys[i].Groups) > 0 {
			continue
		}
		snapshot.Keys[i].Groups = append([]UpstreamBalanceGroupSnapshot(nil), groups...)
		applyConvertedBalancesToGroups(&snapshot.Keys[i])
		mergeKeyGroupsIntoSnapshot(snapshot, snapshot.Keys[i])
	}
	if len(snapshot.Groups) == 0 {
		snapshot.Groups = append([]UpstreamBalanceGroupSnapshot(nil), groups...)
	}
}

func upstreamTokenMatchesKey(token upstreamTokenInfo, apiKey string) bool {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return false
	}
	if token.fullKey != "" && strings.TrimSpace(token.fullKey) == apiKey {
		return true
	}
	if token.masked != "" && maskedTokenMatches(token.masked, apiKey) {
		return true
	}
	if token.fullKey != "" && maskedTokenMatches(token.fullKey, apiKey) {
		return true
	}
	return false
}

func maskedTokenMatches(masked, apiKey string) bool {
	masked = strings.TrimSpace(masked)
	apiKey = strings.TrimSpace(apiKey)
	if masked == "" || apiKey == "" {
		return false
	}
	compact := strings.ReplaceAll(masked, "*", "")
	compact = strings.ReplaceAll(compact, ".", "")
	if compact == "" {
		return false
	}
	parts := strings.FieldsFunc(masked, func(r rune) bool { return r == '*' || r == '.' || r == ' ' })
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 4 && !strings.Contains(apiKey, part) {
			return false
		}
	}
	return true
}

func firstObjectAtPath(root map[string]any, paths ...[]string) map[string]any {
	for _, path := range paths {
		var cur any = root
		for _, key := range path {
			m, ok := cur.(map[string]any)
			if !ok {
				cur = nil
				break
			}
			cur = m[key]
		}
		if m, ok := cur.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func findObjectList(raw any) []map[string]any {
	switch v := raw.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"items", "data", "tokens", "keys", "list"} {
			if items := findObjectList(v[key]); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func findObjectListAtKeys(raw any, keys ...string) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, key := range keys {
				out = append(out, findObjectList(x[key])...)
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(raw)
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		switch m[key].(type) {
		case map[string]any, []any, nil:
			continue
		}
		if s := stringValue(m[key]); s != "" {
			return s
		}
	}
	return ""
}

func stringValue(raw any) string {
	switch raw.(type) {
	case nil, map[string]any, []any:
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(raw))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func findStringByKeys(raw any, keys ...string) string {
	switch v := raw.(type) {
	case map[string]any:
		for _, key := range keys {
			if s := firstString(v, key); s != "" {
				return s
			}
		}
		for _, child := range v {
			if s := findStringByKeys(child, keys...); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range v {
			if s := findStringByKeys(child, keys...); s != "" {
				return s
			}
		}
	}
	return ""
}

func floatPtrValueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func (s *UpstreamBalanceSnapshot) toExtraUpdates(now time.Time) map[string]any {
	updates := map[string]any{
		UpstreamBalanceAvailableKey:   s.Available,
		UpstreamBalanceUsedKey:        s.Used,
		UpstreamBalanceTotalKey:       s.Total,
		UpstreamBalanceKeyCountKey:    s.KeyCount,
		UpstreamBalanceOKCountKey:     s.OKCount,
		UpstreamBalanceFailedCountKey: s.FailedCount,
		UpstreamBalanceUpdatedAtKey:   now.UTC().Format(time.RFC3339),
		UpstreamBalanceErrorKey:       s.Error,
		UpstreamBalanceKeysKey:        s.Keys,
		UpstreamBalanceGroupsKey:      s.Groups,
		UpstreamBalanceConvertedKey:   s.ConvertedAvailableByGroup,
	}
	return updates
}

func setFetchedRateGroupUpdates(updates map[string]any, groups []UpstreamBalanceGroupSnapshot) {
	if updates == nil || len(groups) == 0 {
		return
	}
	updates[UpstreamFetchedGroupsKey] = groups
	if len(groups) == 1 {
		updates[UpstreamCommonRateMultiplierKey] = groups[0].Ratio
		updates[UpstreamCommonRateGroupNameKey] = groups[0].Name
	}
}

func applySnapshotToSummary(summary *UpstreamBalanceSummary, snapshot *UpstreamBalanceSnapshot) {
	if summary == nil || snapshot == nil {
		return
	}
	summary.AccountCount++
	summary.Available += snapshot.Available
	summary.Used += snapshot.Used
	summary.Total += snapshot.Total
	if len(snapshot.ConvertedAvailableByGroup) > 0 {
		if summary.ConvertedAvailableByGroup == nil {
			summary.ConvertedAvailableByGroup = make(map[string]float64, len(snapshot.ConvertedAvailableByGroup))
		}
		for group, value := range snapshot.ConvertedAvailableByGroup {
			summary.ConvertedAvailableByGroup[group] += value
		}
	}
	summary.KeyCount += snapshot.KeyCount
	summary.OKKeyCount += snapshot.OKCount
	summary.FailedKeyCount += snapshot.FailedCount
	if snapshot.OKCount == 0 {
		summary.MissingAccounts++
	}
	if snapshot.UpdatedAt != nil {
		updated := snapshot.UpdatedAt.UTC()
		if summary.LatestUpdatedAt == nil || updated.After(*summary.LatestUpdatedAt) {
			summary.LatestUpdatedAt = &updated
		}
		if summary.OldestUpdatedAt == nil || updated.Before(*summary.OldestUpdatedAt) {
			summary.OldestUpdatedAt = &updated
		}
	}
}

func firstFloatPtr(m map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value, ok := parseAnyFloat(m[key]); ok {
			return &value
		}
	}
	return nil
}

func parseAnyFloat(raw any) (float64, bool) {
	switch v := raw.(type) {
	case nil:
		return 0, false
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(v)
		s = strings.TrimPrefix(s, "$")
		s = strings.ReplaceAll(s, ",", "")
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func parseUpstreamBalanceKeySnapshots(raw []any) []UpstreamBalanceKeySnapshot {
	out := make([]UpstreamBalanceKeySnapshot, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		snap := UpstreamBalanceKeySnapshot{
			Fingerprint: strings.TrimSpace(fmt.Sprint(m["fingerprint"])),
			Masked:      strings.TrimSpace(fmt.Sprint(m["masked"])),
			Status:      strings.TrimSpace(fmt.Sprint(m["status"])),
			Error:       strings.TrimSpace(fmt.Sprint(m["error"])),
			Endpoint:    strings.TrimSpace(fmt.Sprint(m["endpoint"])),
		}
		if v, ok := parseAnyFloat(m["available"]); ok {
			snap.Available = &v
		}
		if v, ok := parseAnyFloat(m["used"]); ok {
			snap.Used = &v
		}
		if v, ok := parseAnyFloat(m["total"]); ok {
			snap.Total = &v
		}
		if ts, err := parseTime(strings.TrimSpace(fmt.Sprint(m["updated_at"]))); err == nil {
			utc := ts.UTC()
			snap.UpdatedAt = &utc
		}
		if rawGroups, ok := m["groups"].([]any); ok {
			snap.Groups = parseUpstreamBalanceGroups(rawGroups)
		}
		out = append(out, snap)
	}
	return out
}

func parseUpstreamBalanceGroups(raw []any) []UpstreamBalanceGroupSnapshot {
	out := make([]UpstreamBalanceGroupSnapshot, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ratio, ok := parseAnyFloat(m["ratio"])
		if !ok || ratio <= 0 {
			continue
		}
		group := UpstreamBalanceGroupSnapshot{
			Name:        strings.TrimSpace(fmt.Sprint(m["name"])),
			Ratio:       ratio,
			Description: strings.TrimSpace(fmt.Sprint(m["description"])),
		}
		if v, ok := parseAnyFloat(m["converted_available"]); ok {
			group.ConvertedAvailable = &v
		}
		if v, ok := parseAnyFloat(m["converted_total"]); ok {
			group.ConvertedTotal = &v
		}
		if v, ok := parseAnyFloat(m["converted_used"]); ok {
			group.ConvertedUsed = &v
		}
		if group.Name != "" {
			out = append(out, group)
		}
	}
	return out
}

func parseStringFloatMap(raw map[string]any) map[string]float64 {
	out := make(map[string]float64, len(raw))
	for key, value := range raw {
		if f, ok := parseAnyFloat(value); ok {
			out[key] = f
		}
	}
	return out
}

func applyConvertedBalancesToGroups(item *UpstreamBalanceKeySnapshot) {
	if item == nil || len(item.Groups) == 0 {
		return
	}
	for i := range item.Groups {
		ratio := item.Groups[i].Ratio
		if ratio <= 0 {
			continue
		}
		if item.Available != nil {
			v := *item.Available / ratio
			item.Groups[i].ConvertedAvailable = &v
		}
		if item.Total != nil {
			v := *item.Total / ratio
			item.Groups[i].ConvertedTotal = &v
		}
		if item.Used != nil {
			v := *item.Used / ratio
			item.Groups[i].ConvertedUsed = &v
		}
	}
}

func mergeKeyGroupsIntoSnapshot(snapshot *UpstreamBalanceSnapshot, item UpstreamBalanceKeySnapshot) {
	if snapshot == nil || len(item.Groups) == 0 {
		return
	}
	if snapshot.ConvertedAvailableByGroup == nil {
		snapshot.ConvertedAvailableByGroup = make(map[string]float64)
	}
	seen := make(map[string]int, len(snapshot.Groups))
	for i, group := range snapshot.Groups {
		seen[group.Name] = i
	}
	for _, group := range item.Groups {
		if group.Name == "" || group.Ratio <= 0 {
			continue
		}
		if group.ConvertedAvailable != nil {
			snapshot.ConvertedAvailableByGroup[group.Name] += *group.ConvertedAvailable
		}
		if idx, ok := seen[group.Name]; ok {
			if snapshot.Groups[idx].Description == "" {
				snapshot.Groups[idx].Description = group.Description
			}
			continue
		}
		snapshot.Groups = append(snapshot.Groups, UpstreamBalanceGroupSnapshot{Name: group.Name, Ratio: group.Ratio, Description: group.Description})
		seen[group.Name] = len(snapshot.Groups) - 1
	}
}

func maskUpstreamAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return "****"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func trimErrorBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, err := url.ParseRequestURI(value); err != nil {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
