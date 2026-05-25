package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type OpsRequestKind string

const (
	OpsRequestKindSuccess OpsRequestKind = "success"
	OpsRequestKindError   OpsRequestKind = "error"
)

// OpsRequestDetail is a request-level view across success (usage_logs) and error (ops_error_logs).
// It powers "request drilldown" UIs without exposing full request bodies for successful requests.
type OpsRequestDetail struct {
	Kind      OpsRequestKind `json:"kind"`
	CreatedAt time.Time      `json:"created_at"`
	RequestID string         `json:"request_id"`

	Platform string `json:"platform,omitempty"`
	Model    string `json:"model,omitempty"`

	DurationMs *int `json:"duration_ms,omitempty"`
	StatusCode *int `json:"status_code,omitempty"`

	// When Kind == "error", ErrorID links to /admin/ops/errors/:id.
	ErrorID *int64 `json:"error_id,omitempty"`

	Phase    string `json:"phase,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`

	UserID    *int64 `json:"user_id,omitempty"`
	APIKeyID  *int64 `json:"api_key_id,omitempty"`
	AccountID *int64 `json:"account_id,omitempty"`
	GroupID   *int64 `json:"group_id,omitempty"`

	Stream bool `json:"stream"`

	AccountName      string `json:"account_name,omitempty"`
	RequestedModel   string `json:"requested_model,omitempty"`
	UpstreamEndpoint string `json:"upstream_endpoint,omitempty"`

	InputTokens  *int `json:"input_tokens,omitempty"`
	OutputTokens *int `json:"output_tokens,omitempty"`
	TotalTokens  *int `json:"total_tokens,omitempty"`
	FirstTokenMs *int `json:"first_token_ms,omitempty"`

	TotalCost  string `json:"total_cost,omitempty"`
	ActualCost string `json:"actual_cost,omitempty"`
}

type OpsRequestDetailFilter struct {
	StartTime *time.Time
	EndTime   *time.Time

	// kind: success|error|all
	Kind string

	Platform string
	GroupID  *int64

	UserID    *int64
	APIKeyID  *int64
	AccountID *int64

	Model     string
	RequestID string
	Query     string

	MinDurationMs *int
	MaxDurationMs *int

	// Sort: created_at_desc (default) or duration_desc.
	Sort string

	Page     int
	PageSize int
}

func (f *OpsRequestDetailFilter) Normalize() (page, pageSize int, startTime, endTime time.Time) {
	page = 1
	pageSize = 50
	endTime = time.Now()
	startTime = endTime.Add(-1 * time.Hour)

	if f == nil {
		return page, pageSize, startTime, endTime
	}

	if f.Page > 0 {
		page = f.Page
	}
	if f.PageSize > 0 {
		pageSize = f.PageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}

	if f.EndTime != nil {
		endTime = *f.EndTime
	}
	if f.StartTime != nil {
		startTime = *f.StartTime
	} else if f.EndTime != nil {
		startTime = endTime.Add(-1 * time.Hour)
	}

	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}

	return page, pageSize, startTime, endTime
}

type OpsRequestDetailList struct {
	Items    []*OpsRequestDetail `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type OpsRequestTimelineEvent struct {
	At          time.Time      `json:"at"`
	Phase       string         `json:"phase"`
	EventType   string         `json:"event_type"`
	AccountID   *int64         `json:"account_id,omitempty"`
	AccountName string         `json:"account_name,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	LatencyMs   *int64         `json:"latency_ms,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

type OpsRequestTimeline struct {
	RequestID       string                    `json:"request_id"`
	ClientRequestID string                    `json:"client_request_id,omitempty"`
	StartedAt       *time.Time                `json:"started_at,omitempty"`
	EndedAt         *time.Time                `json:"ended_at,omitempty"`
	Status          string                    `json:"status"`
	Events          []OpsRequestTimelineEvent `json:"events"`
}

type OpsCodexDiagnosis struct {
	RequestID       string                    `json:"request_id"`
	Status          string                    `json:"status"`
	Headline        string                    `json:"headline"`
	Path            map[string]any            `json:"path,omitempty"`
	Latency         map[string]any            `json:"latency,omitempty"`
	Context         map[string]any            `json:"context,omitempty"`
	Routing         map[string]any            `json:"routing,omitempty"`
	Usage           map[string]any            `json:"usage,omitempty"`
	SuggestedAction string                    `json:"suggested_action,omitempty"`
	Timeline        []OpsRequestTimelineEvent `json:"timeline,omitempty"`
}

func (s *OpsService) ListRequestDetails(ctx context.Context, filter *OpsRequestDetailFilter) (*OpsRequestDetailList, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return &OpsRequestDetailList{
			Items:    []*OpsRequestDetail{},
			Total:    0,
			Page:     1,
			PageSize: 50,
		}, nil
	}

	page, pageSize, startTime, endTime := filter.Normalize()
	filterCopy := &OpsRequestDetailFilter{}
	if filter != nil {
		*filterCopy = *filter
	}
	filterCopy.Page = page
	filterCopy.PageSize = pageSize
	filterCopy.StartTime = &startTime
	filterCopy.EndTime = &endTime

	items, total, err := s.opsRepo.ListRequestDetails(ctx, filterCopy)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*OpsRequestDetail{}
	}

	return &OpsRequestDetailList{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *OpsService) GetRequestTimeline(ctx context.Context, requestID string) (*OpsRequestTimeline, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, infraerrors.BadRequest("INVALID_REQUEST_ID", "request_id is required")
	}
	timeline := &OpsRequestTimeline{
		RequestID: requestID,
		Status:    "unknown",
		Events:    []OpsRequestTimelineEvent{},
	}
	if s.opsRepo == nil {
		return timeline, nil
	}
	filter := &OpsRequestDetailFilter{
		RequestID: requestID,
		Kind:      "all",
		Page:      1,
		PageSize:  1,
	}
	now := time.Now()
	start := now.Add(-72 * time.Hour)
	filter.StartTime = &start
	filter.EndTime = &now
	items, _, err := s.opsRepo.ListRequestDetails(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 || items[0] == nil {
		return timeline, nil
	}
	item := items[0]
	timeline.StartedAt = &item.CreatedAt
	if item.DurationMs != nil {
		endedAt := item.CreatedAt.Add(time.Duration(*item.DurationMs) * time.Millisecond)
		timeline.EndedAt = &endedAt
	}
	timeline.Status = string(item.Kind)
	timeline.Events = append(timeline.Events, OpsRequestTimelineEvent{
		At:          item.CreatedAt,
		Phase:       "request",
		EventType:   "request_recorded",
		Reason:      item.Message,
		AccountID:   item.AccountID,
		AccountName: item.AccountName,
		Details: map[string]any{
			"kind":              item.Kind,
			"platform":          item.Platform,
			"model":             item.Model,
			"requested_model":   item.RequestedModel,
			"status_code":       item.StatusCode,
			"stream":            item.Stream,
			"user_id":           item.UserID,
			"api_key_id":        item.APIKeyID,
			"group_id":          item.GroupID,
			"duration_ms":       item.DurationMs,
			"first_token_ms":    item.FirstTokenMs,
			"input_tokens":      item.InputTokens,
			"output_tokens":     item.OutputTokens,
			"total_tokens":      item.TotalTokens,
			"total_cost":        item.TotalCost,
			"actual_cost":       item.ActualCost,
			"upstream_endpoint": item.UpstreamEndpoint,
		},
	})
	return timeline, nil
}

func (s *OpsService) GetCodexDiagnosis(ctx context.Context, requestID string) (*OpsCodexDiagnosis, error) {
	timeline, err := s.GetRequestTimeline(ctx, requestID)
	if err != nil {
		return nil, err
	}
	diagnosis := &OpsCodexDiagnosis{
		RequestID: strings.TrimSpace(requestID),
		Status:    timeline.Status,
		Headline:  "未找到足够的 Codex 请求诊断信息",
		Timeline:  timeline.Events,
		Latency:   map[string]any{},
		Path:      map[string]any{},
		Context:   map[string]any{},
		Routing:   map[string]any{},
		Usage:     map[string]any{},
	}
	if timeline == nil || len(timeline.Events) == 0 {
		diagnosis.SuggestedAction = "确认 ops 监控已开启，并用 request_id 查询最近 72 小时内的请求。"
		return diagnosis, nil
	}
	var lastReason string
	for _, event := range timeline.Events {
		if event.Reason != "" {
			lastReason = event.Reason
		}
		if event.AccountID != nil {
			diagnosis.Routing["account_id"] = *event.AccountID
		}
		if event.AccountName != "" {
			diagnosis.Routing["account_name"] = event.AccountName
		}
		if event.LatencyMs != nil {
			diagnosis.Latency[event.Phase+"_latency_ms"] = *event.LatencyMs
		}
		for key, value := range event.Details {
			switch key {
			case "status_code", "stream", "model", "platform":
				diagnosis.Path[key] = value
			case "duration_ms", "first_token_ms", "time_to_first_token_ms", "ttft_ms", "header_wait_ms", "upstream_latency_ms":
				diagnosis.Latency[key] = value
			case "context_replay_reason", "context_continuity", "journal_reason", "replay_safe":
				diagnosis.Context[key] = value
			case "requested_model", "upstream_endpoint", "user_id", "api_key_id", "group_id":
				diagnosis.Routing[key] = value
			case "input_tokens", "output_tokens", "total_tokens", "total_cost", "actual_cost":
				diagnosis.Usage[key] = value
			}
		}
	}
	reason := strings.ToLower(lastReason)
	switch {
	case strings.Contains(reason, "context") || strings.Contains(reason, "replay") || strings.Contains(reason, "function_call_output") || strings.Contains(reason, "encrypted"):
		diagnosis.Status = "protected"
		diagnosis.Headline = "不可安全重放，已保护会话"
		diagnosis.SuggestedAction = "继续原账号或新开会话；当前续链依赖上游状态，不能静默跨账号重放。"
	case strings.Contains(reason, "timeout awaiting response headers") || strings.Contains(reason, "timed out waiting"):
		diagnosis.Status = "header_timeout"
		diagnosis.Headline = "上游响应头等待超时"
		diagnosis.SuggestedAction = "优先查看 path health 是否熔断该账号/代理/endpoint；未输出前可安全快速切号。"
	case strings.Contains(reason, "unexpected eof") || strings.Contains(reason, "eof"):
		diagnosis.Status = "unexpected_eof"
		diagnosis.Headline = "上游连接提前断开"
		diagnosis.SuggestedAction = "检查代理和 HTTP/2 线路；连续 EOF 应临时降权该 path。"
	case strings.Contains(reason, "401") || strings.Contains(reason, "unauthorized"):
		diagnosis.Status = "unauthorized"
		diagnosis.Headline = "账号认证失败"
		diagnosis.SuggestedAction = "刷新账号凭据或暂停该账号，避免继续调度。"
	case strings.Contains(reason, "429") || strings.Contains(reason, "rate limit"):
		diagnosis.Status = "rate_limited"
		diagnosis.Headline = "上游限流"
		diagnosis.SuggestedAction = "等待 reset 时间或让调度器切到实时余额确认通过的候选账号。"
	default:
		diagnosis.Headline = "请求已记录，可查看 timeline 细节"
		diagnosis.SuggestedAction = "如果仍感觉卡顿，重点看 TTFT、upstream latency 和账号切换次数。"
	}
	return diagnosis, nil
}
