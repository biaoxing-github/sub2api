package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

const defaultRealtimeBalanceTimeout = 1200 * time.Millisecond

var ErrNoVerifiedRealtimeBalanceCandidate = errors.New("no verified realtime balance candidate")

type RealtimeBalanceRefresher interface {
	RefreshAccount(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error)
}

type RealtimeBalanceCheckerOptions struct {
	Timeout time.Duration
}

type RealtimeBalanceChecker struct {
	refresher RealtimeBalanceRefresher
	timeout   time.Duration
	group     singleflight.Group
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
		refresher: refresher,
		timeout:   timeout,
	}
}

func (c *RealtimeBalanceChecker) CheckAccount(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
	if c == nil || c.refresher == nil {
		return nil, errors.New("realtime balance checker is not configured")
	}
	if account == nil || account.ID <= 0 {
		return nil, errors.New("account is required")
	}
	key := fmt.Sprintf("%d", account.ID)
	value, err, _ := c.group.Do(key, func() (any, error) {
		checkCtx := ctx
		cancel := func() {}
		if c.timeout > 0 {
			checkCtx, cancel = context.WithTimeout(ctx, c.timeout)
		}
		defer cancel()

		snapshot, err := c.refresher.RefreshAccount(checkCtx, account)
		if err != nil {
			return nil, err
		}
		if !IsVerifiedRealtimeBalanceSnapshot(snapshot) {
			if snapshot != nil && snapshot.Error != "" {
				return nil, fmt.Errorf("realtime balance not verified: %s", snapshot.Error)
			}
			return nil, errors.New("realtime balance not verified")
		}
		return snapshot, nil
	})
	if err != nil {
		return nil, err
	}
	snapshot, _ := value.(*UpstreamBalanceSnapshot)
	return snapshot, nil
}

func (c *RealtimeBalanceChecker) SelectFirstVerifiedCandidate(ctx context.Context, accounts []*Account, minAvailable float64) (*Account, *UpstreamBalanceSnapshot, error) {
	if c == nil {
		return nil, nil, errors.New("realtime balance checker is not configured")
	}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		snapshot, err := c.CheckAccount(ctx, account)
		if err != nil {
			continue
		}
		if snapshot == nil || snapshot.Available < minAvailable {
			continue
		}
		return account, snapshot, nil
	}
	return nil, nil, ErrNoVerifiedRealtimeBalanceCandidate
}

func IsVerifiedRealtimeBalanceSnapshot(snapshot *UpstreamBalanceSnapshot) bool {
	return snapshot != nil && snapshot.OKCount > 0
}
