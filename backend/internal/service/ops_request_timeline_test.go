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
