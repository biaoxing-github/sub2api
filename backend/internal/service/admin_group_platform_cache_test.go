//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// groupPlatformRepoStub 只实现 UpdateGroup 需要的查询和更新方法。
type groupPlatformRepoStub struct {
	GroupRepository
	group   *Group
	updated *Group
}

func (r *groupPlatformRepoStub) GetByID(_ context.Context, _ int64) (*Group, error) {
	cloned := *r.group
	return &cloned, nil
}

func (r *groupPlatformRepoStub) Update(_ context.Context, group *Group) error {
	r.updated = group
	return nil
}

type channelCacheInvalidatorSpy struct {
	calls int
}

func (s *channelCacheInvalidatorSpy) InvalidateCache() { s.calls++ }

func TestUpdateGroupInvalidatesChannelCacheOnPlatformChange(t *testing.T) {
	tests := []struct {
		name          string
		fromPlatform  string
		inputPlatform string
		wantCalls     int
	}{
		{name: "platform changed", fromPlatform: PlatformAnthropic, inputPlatform: PlatformOpenAI, wantCalls: 1},
		{name: "platform unchanged", fromPlatform: PlatformAnthropic, inputPlatform: PlatformAnthropic, wantCalls: 0},
		{name: "platform omitted", fromPlatform: PlatformAnthropic, inputPlatform: "", wantCalls: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &groupPlatformRepoStub{group: &Group{ID: 7, Name: "g", Platform: tt.fromPlatform}}
			spy := &channelCacheInvalidatorSpy{}
			svc := &adminServiceImpl{groupRepo: repo, channelCacheInvalidator: spy}

			got, err := svc.UpdateGroup(context.Background(), 7, &UpdateGroupInput{Platform: tt.inputPlatform})
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tt.wantCalls, spy.calls)
		})
	}
}

func TestUpdateGroupWithoutChannelCacheInvalidator(t *testing.T) {
	repo := &groupPlatformRepoStub{group: &Group{ID: 7, Name: "g", Platform: PlatformAnthropic}}
	svc := &adminServiceImpl{groupRepo: repo}

	got, err := svc.UpdateGroup(context.Background(), 7, &UpdateGroupInput{Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, got.Platform)
}
