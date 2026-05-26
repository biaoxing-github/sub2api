package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestOpsServiceGetRequestTimelineReturnsEmptyForMissingRequest(t *testing.T) {
	svc := NewOpsService(&opsRepoMock{}, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)

	timeline, err := svc.GetRequestTimeline(context.Background(), "req_missing")
	if err != nil {
		t.Fatalf("GetRequestTimeline() error = %v", err)
	}
	if timeline.RequestID != "req_missing" || timeline.Status != "unknown" || len(timeline.Events) != 0 {
		t.Fatalf("timeline = %#v", timeline)
	}
}

func TestOpsServiceGetRequestTimelineBuildsRecordedEvent(t *testing.T) {
	startedAt := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	durationMs := 1234
	statusCode := 200
	var seenFilter *OpsRequestDetailFilter
	svc := NewOpsService(&opsRepoMock{
		ListRequestDetailsFn: func(ctx context.Context, filter *OpsRequestDetailFilter) ([]*OpsRequestDetail, int64, error) {
			seenFilter = filter
			return []*OpsRequestDetail{{
				Kind:       OpsRequestKindSuccess,
				CreatedAt:  startedAt,
				RequestID:  "req_123",
				Platform:   PlatformOpenAI,
				Model:      "gpt-5.5",
				DurationMs: &durationMs,
				StatusCode: &statusCode,
				Stream:     true,
			}}, 1, nil
		},
	}, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)

	timeline, err := svc.GetRequestTimeline(context.Background(), " req_123 ")
	if err != nil {
		t.Fatalf("GetRequestTimeline() error = %v", err)
	}
	if seenFilter == nil || seenFilter.RequestID != "req_123" || seenFilter.Kind != "all" {
		t.Fatalf("filter = %#v", seenFilter)
	}
	if timeline.Status != "success" || timeline.StartedAt == nil || !timeline.StartedAt.Equal(startedAt) {
		t.Fatalf("timeline = %#v", timeline)
	}
	if timeline.EndedAt == nil || !timeline.EndedAt.Equal(startedAt.Add(1234*time.Millisecond)) {
		t.Fatalf("ended_at = %v", timeline.EndedAt)
	}
	if len(timeline.Events) != 1 || timeline.Events[0].EventType != "request_recorded" {
		t.Fatalf("events = %#v", timeline.Events)
	}
}

func TestOpsServiceGetRequestTimelineIncludesLatencyAndUpstreamErrors(t *testing.T) {
	startedAt := time.Date(2026, 5, 26, 7, 0, 0, 0, time.UTC)
	durationMs := 5000
	statusCode := 502
	upstreamStatus := 0
	upstreamLatency := int64(3000)
	responseLatency := int64(100)
	accountID := int64(123)

	svc := NewOpsService(&opsRepoMock{
		ListRequestDetailsFn: func(ctx context.Context, filter *OpsRequestDetailFilter) ([]*OpsRequestDetail, int64, error) {
			return []*OpsRequestDetail{{
				Kind:                 OpsRequestKindError,
				CreatedAt:            startedAt,
				RequestID:            "req_timeout",
				Platform:             PlatformOpenAI,
				Model:                "gpt-5.5",
				DurationMs:           &durationMs,
				StatusCode:           &statusCode,
				Message:              "context deadline exceeded",
				AccountID:            &accountID,
				AccountName:          "encore",
				Stream:               true,
				UpstreamStatusCode:   &upstreamStatus,
				UpstreamErrorMessage: "Post \"https://api.okcodex.cn/v1/responses\": context deadline exceeded",
				UpstreamErrors: []*OpsUpstreamErrorEvent{{
					AtUnixMs:           startedAt.Add(3 * time.Second).UnixMilli(),
					Platform:           PlatformOpenAI,
					AccountID:          accountID,
					AccountName:        "encore",
					Kind:               "failover",
					Message:            "context deadline exceeded",
					UpstreamStatusCode: 0,
					UpstreamURL:        "https://api.okcodex.cn/v1/responses",
				}},
				UpstreamLatencyMs: &upstreamLatency,
				ResponseLatencyMs: &responseLatency,
			}}, 1, nil
		},
	}, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)

	timeline, err := svc.GetRequestTimeline(context.Background(), "req_timeout")
	if err != nil {
		t.Fatalf("GetRequestTimeline() error = %v", err)
	}
	if len(timeline.Events) < 3 {
		t.Fatalf("events = %#v, want request + latency + upstream error", timeline.Events)
	}
	var sawUpstreamLatency, sawUpstreamError bool
	for _, event := range timeline.Events {
		if event.Phase == "upstream" && event.EventType == "phase_latency_recorded" && event.LatencyMs != nil && *event.LatencyMs == upstreamLatency {
			sawUpstreamLatency = true
		}
		if event.Phase == "upstream" && event.EventType == "upstream_failover" && event.Reason == "context deadline exceeded" {
			sawUpstreamError = true
		}
	}
	if !sawUpstreamLatency || !sawUpstreamError {
		t.Fatalf("timeline missing latency=%v upstream_error=%v events=%#v", sawUpstreamLatency, sawUpstreamError, timeline.Events)
	}
}

func TestOpsServiceGetCodexDiagnosisTreatsContextDeadlineAsUpstreamTimeout(t *testing.T) {
	startedAt := time.Date(2026, 5, 26, 8, 0, 0, 0, time.UTC)
	statusCode := 502
	svc := NewOpsService(&opsRepoMock{
		ListRequestDetailsFn: func(ctx context.Context, filter *OpsRequestDetailFilter) ([]*OpsRequestDetail, int64, error) {
			return []*OpsRequestDetail{{
				Kind:       OpsRequestKindError,
				CreatedAt:  startedAt,
				RequestID:  "req_context_deadline",
				Platform:   PlatformOpenAI,
				StatusCode: &statusCode,
				Message:    "Post \"https://api.okcodex.cn/v1/responses\": context deadline exceeded",
				Stream:     true,
			}}, 1, nil
		},
	}, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)

	diagnosis, err := svc.GetCodexDiagnosis(context.Background(), "req_context_deadline")
	if err != nil {
		t.Fatalf("GetCodexDiagnosis() error = %v", err)
	}
	if diagnosis.Status != "upstream_timeout" {
		t.Fatalf("status = %q, want upstream_timeout, diagnosis=%#v", diagnosis.Status, diagnosis)
	}
}
