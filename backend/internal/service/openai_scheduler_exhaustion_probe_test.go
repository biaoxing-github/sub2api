package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type schedulerExhaustionProbeRepo struct {
	stubOpenAIAccountRepo
	listByGroupCalls    int
	listByPlatformCalls int
}

func (r *schedulerExhaustionProbeRepo) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	r.listByGroupCalls++
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == PlatformOpenAI {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r *schedulerExhaustionProbeRepo) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	r.listByPlatformCalls++
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

func TestOpenAISchedulerExhaustionProbeFiniteTriesEachCandidateSixTimes(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
			{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := map[int64]int{}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool) error {
			attempts[account.ID]++
			return fmt.Errorf("probe %d failed", account.ID)
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
	})

	require.False(t, recovered)
	require.Error(t, err)
	require.Equal(t, 1, repo.listByGroupCalls)
	require.Equal(t, 6, attempts[11])
	require.Equal(t, 6, attempts[12])
}

func TestOpenAISchedulerExhaustionProbeStopsOnSuccessAndClearsRuntimeBlock(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 21, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
			{ID: 22, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := map[int64]int{}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool) error {
			attempts[account.ID]++
			if account.ID == 22 && attempts[account.ID] == 2 {
				return nil
			}
			return errors.New("still unavailable")
		},
	}
	svc.BlockAccountScheduling(&repo.accounts[1], time.Now().Add(time.Minute), "test_block")

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
	})

	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, 6, attempts[21])
	require.Equal(t, 2, attempts[22])
	_, blocked := svc.SnapshotOpenAIAccountRuntimeBlock(&repo.accounts[1], time.Now())
	require.False(t, blocked)
}

func TestOpenAISchedulerExhaustionProbeInfiniteIgnoresFiniteAttemptCapUntilSuccess(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 31, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := 0
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool) error {
			attempts++
			if attempts == 8 {
				return nil
			}
			return errors.New("not yet")
		},
		openAISchedulerExhaustionProbeSleep: func(ctx context.Context, d time.Duration) error {
			return ctx.Err()
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
		Infinite:       true,
	})

	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, 8, attempts)
}
