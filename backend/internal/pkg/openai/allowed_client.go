package openai

import "strings"

const (
	// AllowedClientClaudeCode 对应 Claude Code 的 Codex 插件。
	AllowedClientClaudeCode = "claude_code"
)

// AllowedClientEntry 描述一个被额外放行的非官方 Codex 客户端签名。
type AllowedClientEntry struct {
	Originator string
	UAContains []string
}

var allowedClientRegistry = map[string]AllowedClientEntry{
	AllowedClientClaudeCode: {
		Originator: "Claude Code",
		UAContains: []string{"Claude Code/"},
	},
}

// IsAllowedClientMatch 判断请求头是否命中给定的额外客户端签名。
// originator 必须等值匹配，UAContains 中每一项都必须出现在 User-Agent 中。
func IsAllowedClientMatch(userAgent, originator string, entry AllowedClientEntry) bool {
	wantOriginator := normalizeCodexClientHeader(entry.Originator)
	if wantOriginator == "" || normalizeCodexClientHeader(originator) != wantOriginator {
		return false
	}
	if len(entry.UAContains) == 0 {
		return false
	}
	ua := normalizeCodexClientHeader(userAgent)
	for _, marker := range entry.UAContains {
		normalizedMarker := normalizeCodexClientHeader(marker)
		if normalizedMarker == "" || !strings.Contains(ua, normalizedMarker) {
			return false
		}
	}
	return true
}

// MatchAllowedClients 判断请求头是否命中 clientIDs 引用的任一预设签名。
func MatchAllowedClients(userAgent, originator string, clientIDs []string) bool {
	for _, id := range clientIDs {
		entry, ok := allowedClientRegistry[normalizeCodexClientHeader(id)]
		if !ok {
			continue
		}
		if IsAllowedClientMatch(userAgent, originator, entry) {
			return true
		}
	}
	return false
}
