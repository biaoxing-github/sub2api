//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAdminService_UpdateGroup_LimitFieldsPartialUpdate 覆盖省略、清除和显式零限额。
func TestAdminService_UpdateGroup_LimitFieldsPartialUpdate(t *testing.T) {
	daily, weekly, monthly := 10.0, 20.0, 30.0
	group := &Group{ID: 1, Name: "existing", Platform: PlatformOpenAI, Status: StatusActive,
		DailyLimitUSD: &daily, WeeklyLimitUSD: &weekly, MonthlyLimitUSD: &monthly}
	repo := &groupRepoStubForAdmin{getByID: group}
	svc := &adminServiceImpl{groupRepo: repo}
	description := "updated"
	updated, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{Description: &description})
	require.NoError(t, err)
	require.Equal(t, 10.0, *updated.DailyLimitUSD)
	require.Equal(t, 20.0, *updated.WeeklyLimitUSD)
	require.Equal(t, 30.0, *updated.MonthlyLimitUSD)
	zero, unlimited := 0.0, -1.0
	updated, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{DailyLimitUSD: &zero, WeeklyLimitUSD: &unlimited})
	require.NoError(t, err)
	require.Equal(t, 0.0, *updated.DailyLimitUSD)
	require.Nil(t, updated.WeeklyLimitUSD)
	require.Equal(t, 30.0, *updated.MonthlyLimitUSD)
}
