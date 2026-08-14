package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildGrokXSearchResponsesBody(t *testing.T) {
	images := true
	videos := false
	body, err := buildGrokXSearchResponsesBody(grokStandaloneXSearchRequest{
		Query:                    "latest posts from xAI",
		AllowedXHandles:          []string{"xai"},
		ExcludedXHandles:         []string{"spam"},
		FromDate:                 "2026-08-01",
		ToDate:                   "2026-08-10",
		EnableImageUnderstanding: &images,
		EnableVideoUnderstanding: &videos,
	}, "grok-4.5", 3)
	require.NoError(t, err)
	require.Equal(t, "grok-4.5", gjson.GetBytes(body, "model").String())
	require.Equal(t, "required", gjson.GetBytes(body, "tool_choice").String())
	require.Equal(t, "x_search", gjson.GetBytes(body, "tools.0.type").String())
	require.Equal(t, "xai", gjson.GetBytes(body, "tools.0.allowed_x_handles.0").String())
	require.Equal(t, "spam", gjson.GetBytes(body, "tools.0.excluded_x_handles.0").String())
	require.Equal(t, "2026-08-01", gjson.GetBytes(body, "tools.0.from_date").String())
	require.True(t, gjson.GetBytes(body, "tools.0.enable_image_understanding").Bool())
	require.False(t, gjson.GetBytes(body, "tools.0.enable_video_understanding").Bool())
	require.Equal(t, "x_search_call.action.sources", gjson.GetBytes(body, "include.0").String())
	require.Contains(t, gjson.GetBytes(body, "input").String(), "Return at most 3 unique results")
}

func TestExtractGrokXSearchSourcesUsesDeclaredSourcesOnly(t *testing.T) {
	body := []byte(`{
		"output":[
			{"type":"x_search_call","action":{"sources":[
				{"url":"https://x.com/xai/status/1","title":"xAI"},
				{"url":"https://example.com/fallback","title":"2","snippet":"raw source"}
			]}},
			{"type":"message","content":[{"type":"output_text","text":"{\"results\":[{\"url\":\"https://x.com/xai/status/1#fragment\",\"title\":\"xAI update\",\"snippet\":\"new release\"},{\"url\":\"https://untrusted.example/\",\"title\":\"ignore\"}]}"}]}
		]
	}`)
	results := extractGrokXSearchSources(body, 5)
	require.Len(t, results, 2)
	require.Equal(t, "https://x.com/xai/status/1", results[0].URL)
	require.Equal(t, "xAI update", results[0].Title)
	require.Equal(t, "new release", results[0].Snippet)
	require.Equal(t, "https://example.com/fallback", results[1].URL)
	require.Equal(t, "example.com", results[1].Title)
	require.Equal(t, "raw source", results[1].Snippet)
}

func TestNormalizeGrokXSearchMaxResults(t *testing.T) {
	require.Equal(t, defaultGrokXSearchResults, normalizeGrokXSearchMaxResults(0))
	require.Equal(t, maxGrokXSearchResults, normalizeGrokXSearchMaxResults(maxGrokXSearchResults+1))
	require.Equal(t, 7, normalizeGrokXSearchMaxResults(7))
}
