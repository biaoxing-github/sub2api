package service

import (
	"context"
	"errors"
	"strings"
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

func TestRealtimeBalanceCheckerDecidesHealthyDrainingExhausted(t *testing.T) {
	tests := []struct {
		name      string
		available float64
		wantState string
		wantLimit string
	}{
		{name: "healthy", available: 3, wantState: RealtimeBalanceStateHealthy, wantLimit: "drain_threshold_usd"},
		{name: "draining", available: 1.2, wantState: RealtimeBalanceStateDraining, wantLimit: "sticky_reserve_usd"},
		{name: "exhausted", available: 0.2, wantState: RealtimeBalanceStateExhausted, wantLimit: "sticky_reserve_usd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refresher := &fakeRealtimeBalanceRefresher{
				snapshot: &UpstreamBalanceSnapshot{Available: tt.available, OKCount: 1},
			}
			checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
				Enabled:           true,
				DrainThresholdUSD: 2,
				StickyReserveUSD:  0.5,
				Timeout:           time.Second,
			})

			decision, err := checker.CheckAccount(context.Background(), &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey})
			if err != nil {
				t.Fatalf("CheckAccount() error = %v", err)
			}
			if decision.State != tt.wantState {
				t.Fatalf("State = %q, want %q", decision.State, tt.wantState)
			}
			if decision.AccountID != 42 {
				t.Fatalf("AccountID = %d, want 42", decision.AccountID)
			}
			if decision.Available != tt.available {
				t.Fatalf("Available = %v, want %v", decision.Available, tt.available)
			}
			if decision.Threshold != tt.wantLimit {
				t.Fatalf("Threshold = %q, want %q", decision.Threshold, tt.wantLimit)
			}
			if decision.Source != RealtimeBalanceSourceRemote {
				t.Fatalf("Source = %q, want remote", decision.Source)
			}
			if decision.CheckedAt.IsZero() {
				t.Fatal("CheckedAt is zero")
			}
		})
	}
}

func TestRealtimeBalanceCheckerReturnsUnknownForRefreshErrorAndUnverifiedSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		refresher *fakeRealtimeBalanceRefresher
		wantError string
	}{
		{
			name:      "refresh error",
			refresher: &fakeRealtimeBalanceRefresher{err: errors.New("upstream unavailable")},
			wantError: "upstream unavailable",
		},
		{
			name:      "no ok snapshot",
			refresher: &fakeRealtimeBalanceRefresher{snapshot: &UpstreamBalanceSnapshot{Available: 10, OKCount: 0, Error: "parse failed"}},
			wantError: "parse failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewRealtimeBalanceChecker(tt.refresher, RealtimeBalanceCheckerOptions{
				Enabled:           true,
				DrainThresholdUSD: 2,
				StickyReserveUSD:  0.5,
				Timeout:           time.Second,
			})

			decision, err := checker.CheckAccount(context.Background(), &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey})
			if err != nil {
				t.Fatalf("CheckAccount() error = %v", err)
			}
			if decision.State != RealtimeBalanceStateUnknown {
				t.Fatalf("State = %q, want %q", decision.State, RealtimeBalanceStateUnknown)
			}
			if decision.Source != RealtimeBalanceSourceError {
				t.Fatalf("Source = %q, want error", decision.Source)
			}
			if decision.Error != tt.wantError {
				t.Fatalf("Error = %q, want %q", decision.Error, tt.wantError)
			}
		})
	}
}

func TestRealtimeBalanceCheckerMarksManualSnapshotSource(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{
		snapshot: &UpstreamBalanceSnapshot{
			Available: 4,
			OKCount:   1,
			Keys: []UpstreamBalanceKeySnapshot{{
				Fingerprint: "manual:9",
				Status:      "ok",
				Endpoint:    "manual",
			}},
		},
	}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})

	decision, err := checker.CheckAccount(context.Background(), &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey})
	if err != nil {
		t.Fatalf("CheckAccount() error = %v", err)
	}
	if decision.Source != RealtimeBalanceSourceManual {
		t.Fatalf("Source = %q, want manual", decision.Source)
	}
	if decision.State != RealtimeBalanceStateHealthy {
		t.Fatalf("State = %q, want healthy", decision.State)
	}
}

func TestRealtimeBalanceCheckerDisabledDoesNotRefresh(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{
		snapshot: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1},
	}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           false,
		EnabledSet:        true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})

	decision, err := checker.CheckAccount(context.Background(), &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeAPIKey})
	if err != nil {
		t.Fatalf("CheckAccount() error = %v", err)
	}
	if decision.State != RealtimeBalanceStateUnknown {
		t.Fatalf("State = %q, want %q", decision.State, RealtimeBalanceStateUnknown)
	}
	if refresher.callCount() != 0 {
		t.Fatalf("calls = %d, want 0", refresher.callCount())
	}
}

func TestRealtimeBalanceCheckerCollapsesConcurrentAccountChecks(t *testing.T) {
	wait := make(chan struct{})
	refresher := &fakeRealtimeBalanceRefresher{
		wait:     wait,
		snapshot: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1},
	}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	const workers = 8
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		decision, err := checker.CheckAccount(context.Background(), account)
		if err != nil {
			errs <- err
			return
		}
		if decision.State != RealtimeBalanceStateHealthy || decision.Available != 5 {
			errs <- errors.New("unexpected decision")
		}
	}()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if refresher.callCount() == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if refresher.callCount() != 1 {
		t.Fatalf("first request did not start")
	}
	for i := 1; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := checker.CheckAccount(context.Background(), account)
			if err != nil {
				errs <- err
				return
			}
			if decision.State != RealtimeBalanceStateHealthy || decision.Available != 5 {
				errs <- errors.New("unexpected decision")
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
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

func TestRealtimeBalanceCheckerTimeoutReturnsUnknownDecision(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{wait: make(chan struct{})}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           10 * time.Millisecond,
	})

	decision, err := checker.CheckAccount(context.Background(), &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey})
	if err != nil {
		t.Fatalf("CheckAccount() error = %v", err)
	}
	if decision.State != RealtimeBalanceStateUnknown {
		t.Fatalf("State = %q, want %q", decision.State, RealtimeBalanceStateUnknown)
	}
	if decision.Source != RealtimeBalanceSourceError {
		t.Fatalf("Source = %q, want error", decision.Source)
	}
	if !errors.Is(context.DeadlineExceeded, context.DeadlineExceeded) || decision.Error == "" {
		t.Fatalf("Error = %q, want timeout text", decision.Error)
	}
}

func TestRealtimeBalanceCheckerSnapshotFirstUsesFreshSnapshotWithoutBlocking(t *testing.T) {
	wait := make(chan struct{})
	refresher := &fakeRealtimeBalanceRefresher{wait: wait, snapshot: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1}}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})
	now := time.Now().UTC()
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			UpstreamBalanceAvailableKey:   4.0,
			UpstreamBalanceOKCountKey:     1,
			UpstreamBalanceUpdatedAtKey:   now.Format(time.RFC3339),
			UpstreamBalanceKeyCountKey:    1,
			UpstreamBalanceFailedCountKey: 0,
		},
	}

	start := time.Now()
	decision, err := checker.CheckAccountSnapshotFirst(context.Background(), account, RealtimeBalanceCheckOptions{
		Now: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CheckAccountSnapshotFirst() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("CheckAccountSnapshotFirst() blocked for %v", elapsed)
	}
	if decision.State != RealtimeBalanceStateHealthy || decision.Available != 4 {
		t.Fatalf("decision = %+v", decision)
	}
	if refresher.callCount() != 0 {
		t.Fatalf("refresh calls = %d, want 0", refresher.callCount())
	}
	close(wait)
}

func TestRealtimeBalanceCheckerSnapshotFirstSchedulesAsyncRefreshForAgingSnapshot(t *testing.T) {
	wait := make(chan struct{})
	refresher := &fakeRealtimeBalanceRefresher{wait: wait, snapshot: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1}}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})
	now := time.Now().UTC()
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			UpstreamBalanceAvailableKey: 5.0,
			UpstreamBalanceOKCountKey:   1,
			UpstreamBalanceUpdatedAtKey: now.Add(-10 * time.Minute).Format(time.RFC3339),
		},
	}

	start := time.Now()
	decision, err := checker.CheckAccountSnapshotFirst(context.Background(), account, RealtimeBalanceCheckOptions{
		Now:               now,
		AllowAsyncRefresh: true,
	})
	if err != nil {
		t.Fatalf("CheckAccountSnapshotFirst() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("CheckAccountSnapshotFirst() blocked for %v", elapsed)
	}
	if decision.State != RealtimeBalanceStateHealthy || decision.Available != 5 {
		t.Fatalf("decision = %+v", decision)
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if refresher.callCount() == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if refresher.callCount() != 1 {
		t.Fatalf("async refresh calls = %d, want 1", refresher.callCount())
	}
	close(wait)
}

func TestRealtimeBalanceCheckerSnapshotFirstPreflightsHighRisk(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{snapshot: &UpstreamBalanceSnapshot{Available: 3, OKCount: 1}}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})
	now := time.Now().UTC()
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			UpstreamBalanceAvailableKey: 0.25,
			UpstreamBalanceOKCountKey:   1,
			UpstreamBalanceUpdatedAtKey: now.Format(time.RFC3339),
		},
	}

	decision, err := checker.CheckAccountSnapshotFirst(context.Background(), account, RealtimeBalanceCheckOptions{
		Now: now,
	})
	if err != nil {
		t.Fatalf("CheckAccountSnapshotFirst() error = %v", err)
	}
	if refresher.callCount() != 1 {
		t.Fatalf("refresh calls = %d, want 1", refresher.callCount())
	}
	if decision.State != RealtimeBalanceStateHealthy || decision.Available != 3 {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestRealtimeBalanceCheckerSnapshotOnlyDoesNotPreflightHighRisk(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{snapshot: &UpstreamBalanceSnapshot{Available: 3, OKCount: 1}}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})
	now := time.Now().UTC()
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			UpstreamBalanceAvailableKey: 0.25,
			UpstreamBalanceOKCountKey:   1,
			UpstreamBalanceUpdatedAtKey: now.Format(time.RFC3339),
		},
	}

	decision, err := checker.CheckAccountSnapshotFirst(context.Background(), account, RealtimeBalanceCheckOptions{
		Now:          now,
		SnapshotOnly: true,
	})
	if err != nil {
		t.Fatalf("CheckAccountSnapshotFirst() error = %v", err)
	}
	if refresher.callCount() != 0 {
		t.Fatalf("refresh calls = %d, want 0", refresher.callCount())
	}
	if decision.State != RealtimeBalanceStateExhausted || decision.Available != 0.25 {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestRealtimeBalanceCheckerSnapshotFirstReturnsRiskOnPreflightFailure(t *testing.T) {
	refresher := &fakeRealtimeBalanceRefresher{err: errors.New("upstream balance unreachable")}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
	})
	now := time.Now().UTC()
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			UpstreamBalanceAvailableKey: 0.25,
			UpstreamBalanceOKCountKey:   1,
			UpstreamBalanceUpdatedAtKey: now.Format(time.RFC3339),
		},
	}

	decision, err := checker.CheckAccountSnapshotFirst(context.Background(), account, RealtimeBalanceCheckOptions{
		Now: now,
	})
	if err != nil {
		t.Fatalf("CheckAccountSnapshotFirst() error = %v", err)
	}
	if decision.State != RealtimeBalanceStateUnknown {
		t.Fatalf("State = %q, want %q", decision.State, RealtimeBalanceStateUnknown)
	}
	if !strings.Contains(decision.Error, "preflight") || !strings.Contains(decision.Error, "upstream balance unreachable") {
		t.Fatalf("Error = %q, want explicit preflight risk", decision.Error)
	}
}

func TestRealtimeBalanceCheckerSelectFirstVerifiedCandidateChecksTopNConcurrently(t *testing.T) {
	wait := make(chan struct{})
	refresher := &fakeRealtimeBalanceRefresher{
		wait:     wait,
		snapshot: &UpstreamBalanceSnapshot{Available: 5, OKCount: 1},
	}
	checker := NewRealtimeBalanceChecker(refresher, RealtimeBalanceCheckerOptions{
		Enabled:           true,
		DrainThresholdUSD: 2,
		StickyReserveUSD:  0.5,
		Timeout:           time.Second,
		CandidateTopN:     3,
	})
	accounts := []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	}

	done := make(chan struct{})
	go func() {
		_, _, _ = checker.SelectFirstVerifiedCandidate(context.Background(), accounts, 1)
		close(done)
	}()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if refresher.callCount() == 3 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if refresher.callCount() != 3 {
		t.Fatalf("calls before release = %d, want 3", refresher.callCount())
	}
	close(wait)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SelectFirstVerifiedCandidate did not return")
	}
}

func TestRealtimeBalanceCheckerSelectFirstVerifiedCandidateWithDiagnosticsRecordsTopNOutcome(t *testing.T) {
	checker := NewRealtimeBalanceChecker(realtimeBalanceRefresherFunc(func(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
		switch account.ID {
		case 81:
			return &UpstreamBalanceSnapshot{Available: 0, OKCount: 1}, nil
		case 82:
			return &UpstreamBalanceSnapshot{Available: 10, OKCount: 0, Error: "parse failed"}, nil
		default:
			return nil, errors.New("unexpected account")
		}
	}), RealtimeBalanceCheckerOptions{Timeout: time.Second, CandidateTopN: 2, StickyReserveUSD: 0.5})

	account, snapshot, diagnostic, err := checker.SelectFirstVerifiedCandidateWithDiagnostics(
		context.Background(),
		[]*Account{
			{ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			{ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			{ID: 83, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		},
		1,
		RealtimeBalanceCheckOptions{},
	)
	if !errors.Is(err, ErrNoVerifiedRealtimeBalanceCandidate) {
		t.Fatalf("err = %v, want ErrNoVerifiedRealtimeBalanceCandidate", err)
	}
	if account != nil || snapshot != nil {
		t.Fatalf("account=%v snapshot=%v, want nil", account, snapshot)
	}
	if diagnostic == nil {
		t.Fatal("diagnostic is nil")
	}
	if diagnostic.Source != "top_n" || diagnostic.TopN != 2 || diagnostic.Checked != 2 || diagnostic.SelectedAccountID != 0 {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
	if diagnostic.LatencyMs < 0 {
		t.Fatalf("LatencyMs = %d, want non-negative", diagnostic.LatencyMs)
	}
	if !strings.Contains(diagnostic.Reason, "no verified realtime balance candidate") {
		t.Fatalf("Reason = %q, want no verified candidate", diagnostic.Reason)
	}
	if len(diagnostic.Candidates) != 2 {
		t.Fatalf("candidates = %+v, want 2", diagnostic.Candidates)
	}
	if diagnostic.Candidates[0].AccountID != 81 || diagnostic.Candidates[0].State != RealtimeBalanceStateExhausted || diagnostic.Candidates[0].Reason != "available_below_min" {
		t.Fatalf("candidate[0] = %+v", diagnostic.Candidates[0])
	}
	if diagnostic.Candidates[1].AccountID != 82 || diagnostic.Candidates[1].Source != RealtimeBalanceSourceError || !strings.Contains(diagnostic.Candidates[1].Reason, "parse failed") {
		t.Fatalf("candidate[1] = %+v", diagnostic.Candidates[1])
	}
}
