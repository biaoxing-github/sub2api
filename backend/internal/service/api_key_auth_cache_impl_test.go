//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotRoundTripExclusiveGroupAuthFields(t *testing.T) {
	groupID := int64(42)
	apiKey := &APIKey{
		ID:      100,
		UserID:  7,
		GroupID: &groupID,
		Name:    "exclusive-key",
		Status:  StatusActive,
		User: &User{
			ID:            7,
			Status:        StatusActive,
			Role:          RoleUser,
			AllowedGroups: []int64{groupID},
		},
		Group: &Group{
			ID:               groupID,
			Name:             "exclusive",
			Status:           StatusActive,
			Hydrated:         true,
			IsExclusive:      true,
			SubscriptionType: SubscriptionTypeStandard,
		},
	}
	svc := &APIKeyService{}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.Equal(t, apiKeyAuthSnapshotVersion, snapshot.Version)
	require.Equal(t, []int64{groupID}, snapshot.User.AllowedGroups)
	require.NotNil(t, snapshot.Group)
	require.True(t, snapshot.Group.IsExclusive)

	got := svc.snapshotToAPIKey("sk-test", snapshot)
	require.NotNil(t, got)
	require.NotNil(t, got.User)
	require.Equal(t, []int64{groupID}, got.User.AllowedGroups)
	require.NotNil(t, got.Group)
	require.True(t, got.Group.IsExclusive)
	require.True(t, got.User.CanBindGroup(groupID, got.Group.IsExclusive))
}

func TestAPIKeyAuthCacheIgnoresOlderSnapshotVersion(t *testing.T) {
	svc := &APIKeyService{}
	snapshot := &APIKeyAuthSnapshot{
		Version:  apiKeyAuthSnapshotVersion - 1,
		APIKeyID: 100,
		UserID:   7,
		User: APIKeyAuthUserSnapshot{
			ID:     7,
			Status: StatusActive,
			Role:   RoleUser,
		},
	}

	got, cached, err := svc.applyAuthCacheEntry("sk-test", &APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	require.False(t, cached)
	require.Nil(t, got)
}
