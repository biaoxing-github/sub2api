package service

import (
	"strings"
	"time"
)

// ResolveProxyFallbackTarget 计算过期代理绑定账号的目标代理。
// change=false 表示保持原绑定，target=nil 且 change=true 表示改为直连。
func ResolveProxyFallbackTarget(start Proxy, byID map[int64]Proxy, now time.Time) (*int64, bool) {
	switch normalizeProxyFallbackMode(start.FallbackMode) {
	case FallbackModeDirect:
		return nil, true
	case FallbackModeProxy:
		visited := map[int64]struct{}{start.ID: {}}
		curID := start.BackupProxyID
		for {
			if curID == nil {
				return nil, false
			}
			if _, seen := visited[*curID]; seen {
				return nil, false
			}
			next, ok := byID[*curID]
			if !ok {
				return nil, false
			}
			if next.Status != StatusExpired && !(&next).IsExpired(now) {
				id := next.ID
				return &id, true
			}
			visited[*curID] = struct{}{}
			switch normalizeProxyFallbackMode(next.FallbackMode) {
			case FallbackModeDirect:
				return nil, true
			case FallbackModeProxy:
				curID = next.BackupProxyID
			default:
				return nil, false
			}
		}
	default:
		return nil, false
	}
}

func normalizeProxyFallbackMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case FallbackModeProxy:
		return FallbackModeProxy
	case FallbackModeDirect:
		return FallbackModeDirect
	default:
		return FallbackModeNone
	}
}
