//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testProxyFallbackProxy(id int64, mode string, backup *int64, expiresInDays *int, now time.Time) Proxy {
	p := Proxy{ID: id, FallbackMode: mode, BackupProxyID: backup, Status: StatusActive}
	if expiresInDays != nil {
		t := now.AddDate(0, 0, *expiresInDays)
		p.ExpiresAt = &t
	}
	return p
}

func testProxyFallbackInt64(v int64) *int64 { return &v }
func testProxyFallbackDays(v int) *int      { return &v }

func TestResolveProxyFallbackTarget(t *testing.T) {
	now := time.Now()

	t.Run("none mode keeps original binding", func(t *testing.T) {
		start := testProxyFallbackProxy(1, FallbackModeNone, nil, testProxyFallbackDays(-1), now)
		target, change := ResolveProxyFallbackTarget(start, map[int64]Proxy{1: start}, now)

		require.False(t, change)
		require.Nil(t, target)
	})

	t.Run("direct mode moves accounts to direct connection", func(t *testing.T) {
		start := testProxyFallbackProxy(1, FallbackModeDirect, nil, testProxyFallbackDays(-1), now)
		target, change := ResolveProxyFallbackTarget(start, map[int64]Proxy{1: start}, now)

		require.True(t, change)
		require.Nil(t, target)
	})

	t.Run("proxy mode moves accounts to healthy backup", func(t *testing.T) {
		backup := testProxyFallbackProxy(2, FallbackModeNone, nil, testProxyFallbackDays(30), now)
		start := testProxyFallbackProxy(1, FallbackModeProxy, testProxyFallbackInt64(2), testProxyFallbackDays(-1), now)
		target, change := ResolveProxyFallbackTarget(start, map[int64]Proxy{1: start, 2: backup}, now)

		require.True(t, change)
		require.NotNil(t, target)
		require.Equal(t, int64(2), *target)
	})

	t.Run("expired backup follows fallback chain", func(t *testing.T) {
		final := testProxyFallbackProxy(3, FallbackModeNone, nil, testProxyFallbackDays(30), now)
		expiredBackup := testProxyFallbackProxy(2, FallbackModeProxy, testProxyFallbackInt64(3), testProxyFallbackDays(-1), now)
		start := testProxyFallbackProxy(1, FallbackModeProxy, testProxyFallbackInt64(2), testProxyFallbackDays(-1), now)
		target, change := ResolveProxyFallbackTarget(start, map[int64]Proxy{1: start, 2: expiredBackup, 3: final}, now)

		require.True(t, change)
		require.NotNil(t, target)
		require.Equal(t, int64(3), *target)
	})

	t.Run("cycle keeps original binding", func(t *testing.T) {
		backup := testProxyFallbackProxy(2, FallbackModeProxy, testProxyFallbackInt64(1), testProxyFallbackDays(-1), now)
		start := testProxyFallbackProxy(1, FallbackModeProxy, testProxyFallbackInt64(2), testProxyFallbackDays(-1), now)
		target, change := ResolveProxyFallbackTarget(start, map[int64]Proxy{1: start, 2: backup}, now)

		require.False(t, change)
		require.Nil(t, target)
	})

	t.Run("chain tail direct fallback moves accounts to direct connection", func(t *testing.T) {
		backup := testProxyFallbackProxy(2, FallbackModeDirect, nil, testProxyFallbackDays(-1), now)
		start := testProxyFallbackProxy(1, FallbackModeProxy, testProxyFallbackInt64(2), testProxyFallbackDays(-1), now)
		target, change := ResolveProxyFallbackTarget(start, map[int64]Proxy{1: start, 2: backup}, now)

		require.True(t, change)
		require.Nil(t, target)
	})
}
