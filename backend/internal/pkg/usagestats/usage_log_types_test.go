package usagestats

import "testing"

func TestDashboardStatsRecalculateDerivedFields(t *testing.T) {
	stats := &DashboardStats{
		TotalInputTokens:     900,
		TotalOutputTokens:    400,
		TotalCacheReadTokens: 100,
		TodayInputTokens:     120,
		TodayOutputTokens:    60,
		TodayCacheReadTokens: 30,
	}

	stats.RecalculateDerivedFields()

	if stats.TotalTokens != 1400 {
		t.Fatalf("TotalTokens=%d want 1400", stats.TotalTokens)
	}
	if stats.TodayTokens != 210 {
		t.Fatalf("TodayTokens=%d want 210", stats.TodayTokens)
	}
	if stats.TotalCacheReadRatio != 0.1 {
		t.Fatalf("TotalCacheReadRatio=%v want 0.1", stats.TotalCacheReadRatio)
	}
	if stats.TodayCacheReadRatio != 0.2 {
		t.Fatalf("TodayCacheReadRatio=%v want 0.2", stats.TodayCacheReadRatio)
	}
}

func TestDashboardStatsRecalculateDerivedFieldsZeroInputSide(t *testing.T) {
	stats := &DashboardStats{
		TotalOutputTokens:    300,
		TotalCacheReadTokens: 0,
		TodayOutputTokens:    100,
		TodayCacheReadTokens: 0,
	}

	stats.RecalculateDerivedFields()

	if stats.TotalCacheReadRatio != 0 {
		t.Fatalf("TotalCacheReadRatio=%v want 0", stats.TotalCacheReadRatio)
	}
	if stats.TodayCacheReadRatio != 0 {
		t.Fatalf("TodayCacheReadRatio=%v want 0", stats.TodayCacheReadRatio)
	}
}

func TestIsValidModelSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{name: "requested", source: ModelSourceRequested, want: true},
		{name: "upstream", source: ModelSourceUpstream, want: true},
		{name: "mapping", source: ModelSourceMapping, want: true},
		{name: "invalid", source: "foobar", want: false},
		{name: "empty", source: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidModelSource(tc.source); got != tc.want {
				t.Fatalf("IsValidModelSource(%q)=%v want %v", tc.source, got, tc.want)
			}
		})
	}
}

func TestNormalizeModelSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "requested", source: ModelSourceRequested, want: ModelSourceRequested},
		{name: "upstream", source: ModelSourceUpstream, want: ModelSourceUpstream},
		{name: "mapping", source: ModelSourceMapping, want: ModelSourceMapping},
		{name: "invalid falls back", source: "foobar", want: ModelSourceRequested},
		{name: "empty falls back", source: "", want: ModelSourceRequested},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeModelSource(tc.source); got != tc.want {
				t.Fatalf("NormalizeModelSource(%q)=%q want %q", tc.source, got, tc.want)
			}
		})
	}
}
