package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// defaultContextJournalRedisOperationTimeout 是单次 Redis Journal 公开操作的默认总预算。
const defaultContextJournalRedisOperationTimeout = 100 * time.Millisecond

type redisContextJournal struct {
	rdb              *redis.Client
	ttl              time.Duration
	maxBytes         int64
	operationTimeout time.Duration // 单次公开操作的总 deadline 与 Redis 读写预算
	now              func() time.Time
}

func NewRedisContextJournal(rdb *redis.Client, options ContextJournalOptions) ContextJournal {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = defaultContextJournalTTL
	}
	maxBytes := options.MaxSessionBytes
	if maxBytes <= 0 {
		maxBytes = defaultContextJournalMaxSessionBytes
	}
	operationTimeout := options.OperationTimeout
	if operationTimeout <= 0 {
		operationTimeout = defaultContextJournalRedisOperationTimeout
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	journalRedis := rdb
	if rdb != nil {
		// journal 与其他缓存共享连接池，但使用独立的短读写预算和调用方 deadline。
		journalRedis = rdb.WithTimeout(operationTimeout)
		journalRedis.Options().ContextTimeoutEnabled = true
	}
	return &redisContextJournal{
		rdb:              journalRedis,
		ttl:              ttl,
		maxBytes:         maxBytes,
		operationTimeout: operationTimeout,
		now:              now,
	}
}

func (j *redisContextJournal) AppendTurn(ctx context.Context, input ContextJournalAppendInput) (*ContextJournalTurn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if j == nil || j.rdb == nil {
		return nil, errors.New("redis context journal is not configured")
	}
	operationCtx, cancel := j.withOperationTimeout(ctx)
	defer cancel()
	sessionHash := strings.TrimSpace(input.SessionHash)
	if sessionHash == "" {
		return nil, errors.New("context journal session hash is required")
	}
	now := j.now().UTC()
	expiresAt := now.Add(j.ttl)
	body := cloneBytes(input.RequestBody)
	bodySize := int64(len(body))
	stateKey := redisContextJournalStateKey(input.GroupID, sessionHash)
	turnsKey := redisContextJournalTurnsKey(input.GroupID, sessionHash)

	state, ok, err := j.getSessionState(operationCtx, input.GroupID, sessionHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		state = ContextJournalSessionState{GroupID: input.GroupID, SessionHash: sessionHash}
	}
	if state.Overflow || state.TotalBytes+bodySize > j.maxBytes {
		state.Overflow = true
		state.UpdatedAt = now
		state.ExpiresAt = expiresAt
		if err := j.setSessionState(operationCtx, stateKey, state); err != nil {
			return nil, err
		}
		_ = j.rdb.Expire(operationCtx, turnsKey, j.ttl).Err()
		return nil, ErrContextJournalSessionOverflow
	}
	protocol := strings.TrimSpace(input.RequestProtocol)
	if protocol == "" {
		protocol = strings.TrimSpace(input.Protocol)
	}
	turn := ContextJournalTurn{
		TurnID:                newContextJournalTurnID(now),
		GroupID:               input.GroupID,
		SessionHash:           sessionHash,
		AccountID:             input.AccountID,
		Protocol:              protocol,
		RequestProtocol:       protocol,
		RequestBody:           body,
		RequestBodyHash:       sha256Hex(body),
		ResponseID:            strings.TrimSpace(input.ResponseID),
		HasFunctionCallOutput: input.HasFunctionCallOutput || bodyHasFunctionCallOutput(body),
		HasEncryptedReasoning: input.HasEncryptedReasoning || bodyHasEncryptedReasoning(body),
		HasPreviousResponseID: input.HasPreviousResponseID || bodyHasPreviousResponseID(body),
		ClientOutputStarted:   input.ClientOutputStarted,
		ReplaySafety:          ClassifyContextReplaySafety(body),
		CreatedAt:             now,
		ExpiresAt:             expiresAt,
	}
	if turn.HasFunctionCallOutput || turn.HasEncryptedReasoning || turn.ClientOutputStarted {
		turn.ReplaySafety = ContextReplayProtected
	}
	raw, err := json.Marshal(turn)
	if err != nil {
		return nil, err
	}
	state.TotalBytes += bodySize
	state.UpdatedAt = now
	state.ExpiresAt = expiresAt
	pipe := j.rdb.TxPipeline()
	pipe.Set(operationCtx, stateKey, mustJSON(state), j.ttl)
	pipe.RPush(operationCtx, turnsKey, raw)
	pipe.Expire(operationCtx, turnsKey, j.ttl)
	if _, err := pipe.Exec(operationCtx); err != nil {
		return nil, redisContextJournalError(operationCtx, err)
	}
	out := cloneContextJournalTurn(turn)
	return &out, nil
}

func (j *redisContextJournal) ListTurns(ctx context.Context, groupID int64, sessionHash string) ([]ContextJournalTurn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if j == nil || j.rdb == nil {
		return nil, errors.New("redis context journal is not configured")
	}
	operationCtx, cancel := j.withOperationTimeout(ctx)
	defer cancel()
	turnsKey := redisContextJournalTurnsKey(groupID, sessionHash)
	if turnsKey == "" {
		return nil, nil
	}
	rawItems, err := j.rdb.LRange(operationCtx, turnsKey, 0, -1).Result()
	if err != nil {
		return nil, redisContextJournalError(operationCtx, err)
	}
	turns := make([]ContextJournalTurn, 0, len(rawItems))
	for _, raw := range rawItems {
		var turn ContextJournalTurn
		if err := json.Unmarshal([]byte(raw), &turn); err != nil {
			return nil, err
		}
		turns = append(turns, cloneContextJournalTurn(turn))
	}
	return turns, nil
}

func (j *redisContextJournal) BindResponse(ctx context.Context, groupID int64, responseID string, ref ContextJournalResponseRef, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if j == nil || j.rdb == nil {
		return errors.New("redis context journal is not configured")
	}
	operationCtx, cancel := j.withOperationTimeout(ctx)
	defer cancel()
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = j.ttl
	}
	now := j.now().UTC()
	ref.GroupID = groupID
	ref.SessionHash = strings.TrimSpace(ref.SessionHash)
	ref.TurnID = strings.TrimSpace(ref.TurnID)
	ref.CreatedAt = now
	ref.ExpiresAt = now.Add(ttl)
	return redisContextJournalError(operationCtx, j.rdb.Set(operationCtx, redisContextJournalResponseKey(groupID, responseID), mustJSON(ref), ttl).Err())
}

func (j *redisContextJournal) GetResponse(ctx context.Context, groupID int64, responseID string) (*ContextJournalResponseRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if j == nil || j.rdb == nil {
		return nil, errors.New("redis context journal is not configured")
	}
	operationCtx, cancel := j.withOperationTimeout(ctx)
	defer cancel()
	key := redisContextJournalResponseKey(groupID, responseID)
	if key == "" {
		return nil, nil
	}
	raw, err := j.rdb.Get(operationCtx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, redisContextJournalError(operationCtx, err)
	}
	var ref ContextJournalResponseRef
	if err := json.Unmarshal([]byte(raw), &ref); err != nil {
		return nil, err
	}
	return &ref, nil
}

func (j *redisContextJournal) GetResponseBinding(ctx context.Context, groupID int64, responseID string) (*ContextJournalResponseRef, error) {
	return j.GetResponse(ctx, groupID, responseID)
}

func (j *redisContextJournal) BuildReplay(ctx context.Context, groupID int64, sessionHash string) (*ContextJournalReplay, error) {
	operationCtx, cancel := j.withOperationTimeout(ctx)
	defer cancel()
	safety, turns, err := j.replaySafetyAndTurns(operationCtx, groupID, sessionHash)
	if err != nil {
		return nil, err
	}
	replay := &ContextJournalReplay{
		ContextJournalReplaySafetyResult: safety,
		Turns:                            turns,
	}
	if safety.Safe && len(turns) > 0 {
		replay.RequestBody = cloneBytes(turns[len(turns)-1].RequestBody)
	}
	return replay, nil
}

func (j *redisContextJournal) IsReplaySafe(ctx context.Context, groupID int64, sessionHash string) (ContextJournalReplaySafetyResult, error) {
	operationCtx, cancel := j.withOperationTimeout(ctx)
	defer cancel()
	safety, _, err := j.replaySafetyAndTurns(operationCtx, groupID, sessionHash)
	return safety, err
}

func (j *redisContextJournal) replaySafetyAndTurns(ctx context.Context, groupID int64, sessionHash string) (ContextJournalReplaySafetyResult, []ContextJournalTurn, error) {
	if err := ctx.Err(); err != nil {
		return ContextJournalReplaySafetyResult{}, nil, err
	}
	state, ok, err := j.getSessionState(ctx, groupID, sessionHash)
	if err != nil {
		return ContextJournalReplaySafetyResult{}, nil, err
	}
	if !ok {
		return withContextJournalDiagnostics(
			protectedReplayResult(ContextReplayReasonMissingBody),
			ContextJournalBackendRedis,
			nil,
			nil,
			j.maxBytes,
		), nil, nil
	}
	turns, err := j.ListTurns(ctx, groupID, sessionHash)
	if err != nil {
		return ContextJournalReplaySafetyResult{}, nil, err
	}
	if state.Overflow {
		return withContextJournalDiagnostics(
			protectedReplayResult(ContextReplayReasonJournalOverflow),
			ContextJournalBackendRedis,
			&state,
			turns,
			j.maxBytes,
		), turns, nil
	}
	return withContextJournalDiagnostics(
		classifyReplayTurns(turns),
		ContextJournalBackendRedis,
		&state,
		turns,
		j.maxBytes,
	), turns, nil
}

func (j *redisContextJournal) getSessionState(ctx context.Context, groupID int64, sessionHash string) (ContextJournalSessionState, bool, error) {
	key := redisContextJournalStateKey(groupID, sessionHash)
	if key == "" {
		return ContextJournalSessionState{}, false, nil
	}
	raw, err := j.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return ContextJournalSessionState{}, false, nil
	}
	if err != nil {
		return ContextJournalSessionState{}, false, redisContextJournalError(ctx, err)
	}
	var state ContextJournalSessionState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return ContextJournalSessionState{}, false, err
	}
	return state, true, nil
}

func (j *redisContextJournal) setSessionState(ctx context.Context, key string, state ContextJournalSessionState) error {
	if key == "" {
		return nil
	}
	return redisContextJournalError(ctx, j.rdb.Set(ctx, key, mustJSON(state), j.ttl).Err())
}

// withOperationTimeout 将每个公开 Journal 操作限制在同一个总预算内。
// 嵌套调用沿用更早的父 deadline，不会为每个 Redis 命令重新获得完整预算。
func (j *redisContextJournal) withOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := defaultContextJournalRedisOperationTimeout
	if j != nil && j.operationTimeout > 0 {
		timeout = j.operationTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

// redisContextJournalError 优先返回稳定的 context 错误，便于调用方区分预算耗尽和 Redis 断连。
func redisContextJournalError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func redisContextJournalStateKey(groupID int64, sessionHash string) string {
	sessionHash = strings.TrimSpace(sessionHash)
	if sessionHash == "" {
		return ""
	}
	return fmt.Sprintf("ctx:j:%d:%s", groupID, sessionHash)
}

func redisContextJournalTurnsKey(groupID int64, sessionHash string) string {
	sessionHash = strings.TrimSpace(sessionHash)
	if sessionHash == "" {
		return ""
	}
	return fmt.Sprintf("ctx:j:%d:%s:turns", groupID, sessionHash)
}

func redisContextJournalResponseKey(groupID int64, responseID string) string {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return ""
	}
	return fmt.Sprintf("ctx:resp:%d:%s", groupID, responseID)
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return raw
}
