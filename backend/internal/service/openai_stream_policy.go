package service

import (
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// openAIUpstreamErrorPolicyPhase 区分错误发生阶段，避免请求异常、HTTP 响应错误和流内失败混用同一避让语义。
type openAIUpstreamErrorPolicyPhase string

const (
	openAIUpstreamErrorPolicyPhaseRequest      openAIUpstreamErrorPolicyPhase = "request_phase"
	openAIUpstreamErrorPolicyPhaseHTTPResponse openAIUpstreamErrorPolicyPhase = "http_response"
	openAIUpstreamErrorPolicyPhaseStream       openAIUpstreamErrorPolicyPhase = "stream"
)

type openAIUpstreamErrorPolicyDecision struct {
	Phase            openAIUpstreamErrorPolicyPhase
	Category         string
	Label            string
	ActionLabel      OpenAIStreamActionLabel
	AvoidanceScope   string
	PathHealthReason string
	Retryable        bool
	AccountInvalid   bool
	RateLimited      bool
	RecordPathHealth bool
}

// Metadata 把策略判断写成 ops 可读字段，供 request timeline 和管理端动作模板直接展示。
func (d openAIUpstreamErrorPolicyDecision) Metadata() map[string]string {
	metadata := map[string]string{
		"error_phase":     string(d.Phase),
		"error_category":  strings.TrimSpace(d.Category),
		"error_label":     strings.TrimSpace(d.Label),
		"action_label":    string(d.ActionLabel),
		"stream_action":   string(d.ActionLabel),
		"avoidance_scope": strings.TrimSpace(d.AvoidanceScope),
	}
	if d.PathHealthReason != "" {
		metadata["path_health_reason"] = d.PathHealthReason
	}
	if d.Retryable {
		metadata["retryable"] = "true"
	}
	if d.AccountInvalid {
		metadata["account_invalid"] = "true"
	}
	if d.RateLimited {
		metadata["rate_limited"] = "true"
	}
	if d.RecordPathHealth {
		metadata["record_path_health"] = "true"
	}
	return metadata
}

// classifyOpenAIUpstreamErrorPolicy 复用统一上游错误分类器，再按 OpenAI 网关阶段映射为 retry/avoid 动作。
func classifyOpenAIUpstreamErrorPolicy(phase openAIUpstreamErrorPolicyPhase, input UpstreamErrorInput) openAIUpstreamErrorPolicyDecision {
	classification := ClassifyUpstreamError(input)
	action := OpenAIStreamActionRetryNoAvoidance
	recordPathHealth := false

	switch classification.Category {
	case UpstreamErrorCategoryUnauthorized, UpstreamErrorCategoryReauthRequired, UpstreamErrorCategoryQuota, UpstreamErrorCategoryRateLimited, UpstreamErrorCategoryClientIPCircuitOpen:
		action = OpenAIStreamActionAvoidAccountTTL
	case UpstreamErrorCategoryCloudflareWAF, UpstreamErrorCategoryUnexpectedEOF, UpstreamErrorCategoryHeaderTimeout, UpstreamErrorCategoryTimeout:
		action = OpenAIStreamActionAvoidUpstreamBucketTTL
	case UpstreamErrorCategoryUpstream5xx:
		// 502/503/504 等上游 5xx 错误：触发账号级冷却
		// 确保频繁 502 的账号被临时摘除，避免持续调度到不可用账号
		action = OpenAIStreamActionAvoidAccountTTL
	case UpstreamErrorCategoryBusinessLimited, UpstreamErrorCategoryPreviousResponseNotFound, UpstreamErrorCategoryRequestTooLarge:
		action = OpenAIStreamActionRetryNextAccount
	case UpstreamErrorCategoryUpstreamError:
		action = OpenAIStreamActionRetryNextAccount
	case UpstreamErrorCategoryOK:
		action = OpenAIStreamActionRetryNoAvoidance
	default:
		action = OpenAIStreamActionRetryNextAccount
	}

	if phase == openAIUpstreamErrorPolicyPhaseRequest && action != OpenAIStreamActionRetryNoAvoidance {
		action = OpenAIStreamActionRetryNextAccount
	}
	if classification.LineDegraded || strings.TrimSpace(classification.PathHealthReason) != "" {
		recordPathHealth = true
	}
	if phase == openAIUpstreamErrorPolicyPhaseStream && classification.Category == UpstreamErrorCategoryBusinessLimited {
		recordPathHealth = false
	}

	return openAIUpstreamErrorPolicyDecision{
		Phase:            phase,
		Category:         classification.Category,
		Label:            classification.Label,
		ActionLabel:      action,
		AvoidanceScope:   openAIStreamActionAvoidanceScope(action, string(phase)),
		PathHealthReason: classification.PathHealthReason,
		Retryable:        classification.Retryable,
		AccountInvalid:   classification.AccountInvalid,
		RateLimited:      classification.RateLimited,
		RecordPathHealth: recordPathHealth,
	}
}

// openAIStreamInterceptDecision 表达一条 response.failed 内置规则命中后的动作和审计字段。
type openAIStreamInterceptDecision struct {
	RuleID                      string
	Priority                    int
	DescriptionKey              string
	ActionLabel                 OpenAIStreamActionLabel
	MatchField                  string
	MatchValue                  string
	ReasonCategory              string
	FailoverBeforeOutput        bool
	GatewayRetryableAfterOutput bool
	DropOriginalEvent           bool
}

// Metadata 输出流式拦截策略命中信息，便于管理端解释动作来源。
func (d openAIStreamInterceptDecision) Metadata() map[string]string {
	metadata := map[string]string{
		"stream_rule_id":       strings.TrimSpace(d.RuleID),
		"stream_rule_priority": strconv.Itoa(d.Priority),
		"stream_action":        string(d.ActionLabel),
		"action_label":         string(d.ActionLabel),
		"avoidance_scope":      openAIStreamActionAvoidanceScope(d.ActionLabel, string(openAIUpstreamErrorPolicyPhaseStream)),
		"reason_scope":         string(openAIUpstreamErrorPolicyPhaseStream),
		"match_field":          strings.TrimSpace(d.MatchField),
		"description_key":      strings.TrimSpace(d.DescriptionKey),
	}
	if d.MatchValue != "" {
		metadata["match_value"] = d.MatchValue
	}
	if d.ReasonCategory != "" {
		metadata["error_category"] = d.ReasonCategory
	}
	if d.FailoverBeforeOutput {
		metadata["failover_before_output"] = "true"
	}
	if d.GatewayRetryableAfterOutput {
		metadata["gateway_retryable_after_output"] = "true"
	}
	if d.DropOriginalEvent {
		metadata["drop_original_event"] = "true"
	}
	return metadata
}

// openAIStreamInterceptRule 是后续开放配置前的固定内置规则形态，先限制匹配字段和动作范围。
type openAIStreamInterceptRule struct {
	ID             string
	Priority       int
	DescriptionKey string
	ActionLabel    OpenAIStreamActionLabel
	JSONPath       string
	ContainsAny    []string
	Category       string
}

// openAIStreamInterceptBuiltInRules 覆盖 quota/billing、容量过载、策略拒绝三类高频 response.failed。
var openAIStreamInterceptBuiltInRules = []openAIStreamInterceptRule{
	{
		ID:             "openai_stream_quota_or_billing",
		Priority:       10,
		DescriptionKey: "quotaOrBilling",
		ActionLabel:    OpenAIStreamActionAvoidAccountTTL,
		JSONPath:       "error.code",
		ContainsAny:    []string{"quota", "billing", "insufficient_quota", "insufficient_balance", "usage_limit"},
		Category:       UpstreamErrorCategoryQuota,
	},
	{
		ID:             "openai_stream_quota_or_billing",
		Priority:       11,
		DescriptionKey: "quotaOrBilling",
		ActionLabel:    OpenAIStreamActionAvoidAccountTTL,
		JSONPath:       "response.error.code",
		ContainsAny:    []string{"quota", "billing", "insufficient_quota", "insufficient_balance", "usage_limit"},
		Category:       UpstreamErrorCategoryQuota,
	},
	{
		ID:             "openai_stream_capacity_or_overload",
		Priority:       20,
		DescriptionKey: "capacityOrOverload",
		ActionLabel:    OpenAIStreamActionAvoidUpstreamBucketTTL,
		JSONPath:       "error.message",
		ContainsAny:    []string{"capacity", "overloaded", "slow_down", "server overloaded"},
		Category:       UpstreamErrorCategoryUpstreamError,
	},
	{
		ID:             "openai_stream_capacity_or_overload",
		Priority:       21,
		DescriptionKey: "capacityOrOverload",
		ActionLabel:    OpenAIStreamActionAvoidUpstreamBucketTTL,
		JSONPath:       "response.error.message",
		ContainsAny:    []string{"capacity", "overloaded", "slow_down", "server overloaded"},
		Category:       UpstreamErrorCategoryUpstreamError,
	},
	{
		ID:             "openai_stream_policy_or_invalid_request",
		Priority:       30,
		DescriptionKey: "policyOrInvalidRequest",
		ActionLabel:    OpenAIStreamActionRetryNextAccount,
		JSONPath:       "error.type",
		ContainsAny:    []string{"policy", "safety", "content_policy", "invalid_request"},
		Category:       UpstreamErrorCategoryBusinessLimited,
	},
}

// openAIClassifyStreamInterceptDecision 优先匹配内置流规则，未命中时回落到统一错误策略。
func openAIClassifyStreamInterceptDecision(payload []byte, message string) openAIStreamInterceptDecision {
	for _, rule := range openAIStreamInterceptBuiltInRules {
		value := strings.TrimSpace(gjson.GetBytes(payload, rule.JSONPath).String())
		if value == "" {
			continue
		}
		lower := strings.ToLower(value)
		for _, needle := range rule.ContainsAny {
			if strings.Contains(lower, strings.ToLower(needle)) {
				return openAIStreamInterceptDecision{
					RuleID:                      rule.ID,
					Priority:                    rule.Priority,
					DescriptionKey:              rule.DescriptionKey,
					ActionLabel:                 rule.ActionLabel,
					MatchField:                  rule.JSONPath,
					MatchValue:                  value,
					ReasonCategory:              rule.Category,
					FailoverBeforeOutput:        true,
					GatewayRetryableAfterOutput: true,
					DropOriginalEvent:           true,
				}
			}
		}
	}

	streamMessage := firstNonEmptyString(message, extractOpenAISSEErrorMessage(payload))
	policy := classifyOpenAIUpstreamErrorPolicy(openAIUpstreamErrorPolicyPhaseStream, UpstreamErrorInput{
		Message: streamMessage,
		Body:    payload,
	})
	descriptionKey := "defaultResponseFailed"
	if policy.Category == UpstreamErrorCategoryBusinessLimited {
		descriptionKey = "policyOrInvalidRequest"
	}
	return openAIStreamInterceptDecision{
		RuleID:                      "openai_stream_default_response_failed",
		Priority:                    1000,
		DescriptionKey:              descriptionKey,
		ActionLabel:                 policy.ActionLabel,
		MatchField:                  "event.type",
		MatchValue:                  "response.failed",
		ReasonCategory:              policy.Category,
		FailoverBeforeOutput:        true,
		GatewayRetryableAfterOutput: true,
		DropOriginalEvent:           true,
	}
}
