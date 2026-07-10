package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserSubscriptionFromServiceAdmin_IncludesRevokedAt(t *testing.T) {
	t.Parallel()

	revokedAt := time.Date(2026, time.July, 10, 8, 30, 0, 0, time.UTC)
	subscription := &service.UserSubscription{
		ID:        42,
		DeletedAt: &revokedAt,
	}

	result := UserSubscriptionFromServiceAdmin(subscription)
	require.NotNil(t, result.RevokedAt)
	require.Equal(t, revokedAt, *result.RevokedAt)

	body, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(body), `"revoked_at":"2026-07-10T08:30:00Z"`)
}
