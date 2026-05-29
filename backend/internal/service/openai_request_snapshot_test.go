package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func TestBuildOpenAIRequestSnapshotFromGinOmitsLargeRequestBody(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"` + strings.Repeat("x", 3*1024*1024) + `"}]}]}`)
	obs := ClassifyOpenAIContextMigration(body, false)

	snapshot, ok := buildOpenAIRequestSnapshotFromGin(nil, &Account{ID: 11}, body, "gpt-5.5", "", obs, 24)

	require.True(t, ok)
	require.Equal(t, len(body), snapshot.RequestBodyBytes)
	require.NotEmpty(t, snapshot.RequestBodySHA256)
	require.Less(t, len(snapshot.RequestBody), 4096)
	require.False(t, snapshot.SnapshotReplayable)
	require.Equal(t, "request_body_omitted_for_snapshot_size", snapshot.ReplayBlockReason)
	require.NotContains(t, string(snapshot.RequestBody), strings.Repeat("x", 1024))

	var summary map[string]any
	require.NoError(t, json.Unmarshal(snapshot.RequestBody, &summary))
	require.Equal(t, true, summary["omitted"])
	require.Equal(t, "request_body_too_large_for_snapshot", summary["reason"])
	require.Equal(t, float64(len(body)), summary["request_body_bytes"])
	require.Equal(t, snapshot.RequestBodySHA256, summary["request_body_sha256"])
}

func TestBuildOpenAIRequestSnapshotRejectsInvalidJSON(t *testing.T) {
	_, ok := buildOpenAIRequestSnapshotFromGin(nil, nil, []byte(`not-json`), "", "", OpenAIContextMigrationObservation{}, 24)
	require.False(t, ok)
}

func TestOpenAIRequestSnapshotServiceRunsCleanupOnStart(t *testing.T) {
	repo := &requestSnapshotStartupCleanupRepo{cleanupCalls: make(chan struct{}, 1)}
	svc := NewOpenAIRequestSnapshotService(repo, nil)
	require.NotNil(t, svc)
	defer svc.Close()

	select {
	case <-repo.cleanupCalls:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected expired snapshot cleanup to run when service starts")
	}
}

type requestSnapshotStartupCleanupRepo struct {
	cleanupCalls chan struct{}
}

func (r *requestSnapshotStartupCleanupRepo) SaveOpenAIRequestSnapshot(context.Context, *OpenAIRequestSnapshot) error {
	return nil
}

func (r *requestSnapshotStartupCleanupRepo) DeleteExpiredOpenAIRequestSnapshots(context.Context, time.Time, int) (int64, error) {
	select {
	case r.cleanupCalls <- struct{}{}:
	default:
	}
	return 0, nil
}
