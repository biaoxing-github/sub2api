//go:build integration

package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const contextJournalRedisImageTag = "redis:8.4-alpine"

func TestRedisContextJournalAppendListBindReplayAndTTL(t *testing.T) {
	ctx := context.Background()
	rdb := startContextJournalRedis(t, ctx)
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	journal := NewRedisContextJournal(rdb, ContextJournalOptions{
		TTL:             2 * time.Hour,
		MaxSessionBytes: 1024,
		Now:             func() time.Time { return now },
	})

	turn, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-redis",
		AccountID:   42,
		Protocol:    ContextJournalProtocolOpenAIResponses,
		RequestBody: []byte(`{"input":"hello"}`),
		ResponseID:  "resp_redis",
	})
	require.NoError(t, err)
	require.NotEmpty(t, turn.TurnID)
	require.Equal(t, ContextReplaySafe, turn.ReplaySafety)

	turns, err := journal.ListTurns(ctx, 7, "sess-redis")
	require.NoError(t, err)
	require.Len(t, turns, 1)
	require.Equal(t, turn.TurnID, turns[0].TurnID)
	require.Equal(t, []byte(`{"input":"hello"}`), turns[0].RequestBody)

	turns[0].RequestBody[0] = '['
	turnsAgain, err := journal.ListTurns(ctx, 7, "sess-redis")
	require.NoError(t, err)
	require.Equal(t, []byte(`{"input":"hello"}`), turnsAgain[0].RequestBody)

	require.NoError(t, journal.BindResponse(ctx, 7, "resp_redis", ContextJournalResponseRef{
		SessionHash: "sess-redis",
		AccountID:   42,
		TurnID:      turn.TurnID,
	}, 30*time.Minute))
	ref, err := journal.GetResponse(ctx, 7, "resp_redis")
	require.NoError(t, err)
	require.NotNil(t, ref)
	require.Equal(t, "sess-redis", ref.SessionHash)
	require.Equal(t, int64(42), ref.AccountID)
	require.Equal(t, turn.TurnID, ref.TurnID)

	replay, err := journal.BuildReplay(ctx, 7, "sess-redis")
	require.NoError(t, err)
	require.True(t, replay.Safe)
	require.Equal(t, ContextReplayReasonSafe, replay.Reason)
	require.Equal(t, ContextJournalBackendRedis, replay.Backend)
	require.Equal(t, 1, replay.TurnCount)
	require.Equal(t, int64(len(`{"input":"hello"}`)), replay.SessionBytes)
	require.Equal(t, int64(1024), replay.MaxSessionBytes)
	require.False(t, replay.Overflow)
	require.Equal(t, []byte(`{"input":"hello"}`), replay.RequestBody)

	stateTTL, err := rdb.TTL(ctx, redisContextJournalStateKey(7, "sess-redis")).Result()
	require.NoError(t, err)
	require.Greater(t, stateTTL, time.Duration(0))
	require.LessOrEqual(t, stateTTL, 2*time.Hour)
	turnsTTL, err := rdb.TTL(ctx, redisContextJournalTurnsKey(7, "sess-redis")).Result()
	require.NoError(t, err)
	require.Greater(t, turnsTTL, time.Duration(0))
	require.LessOrEqual(t, turnsTTL, 2*time.Hour)
	refTTL, err := rdb.TTL(ctx, redisContextJournalResponseKey(7, "resp_redis")).Result()
	require.NoError(t, err)
	require.Greater(t, refTTL, time.Duration(0))
	require.LessOrEqual(t, refTTL, 30*time.Minute)
}

func TestRedisContextJournalRejectsOverflowAndReportsDiagnostics(t *testing.T) {
	ctx := context.Background()
	rdb := startContextJournalRedis(t, ctx)
	journal := NewRedisContextJournal(rdb, ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 12,
	})

	_, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-overflow",
		AccountID:   42,
		RequestBody: []byte(`1234567890`),
	})
	require.NoError(t, err)

	_, err = journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-overflow",
		AccountID:   42,
		RequestBody: []byte(`abc`),
	})
	require.ErrorIs(t, err, ErrContextJournalSessionOverflow)

	result, err := journal.IsReplaySafe(ctx, 7, "sess-overflow")
	require.NoError(t, err)
	require.False(t, result.Safe)
	require.Equal(t, ContextReplayReasonJournalOverflow, result.Reason)
	require.Equal(t, ContextJournalBackendRedis, result.Backend)
	require.Equal(t, 1, result.TurnCount)
	require.Equal(t, int64(10), result.SessionBytes)
	require.Equal(t, int64(12), result.MaxSessionBytes)
	require.True(t, result.Overflow)
}

func TestRedisContextJournalReplaySafetyReasons(t *testing.T) {
	ctx := context.Background()
	rdb := startContextJournalRedis(t, ctx)
	journal := NewRedisContextJournal(rdb, ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
	})

	_, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:               7,
		SessionHash:           "sess-protected",
		AccountID:             42,
		RequestBody:           []byte(`{"input":[{"type":"function_call_output","output":"ok"}]}`),
		HasFunctionCallOutput: true,
	})
	require.NoError(t, err)

	result, err := journal.IsReplaySafe(ctx, 7, "sess-protected")
	require.NoError(t, err)
	require.False(t, result.Safe)
	require.Equal(t, ContextReplayReasonFunctionCallOutput, result.Reason)
	require.Equal(t, ContextJournalBackendRedis, result.Backend)
	require.Equal(t, 1, result.TurnCount)

	missing, err := journal.IsReplaySafe(ctx, 7, "missing-session")
	require.NoError(t, err)
	require.False(t, missing.Safe)
	require.Equal(t, ContextReplayReasonMissingBody, missing.Reason)
	require.Equal(t, ContextJournalBackendRedis, missing.Backend)
	require.Equal(t, 0, missing.TurnCount)
	require.Equal(t, int64(1024), missing.MaxSessionBytes)
}

func TestRedisContextJournalRestartExpiryCorruptionAndReplayBoundaries(t *testing.T) {
	ctx := context.Background()
	initialRedis := startContextJournalRedis(t, ctx)
	redisAddr := initialRedis.Options().Addr
	journalBeforeRestart := NewRedisContextJournal(initialRedis, ContextJournalOptions{
		TTL:              time.Hour,
		MaxSessionBytes:  1024,
		OperationTimeout: 250 * time.Millisecond,
	})

	turn, err := journalBeforeRestart.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-restart",
		AccountID:   42,
		Protocol:    ContextJournalProtocolOpenAIResponses,
		RequestBody: []byte(`{"input":"restart-safe"}`),
		ResponseID:  "resp_restart",
	})
	require.NoError(t, err)
	require.NoError(t, journalBeforeRestart.BindResponse(ctx, 7, "resp_restart", ContextJournalResponseRef{
		SessionHash: "sess-restart",
		AccountID:   42,
		TurnID:      turn.TurnID,
	}, time.Minute))

	// 使用全新的 Redis client 和 Journal 实例模拟应用进程重启后的恢复读取。
	require.NoError(t, initialRedis.Close())
	restartedRedis := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = restartedRedis.Close() })
	require.NoError(t, restartedRedis.Ping(ctx).Err())
	journalAfterRestart := NewRedisContextJournal(restartedRedis, ContextJournalOptions{
		TTL:              time.Hour,
		MaxSessionBytes:  1024,
		OperationTimeout: 250 * time.Millisecond,
	})

	ref, err := journalAfterRestart.GetResponse(ctx, 7, "resp_restart")
	require.NoError(t, err)
	require.NotNil(t, ref)
	require.Equal(t, "sess-restart", ref.SessionHash)
	replay, err := journalAfterRestart.BuildReplay(ctx, 7, "sess-restart")
	require.NoError(t, err)
	require.True(t, replay.Safe)
	require.Equal(t, []byte(`{"input":"restart-safe"}`), replay.RequestBody)

	t.Run("expired response binding", func(t *testing.T) {
		require.NoError(t, journalAfterRestart.BindResponse(ctx, 7, "resp_expiring", ContextJournalResponseRef{
			SessionHash: "sess-restart",
			AccountID:   42,
			TurnID:      turn.TurnID,
		}, 30*time.Millisecond))
		require.Eventually(t, func() bool {
			expired, getErr := journalAfterRestart.GetResponse(ctx, 7, "resp_expiring")
			return getErr == nil && expired == nil
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("corrupt JSON is rejected without exposing stored content", func(t *testing.T) {
		const secretMarker = "FULL_REQUEST_BODY_MUST_NOT_APPEAR"
		responseKey := redisContextJournalResponseKey(7, "resp_corrupt")
		require.NoError(t, restartedRedis.Set(ctx, responseKey, `{"broken":"`+secretMarker, time.Minute).Err())
		_, getErr := journalAfterRestart.GetResponse(ctx, 7, "resp_corrupt")
		require.Error(t, getErr)
		require.NotContains(t, getErr.Error(), secretMarker)

		stateKey := redisContextJournalStateKey(7, "sess-corrupt-state")
		require.NoError(t, restartedRedis.Set(ctx, stateKey, `{"broken":"`+secretMarker, time.Minute).Err())
		_, replayErr := journalAfterRestart.BuildReplay(ctx, 7, "sess-corrupt-state")
		require.Error(t, replayErr)
		require.NotContains(t, replayErr.Error(), secretMarker)

		_, appendErr := journalAfterRestart.AppendTurn(ctx, ContextJournalAppendInput{
			GroupID:     7,
			SessionHash: "sess-corrupt-turn",
			AccountID:   42,
			RequestBody: []byte(`{"input":"valid-before-corruption"}`),
		})
		require.NoError(t, appendErr)
		turnsKey := redisContextJournalTurnsKey(7, "sess-corrupt-turn")
		require.NoError(t, restartedRedis.Del(ctx, turnsKey).Err())
		require.NoError(t, restartedRedis.RPush(ctx, turnsKey, `{"broken":"`+secretMarker).Err())
		_, replayErr = journalAfterRestart.BuildReplay(ctx, 7, "sess-corrupt-turn")
		require.Error(t, replayErr)
		require.NotContains(t, replayErr.Error(), secretMarker)
	})

	protectedCases := []struct {
		name                string
		body                []byte
		clientOutputStarted bool
		wantReason          ContextReplayReason
	}{
		{
			name:       "function call output",
			body:       []byte(`{"input":[{"type":"function_call_output","output":"ok"}]}`),
			wantReason: ContextReplayReasonFunctionCallOutput,
		},
		{
			name:       "encrypted reasoning",
			body:       []byte(`{"input":[{"type":"reasoning","encrypted_content":"opaque"}]}`),
			wantReason: ContextReplayReasonEncryptedReasoning,
		},
		{
			name:                "client output started",
			body:                []byte(`{"input":"already-visible"}`),
			clientOutputStarted: true,
			wantReason:          ContextReplayReasonClientOutputStarted,
		},
	}
	for _, tc := range protectedCases {
		t.Run(tc.name, func(t *testing.T) {
			sessionHash := "sess-protected-" + strings.ReplaceAll(tc.name, " ", "-")
			_, appendErr := journalAfterRestart.AppendTurn(ctx, ContextJournalAppendInput{
				GroupID:             7,
				SessionHash:         sessionHash,
				AccountID:           42,
				Protocol:            ContextJournalProtocolOpenAIResponses,
				RequestBody:         tc.body,
				ClientOutputStarted: tc.clientOutputStarted,
			})
			require.NoError(t, appendErr)
			protectedReplay, replayErr := journalAfterRestart.BuildReplay(ctx, 7, sessionHash)
			require.NoError(t, replayErr)
			require.False(t, protectedReplay.Safe)
			require.Equal(t, tc.wantReason, protectedReplay.Reason)
			require.Empty(t, protectedReplay.RequestBody, "protected turn must not produce a replay body")
		})
	}
}

func TestProvideContextJournalUsesRedisWhenAvailable(t *testing.T) {
	ctx := context.Background()
	rdb := startContextJournalRedis(t, ctx)

	journal := ProvideContextJournal(rdb, testConfigWithRedisContextJournal())
	if _, ok := journal.(*redisContextJournal); !ok {
		t.Fatalf("journal = %T, want *redisContextJournal", journal)
	}
}

func startContextJournalRedis(t *testing.T, ctx context.Context) *redis.Client {
	t.Helper()
	if !contextJournalDockerAvailable(ctx) {
		t.Skip("Docker 未启用，跳过依赖 testcontainers 的 Redis context journal 测试")
	}

	redisContainer, err := tcredis.Run(ctx, contextJournalRedisImageTag)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = redisContainer.Terminate(ctx)
	})

	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort.Int()),
		DB:   0,
	})
	require.NoError(t, rdb.Ping(ctx).Err())
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	return rdb
}

func contextJournalDockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}
