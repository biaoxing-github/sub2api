//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWSPassthroughUsageMeta_InitFromFirstFrameUsesMappedModelCandidate(t *testing.T) {
	body := []byte(`{"type":"response.create","model":"sol","reasoning":{"effort":"max"}}`)
	meta := newOpenAIWSPassthroughUsageMeta("sol", body)

	meta.initFromFirstFrame(body, "gpt-5.6-sol")

	got := meta.reasoningEffort.Load()
	require.NotNil(t, got)
	require.Equal(t, "max", *got)
}

func TestWSPassthroughUsageMeta_UpdateFromResponseCreateUsesMappedModelCandidate(t *testing.T) {
	body := []byte(`{"type":"response.create","model":"sol","reasoning":{"effort":"max"}}`)
	meta := newOpenAIWSPassthroughUsageMeta("sol", body)

	meta.updateFromResponseCreate(body, "gpt-5.6-sol", "sol")

	got := meta.reasoningEffort.Load()
	require.NotNil(t, got)
	require.Equal(t, "max", *got)
}
