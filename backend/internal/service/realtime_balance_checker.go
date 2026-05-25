package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	defaultRealtimeBalanceTimeout = 1200 * time.Millisecond

	RealtimeBalanceStateHealthy   = "healthy"
	RealtimeBalanceStateDraining  = "draining"
	RealtimeBalanceStateExhausted = "exhausted"
	RealtimeBalanceStateUnknown   = "balance_unknown"

	RealtimeBalanceSourceRemote = "remote"
	RealtimeBalanceSourceManual = "manual"
	RealtimeBalanceSourceError  = "error"

	realtimeBalanceDrainThreshold  = "drain_threshold_usd"
	realtimeBalanceStickyReserve   = "sticky_reserve_usd"
	realtimeBalanceThresholdRemote = "remote_unverified"
)

var ErrNoVerifiedRealtimeBalanceCandidate = errors.New("no verified realtime balance candidate")

type RealtimeBalanceDecision struct {
	AccountID int64
	State     string
	Available float64
	Threshold string
	Source    string
	CheckedAt time.Time
	Error     string
}

type RealtimeBalanceRefresher interface {
	RefreshAccount(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error)
}

type RealtimeBalanceCheckerOptions struct {
	Enabled           bool
	EnabledSet        bool
	DrainThresholdUSD float64
	StickyReserveUSD  float64
	Timeout           time.Duration
	CandidateTopN     int
}

type RealtimeBalanceChecker struct {
	refresher         RealtimeBalanceRefresher
	enabled           bool
	enabledSet        bool
	drainThresholdUSD float64
	stickyReserveUSD  float64
	timeout           time.Duration
	candidateTopN     int
	group             singleflight.Group
}

type realtimeBalanceRefresherFunc func(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error)

func (f realtimeBalanceRefresherFunc) RefreshAccount(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
	return f(ctx, account)
}

func NewRealtimeBalanceChecker(refresher RealtimeBalanceRefresher, options RealtimeBalanceCheckerOptions) *RealtimeBalanceChecker {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultRealtimeBalanceTimeout
	}
	return &RealtimeBalanceChecker{
		refresher:         refresher,
		enabled:           options.Enabled,
		enabledSet:        options.EnabledSet,
		drainThresholdUSD: options.DrainThresholdUSD,
		stickyReserveUSD:  options.StickyReserveUSD,
		timeout:           timeout,
		candidateTopN:     options.CandidateTopN,
	}
}

func (c *RealtimeBalanceChecker) CheckAccount(ctx context.Context, account *Account) (*RealtimeBalanceDecision, error) {
	if c == nil || c.refresher == nil {
		return nil, errors.New("realtime balance checker is not configured")
	}
	if account == nil || account.ID <= 0 {
		return nil, errors.New("account is required")
	}
	if c.enabledSet && !c.enabled {
		return c.unknownDecision(account.ID, "realtime balance checker disabled"), nil
	}
	key := fmt.Sprintf("%d", account.ID)
	value, _, _ := c.group.Do(key, func() (any, error) {
		checkCtx := ctx
		cancel := func() {}
		if c.timeout > 0 {
			checkCtx, cancel = context.WithTimeout(ctx, c.timeout)
		}
		defer cancel()

		snapshot, err := c.refresher.RefreshAccount(checkCtx, account)
		return c.decisionFromSnapshot(account.ID, snapshot, err), nil
	})
	decision, _ := value.(*RealtimeBalanceDecision)
	return decision, nil
}

func (c *RealtimeBalanceChecker) SelectFirstVerifiedCandidate(ctx context.Context, accounts []*Account, minAvailable float64) (*Account, *UpstreamBalanceSnapshot, error) {
	if c == nil {
		return nil, nil, errors.New("realtime balance checker is not configured")
	}
	topN := c.candidateTopN
	if topN <= 0 || topN > len(accounts) {
		topN = len(accounts)
	}
	type candidateDecision struct {
		index    int
		account  *Account
		decision *RealtimeBalanceDecision
	}
	results := make([]candidateDecision, 0, topN)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, account := range accounts {
		if i >= topN {
			break
		}
		if account == nil {
			continue
		}
		wg.Add(1)
		go func(index int, candidate *Account) {
			defer wg.Done()
			decision, err := c.CheckAccount(ctx, candidate)
			if err != nil || decision == nil {
				return
			}
			mu.Lock()
			results = append(results, candidateDecision{index: index, account: candidate, decision: decision})
			mu.Unlock()
		}(i, account)
	}
	wg.Wait()
	if len(results) == 0 {
		return nil, nil, ErrNoVerifiedRealtimeBalanceCandidate
	}
	for i := 0; i < topN; i++ {
		for _, result := range results {
			if result.index != i || result.decision == nil {
				continue
			}
			if result.decision.State == RealtimeBalanceStateUnknown || result.decision.Available < minAvailable {
				continue
			}
			return result.account, &UpstreamBalanceSnapshot{Available: result.decision.Available, OKCount: 1}, nil
		}
	}
	return nil, nil, ErrNoVerifiedRealtimeBalanceCandidate
}

func (c *RealtimeBalanceChecker) decisionFromSnapshot(accountID int64, snapshot *UpstreamBalanceSnapshot, err error) *RealtimeBalanceDecision {
	if err != nil {
		return c.unknownDecision(accountID, err.Error())
	}
	if !IsVerifiedRealtimeBalanceSnapshot(snapshot) {
		if snapshot != nil && snapshot.Error != "" {
			return c.unknownDecision(accountID, snapshot.Error)
		}
		return c.unknownDecision(accountID, "realtime balance not verified")
	}

	decision := &RealtimeBalanceDecision{
		AccountID: accountID,
		Available: snapshot.Available,
		Source:    realtimeBalanceSourceFromSnapshot(snapshot),
		CheckedAt: time.Now().UTC(),
	}
	if snapshot.Available >= c.drainThresholdUSD {
		decision.State = RealtimeBalanceStateHealthy
		decision.Threshold = realtimeBalanceDrainThreshold
		return decision
	}
	if snapshot.Available >= c.stickyReserveUSD {
		decision.State = RealtimeBalanceStateDraining
		decision.Threshold = realtimeBalanceStickyReserve
		return decision
	}
	decision.State = RealtimeBalanceStateExhausted
	decision.Threshold = realtimeBalanceStickyReserve
	return decision
}

func (c *RealtimeBalanceChecker) unknownDecision(accountID int64, message string) *RealtimeBalanceDecision {
	return &RealtimeBalanceDecision{
		AccountID: accountID,
		State:     RealtimeBalanceStateUnknown,
		Threshold: realtimeBalanceThresholdRemote,
		Source:    RealtimeBalanceSourceError,
		CheckedAt: time.Now().UTC(),
		Error:     message,
	}
}

func realtimeBalanceSourceFromSnapshot(snapshot *UpstreamBalanceSnapshot) string {
	if snapshot == nil {
		return RealtimeBalanceSourceError
	}
	for _, key := range snapshot.Keys {
		if strings.EqualFold(key.Endpoint, "manual") || strings.HasPrefix(key.Fingerprint, "manual:") {
			return RealtimeBalanceSourceManual
		}
	}
	return RealtimeBalanceSourceRemote
}

func IsVerifiedRealtimeBalanceSnapshot(snapshot *UpstreamBalanceSnapshot) bool {
	return snapshot != nil && snapshot.OKCount > 0
}
