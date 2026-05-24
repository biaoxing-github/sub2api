//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeRealtimeBalanceRefresher struct {
	mu       sync.Mutex
	calls    int
	wait     chan struct{}
	snapshot *UpstreamBalanceSnapshot
	err      error
}

func (f *fakeRealtimeBalanceRefresher) RefreshAccount(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if f.wait != nil {
		select {
		case <-f.wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

func (f *fakeRealtimeBalanceRefresher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestRealtimeBalanceCheckerCallsRefresherWithRealAccount(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{
		snapshot: &UpstreamBalanceSnapshot{Available: 3, OKCount: 1},
	}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{Timeout: time.Second})
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	got, err := checker.CheckAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("CheckAccount() error = %v", err)
	}
	if got == nil || got.Available != 3 || got.OKCount != 1 {
		t.Fatalf("snapshot = %+v", got)
	}
	if refresher.callCount() != 1 {
		t.Fatalf("calls = %d, want 1", refresher.callCount())
	}
}

func TestRealtimeBalanceCheckerCollapsesConcurrentAccountChecks(t *testing.T) {
	wait := make(chan struct{})
	refresher := &fakeRealtimeBalanceRefresher{
		wait:     wait,
		snapshot: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1},
	}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{Timeout: time.Second})
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	const workers = 8
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot, err := checker.CheckAccount(context.Background(), account)
			if err != nil {
				errs <- err
				return
			}
			if snapshot == nil || snapshot.Available != 5 {
				errs <- errors.New("unexpected snapshot")
			}
		}()
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if refresher.callCount() == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(wait)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if refresher.callCount() != 1 {
		t.Fatalf("calls = %d, want 1", refresher.callCount())
	}
}

func TestRealtimeBalanceCheckerTimeoutDoesNotReturnVerifiedSnapshot(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{
		wait: make(chan struct{}),
	}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{Timeout: 10 * time.Millisecond})
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	snapshot, err := checker.CheckAccount(context.Background(), account)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CheckAccount() error = %v, want deadline exceeded", err)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want nil", snapshot)
	}
}

func TestSelectFirstVerifiedRealtimeBalanceCandidateSkipsFailuresAndZeroOK(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{Timeout: time.Second})
	accounts := []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	}

	checker.refresher = realtimeBalanceRefresherFunc(func(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
		switch account.ID {
		case 1:
			return &UpstreamBalanceSnapshot{Available: 10, OKCount: 0, Error: "parse failed"}, nil
		case 2:
			return &UpstreamBalanceSnapshot{Available: 2.5, OKCount: 1}, nil
		default:
			return nil, errors.New("unexpected account")
		}
	})

	selected, snapshot, err := checker.SelectFirstVerifiedCandidate(context.Background(), accounts, 1.0)
	if err != nil {
		t.Fatalf("SelectFirstVerifiedCandidate() error = %v", err)
	}
	if selected == nil || selected.ID != 2 {
		t.Fatalf("selected = %+v, want account 2", selected)
	}
	if snapshot == nil || snapshot.Available != 2.5 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
