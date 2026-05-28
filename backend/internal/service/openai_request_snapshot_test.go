package service

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIRequestSnapshotFromGinStoresContextSignals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := &gin.Context{}
	c.Set("request_id", "req_test")
	c.Set("api_key_id", int64(7))
	c.Set("group_id", int64(3))

	obs := ClassifyOpenAIContextMigration([]byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`), false)
	snapshot, ok := buildOpenAIRequestSnapshotFromGin(c, &Account{ID: 11}, []byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`), "gpt-5.4", "pcache", obs, 24)

	require.True(t, ok)
	require.Equal(t, "req_test", snapshot.RequestID)
	require.Equal(t, "pcache", snapshot.PromptCacheKey)
	require.Equal(t, "gpt-5.4", snapshot.Model)
	require.NotEmpty(t, snapshot.RequestBodySHA256)
	require.Equal(t, 11, int(*snapshot.AccountID))
	require.Equal(t, 7, int(*snapshot.APIKeyID))
	require.Equal(t, 3, int(*snapshot.GroupID))
	require.True(t, snapshot.SnapshotReplayable)
	require.NotZero(t, snapshot.InputItemCount)
	require.True(t, snapshot.ExpiresAt.After(snapshot.CreatedAt))
}

func TestBuildOpenAIRequestSnapshotRejectsInvalidJSON(t *testing.T) {
	_, ok := buildOpenAIRequestSnapshotFromGin(nil, nil, []byte(`not-json`), "", "", OpenAIContextMigrationObservation{}, 24)
	require.False(t, ok)
}
