package admin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// accountBatchTestLimiter 只限制后台批量体检流量，避免体检任务和真实请求抢同一批上游承载。
type accountBatchTestLimiter struct {
	global            chan struct{}
	groupLimit        int
	degradedLimit     int
	window            time.Duration
	pauseDuration     time.Duration
	now               func() time.Time
	mu                sync.Mutex
	groups            map[string]*accountBatchTestLimiterGroup
	groupSlotsChanged chan struct{}
}

type accountBatchTestLimiterGroup struct {
	active        int
	windowStart   time.Time
	riskyFailures int
	pauseUntil    time.Time
}

func newAccountBatchTestLimiter(globalLimit, groupLimit int, window, pauseDuration time.Duration) *accountBatchTestLimiter {
	if globalLimit <= 0 {
		globalLimit = 1
	}
	if groupLimit <= 0 {
		groupLimit = 1
	}
	if window <= 0 {
		window = 10 * time.Second
	}
	if pauseDuration <= 0 {
		pauseDuration = 30 * time.Second
	}
	return &accountBatchTestLimiter{
		global:            make(chan struct{}, globalLimit),
		groupLimit:        groupLimit,
		degradedLimit:     1,
		window:            window,
		pauseDuration:     pauseDuration,
		now:               time.Now,
		groups:            make(map[string]*accountBatchTestLimiterGroup),
		groupSlotsChanged: make(chan struct{}),
	}
}

func (l *accountBatchTestLimiter) Acquire(ctx context.Context, groupKey string) (func(), error) {
	if l == nil {
		return func() {}, nil
	}
	groupKey = normalizeAccountBatchTestLimiterKey(groupKey)
	if err := l.waitForGroupPause(ctx, groupKey); err != nil {
		return nil, err
	}

	select {
	case l.global <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	releaseGlobal := true
	defer func() {
		if releaseGlobal {
			<-l.global
		}
	}()

	if err := l.acquireGroup(ctx, groupKey); err != nil {
		return nil, err
	}
	released := false
	releaseGlobal = false
	return func() {
		if released {
			return
		}
		released = true
		l.releaseGroup(groupKey)
		<-l.global
	}, nil
}

func (l *accountBatchTestLimiter) waitForGroupPause(ctx context.Context, groupKey string) error {
	for {
		now := l.now()
		pausedUntil, paused := l.groupPausedUntil(groupKey, now)
		if !paused {
			return nil
		}
		wait := pausedUntil.Sub(now)
		if wait <= 0 {
			return nil
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("batch test group %s paused until %s after upstream error burst: %w", groupKey, pausedUntil.Format(time.RFC3339), ctx.Err())
		case <-timer.C:
		}
	}
}

func (l *accountBatchTestLimiter) RecordResult(groupKey, category string) {
	if l == nil || !accountBatchTestLimiterRiskyCategory(category) {
		return
	}
	groupKey = normalizeAccountBatchTestLimiterKey(groupKey)
	now := l.now()

	l.mu.Lock()
	group := l.groupLocked(groupKey)
	if group.windowStart.IsZero() || now.Sub(group.windowStart) > l.window {
		group.windowStart = now
		group.riskyFailures = 0
	}
	group.riskyFailures++
	if group.riskyFailures >= 3 {
		group.pauseUntil = now.Add(l.pauseDuration)
	}
	l.mu.Unlock()
	l.notifyGroupSlotsChanged()
}

func (l *accountBatchTestLimiter) acquireGroup(ctx context.Context, groupKey string) error {
	for {
		now := l.now()
		l.mu.Lock()
		group := l.groupLocked(groupKey)
		if group.pauseUntil.After(now) {
			l.mu.Unlock()
			if err := l.waitForGroupPause(ctx, groupKey); err != nil {
				return err
			}
			continue
		}
		if !group.windowStart.IsZero() && now.Sub(group.windowStart) > l.window {
			group.windowStart = time.Time{}
			group.riskyFailures = 0
		}
		limit := l.groupLimit
		if group.riskyFailures > 0 {
			limit = l.degradedLimit
		}
		if limit <= 0 {
			limit = 1
		}
		if group.active < limit {
			group.active++
			l.mu.Unlock()
			return nil
		}
		changed := l.groupSlotsChanged
		l.mu.Unlock()

		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (l *accountBatchTestLimiter) releaseGroup(groupKey string) {
	groupKey = normalizeAccountBatchTestLimiterKey(groupKey)
	l.mu.Lock()
	if group := l.groups[groupKey]; group != nil && group.active > 0 {
		group.active--
	}
	l.mu.Unlock()
	l.notifyGroupSlotsChanged()
}

func (l *accountBatchTestLimiter) groupPausedUntil(groupKey string, now time.Time) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	group := l.groupLocked(groupKey)
	if group.pauseUntil.After(now) {
		return group.pauseUntil, true
	}
	return time.Time{}, false
}

func (l *accountBatchTestLimiter) groupLocked(groupKey string) *accountBatchTestLimiterGroup {
	group := l.groups[groupKey]
	if group == nil {
		group = &accountBatchTestLimiterGroup{}
		l.groups[groupKey] = group
	}
	return group
}

func (l *accountBatchTestLimiter) notifyGroupSlotsChanged() {
	l.mu.Lock()
	old := l.groupSlotsChanged
	l.groupSlotsChanged = make(chan struct{})
	close(old)
	l.mu.Unlock()
}

func normalizeAccountBatchTestLimiterKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func accountBatchTestLimiterRiskyCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "rate_limited", "client_ip_circuit_open", "unexpected_eof", "header_timeout", "timeout", "cloudflare_waf", "upstream_5xx":
		return true
	default:
		return false
	}
}
