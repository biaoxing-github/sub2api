//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestGrokMultiAgentCapacityBlocksOnlyThatModel(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 9911, Platform: PlatformGrok, Type: AccountTypeOAuth}
	svc.handleGrokAccountUpstreamError(
		withGrokRequestedModel(context.Background(), "grok-4.20-multi-agent-0309"),
		account,
		http.StatusBadGateway,
		nil,
		[]byte(`{"error":{"message":"engine_overloaded"}}`),
	)
	require.True(t, isGrokModelCapacityBlocked(account.ID, "grok-4.20-multi-agent-0309", time.Now()))
	require.False(t, isGrokModelCapacityBlocked(account.ID, "grok-4.5", time.Now()))
	require.True(t, svc.isOpenAIAccountModelRuntimeBlocked(account, "grok-4.20-multi-agent-0309"))
	require.False(t, svc.isOpenAIAccountModelRuntimeBlocked(account, "grok-4.5"))
}

func TestGrok45HeavyQuotaSignalStampedWithModel(t *testing.T) {
	requestLimit := int64(8300)
	tokenLimit := int64(53000000)
	snapshot := &xai.QuotaSnapshot{
		Requests:          &xai.QuotaWindow{Limit: &requestLimit},
		Tokens:            &xai.QuotaWindow{Limit: &tokenLimit},
		LastHeadersSeenAt: "2026-08-14T03:00:00Z",
		UpdatedAt:         "2026-08-14T03:00:00Z",
	}

	(&OpenAIGatewayService{}).updateGrokUsageSnapshotForAccount(context.Background(), nil, snapshot, "grok-4.5")

	require.Equal(t, "grok-4.5", snapshot.Model)
	require.Equal(t, "supergrok_heavy", snapshot.PlanFrom45Responses)
	require.Equal(t, snapshot.LastHeadersSeenAt, snapshot.PlanFrom45ResponsesAt)
}
