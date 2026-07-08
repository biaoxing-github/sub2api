package service

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
)

const (
	// CodexClientRestrictionReasonDisabled 表示账号未开启 codex_cli_only。
	CodexClientRestrictionReasonDisabled = "codex_cli_only_disabled"
	// CodexClientRestrictionReasonMatchedUA 表示请求命中官方客户端 UA 白名单。
	CodexClientRestrictionReasonMatchedUA = "official_client_user_agent_matched"
	// CodexClientRestrictionReasonMatchedOriginator 表示请求命中官方客户端 originator 白名单。
	CodexClientRestrictionReasonMatchedOriginator = "official_client_originator_matched"
	// CodexClientRestrictionReasonMatchedAllowedClient 表示请求命中账号级额外放行的命名客户端预设。
	CodexClientRestrictionReasonMatchedAllowedClient = "allowed_client_matched"
	// CodexClientRestrictionReasonMatchedGlobalAllowedClient 表示请求命中全局额外放行的命名客户端预设。
	CodexClientRestrictionReasonMatchedGlobalAllowedClient = "global_allowed_client_matched"
	// CodexClientRestrictionReasonNotMatchedUA 表示请求未命中官方客户端 UA 白名单。
	CodexClientRestrictionReasonNotMatchedUA = "official_client_user_agent_not_matched"
	// CodexClientRestrictionReasonVersionTooLow 表示 Codex 客户端版本低于上游要求。
	CodexClientRestrictionReasonVersionTooLow = "codex_client_version_too_low"
	// CodexClientRestrictionReasonVersionTooHigh 表示 Codex 客户端版本高于账号允许范围。
	CodexClientRestrictionReasonVersionTooHigh = "codex_client_version_too_high"
)

const CodexOfficialClientsOnlyMessage = "This account only allows Codex official clients"

// CodexClientRestrictionDetectionResult 是 codex_cli_only 统一检测入口结果。
type CodexClientRestrictionDetectionResult struct {
	Enabled         bool
	Matched         bool
	Reason          string
	DetectedVersion string
	MinCodexVersion string
	MaxCodexVersion string
}

func CodexClientRestrictionMessage(result CodexClientRestrictionDetectionResult) string {
	switch result.Reason {
	case CodexClientRestrictionReasonVersionTooLow:
		if result.DetectedVersion != "" && result.MinCodexVersion != "" {
			return fmt.Sprintf("Your Codex version (%s) is below the minimum required version (%s). Please update Codex.", result.DetectedVersion, result.MinCodexVersion)
		}
	case CodexClientRestrictionReasonVersionTooHigh:
		if result.DetectedVersion != "" && result.MaxCodexVersion != "" {
			return fmt.Sprintf("Your Codex version (%s) exceeds the maximum allowed version (%s). Please downgrade Codex to %s or lower.", result.DetectedVersion, result.MaxCodexVersion, result.MaxCodexVersion)
		}
	}
	return CodexOfficialClientsOnlyMessage
}

// CodexClientRestrictionDetector 定义 codex_cli_only 统一检测入口。
type CodexClientRestrictionDetector interface {
	Detect(c *gin.Context, account *Account, globalAllowedClients []string) CodexClientRestrictionDetectionResult
}

// OpenAICodexClientRestrictionDetector 为 OpenAI OAuth codex_cli_only 的默认实现。
type OpenAICodexClientRestrictionDetector struct{}

func NewOpenAICodexClientRestrictionDetector() *OpenAICodexClientRestrictionDetector {
	return &OpenAICodexClientRestrictionDetector{}
}

func (d *OpenAICodexClientRestrictionDetector) Detect(c *gin.Context, account *Account, globalAllowedClients []string) CodexClientRestrictionDetectionResult {
	if account == nil || !account.IsCodexCLIOnlyEnabled() {
		return CodexClientRestrictionDetectionResult{
			Enabled: false,
			Matched: false,
			Reason:  CodexClientRestrictionReasonDisabled,
		}
	}

	userAgent := ""
	originator := ""
	if c != nil {
		userAgent = c.GetHeader("User-Agent")
		originator = c.GetHeader("originator")
	}
	if openai.IsCodexOfficialClientRequest(userAgent) {
		return CodexClientRestrictionDetectionResult{
			Enabled: true,
			Matched: true,
			Reason:  CodexClientRestrictionReasonMatchedUA,
		}
	}
	if openai.IsCodexOfficialClientOriginator(originator) {
		return CodexClientRestrictionDetectionResult{
			Enabled: true,
			Matched: true,
			Reason:  CodexClientRestrictionReasonMatchedOriginator,
		}
	}
	if allowed := account.GetCodexCLIOnlyAllowedClients(); len(allowed) > 0 &&
		openai.MatchAllowedClients(userAgent, originator, allowed) {
		return CodexClientRestrictionDetectionResult{
			Enabled: true,
			Matched: true,
			Reason:  CodexClientRestrictionReasonMatchedAllowedClient,
		}
	}
	if len(globalAllowedClients) > 0 && openai.MatchAllowedClients(userAgent, originator, globalAllowedClients) {
		return CodexClientRestrictionDetectionResult{
			Enabled: true,
			Matched: true,
			Reason:  CodexClientRestrictionReasonMatchedGlobalAllowedClient,
		}
	}

	return CodexClientRestrictionDetectionResult{
		Enabled: true,
		Matched: false,
		Reason:  CodexClientRestrictionReasonNotMatchedUA,
	}
}
