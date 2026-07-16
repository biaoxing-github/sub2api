package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

const (
	ContextJournalProtocolOpenAIResponses = "openai_responses"
	ContextJournalProtocolOpenAIMessages  = "openai_messages"

	ContextJournalBackendMemory = "memory"
	ContextJournalBackendRedis  = "redis"

	defaultContextJournalTTL             = 24 * time.Hour
	defaultContextJournalMaxSessionBytes = 50 * 1024 * 1024
)

var ErrContextJournalSessionOverflow = errors.New("context journal session overflow")

type ContextReplaySafety string

const (
	ContextReplaySafe      ContextReplaySafety = "safe"
	ContextReplayProtected ContextReplaySafety = "protected"
)

type ContextJournal interface {
	AppendTurn(ctx context.Context, input ContextJournalAppendInput) (*ContextJournalTurn, error)
	ListTurns(ctx context.Context, groupID int64, sessionHash string) ([]ContextJournalTurn, error)
	BindResponse(ctx context.Context, groupID int64, responseID string, ref ContextJournalResponseRef, ttl time.Duration) error
	GetResponse(ctx context.Context, groupID int64, responseID string) (*ContextJournalResponseRef, error)
	GetResponseBinding(ctx context.Context, groupID int64, responseID string) (*ContextJournalResponseRef, error)
	BuildReplay(ctx context.Context, groupID int64, sessionHash string) (*ContextJournalReplay, error)
	IsReplaySafe(ctx context.Context, groupID int64, sessionHash string) (ContextJournalReplaySafetyResult, error)
}

type ContextJournalOptions struct {
	TTL             time.Duration
	MaxSessionBytes int64
	// OperationTimeout 限制 Redis Journal 单次公开操作的总延迟预算。
	// 内存实现不使用该字段；零值由 Redis 实现收敛到短默认值。
	OperationTimeout time.Duration
	Now              func() time.Time
}

type ContextJournalAppendInput struct {
	GroupID               int64
	SessionHash           string
	AccountID             int64
	Protocol              string
	RequestProtocol       string
	RequestBody           []byte
	ResponseID            string
	HasFunctionCallOutput bool
	HasEncryptedReasoning bool
	HasPreviousResponseID bool
	ClientOutputStarted   bool
}

type ContextJournalTurn struct {
	TurnID                string
	GroupID               int64
	SessionHash           string
	AccountID             int64
	Protocol              string
	RequestProtocol       string
	RequestBody           []byte
	RequestBodyHash       string
	ResponseID            string
	HasFunctionCallOutput bool
	HasEncryptedReasoning bool
	HasPreviousResponseID bool
	ClientOutputStarted   bool
	ReplaySafety          ContextReplaySafety
	CreatedAt             time.Time
	ExpiresAt             time.Time
}

type ContextJournalResponseRef struct {
	GroupID     int64
	SessionHash string
	AccountID   int64
	TurnID      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type ContextJournalSessionState struct {
	GroupID     int64
	SessionHash string
	TotalBytes  int64
	Overflow    bool
	UpdatedAt   time.Time
	ExpiresAt   time.Time
}

type ContextReplayReason string

const (
	ContextReplayReasonSafe                ContextReplayReason = "safe"
	ContextReplayReasonMissingBody         ContextReplayReason = "missing_body"
	ContextReplayReasonFunctionCallOutput  ContextReplayReason = "function_call_output_requires_previous_response"
	ContextReplayReasonEncryptedReasoning  ContextReplayReason = "encrypted_reasoning_requires_original_account"
	ContextReplayReasonClientOutputStarted ContextReplayReason = "client_output_started"
	ContextReplayReasonJournalOverflow     ContextReplayReason = "journal_overflow"
)

type ContextJournalReplaySafetyResult struct {
	Safe            bool
	Reason          ContextReplayReason
	Backend         string
	TurnCount       int
	SessionBytes    int64
	MaxSessionBytes int64
	Overflow        bool
}

type ContextJournalReplay struct {
	ContextJournalReplaySafetyResult
	RequestBody []byte
	Turns       []ContextJournalTurn
}

type ContextJournalRecordInput struct {
	GroupID     *int64
	SessionHash string
	Account     *Account
	Protocol    string
	RequestBody []byte
	Result      *OpenAIForwardResult
}

type memoryContextJournal struct {
	mu          sync.Mutex
	ttl         time.Duration
	maxBytes    int64
	now         func() time.Time
	sessions    map[string]*memoryContextJournalSession
	responseRef map[string]ContextJournalResponseRef
}

type memoryContextJournalSession struct {
	state ContextJournalSessionState
	turns []ContextJournalTurn
}

func NewMemoryContextJournal(options ContextJournalOptions) *memoryContextJournal {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = defaultContextJournalTTL
	}
	maxBytes := options.MaxSessionBytes
	if maxBytes <= 0 {
		maxBytes = defaultContextJournalMaxSessionBytes
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &memoryContextJournal{
		ttl:         ttl,
		maxBytes:    maxBytes,
		now:         now,
		sessions:    make(map[string]*memoryContextJournalSession),
		responseRef: make(map[string]ContextJournalResponseRef),
	}
}

func (j *memoryContextJournal) AppendTurn(ctx context.Context, input ContextJournalAppendInput) (*ContextJournalTurn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if j == nil {
		return nil, errors.New("context journal is not configured")
	}
	sessionHash := strings.TrimSpace(input.SessionHash)
	if sessionHash == "" {
		return nil, errors.New("context journal session hash is required")
	}
	now := j.now().UTC()
	expiresAt := now.Add(j.ttl)
	body := cloneBytes(input.RequestBody)
	bodySize := int64(len(body))
	key := contextJournalSessionKey(input.GroupID, sessionHash)

	j.mu.Lock()
	defer j.mu.Unlock()
	j.cleanupLocked(now)

	session := j.sessions[key]
	if session == nil {
		session = &memoryContextJournalSession{
			state: ContextJournalSessionState{
				GroupID:     input.GroupID,
				SessionHash: sessionHash,
			},
		}
		j.sessions[key] = session
	}
	if session.state.Overflow || session.state.TotalBytes+bodySize > j.maxBytes {
		session.state.Overflow = true
		session.state.UpdatedAt = now
		session.state.ExpiresAt = expiresAt
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

	session.turns = append(session.turns, turn)
	session.state.TotalBytes += bodySize
	session.state.UpdatedAt = now
	session.state.ExpiresAt = expiresAt

	out := cloneContextJournalTurn(turn)
	return &out, nil
}

func (j *memoryContextJournal) ListTurns(ctx context.Context, groupID int64, sessionHash string) ([]ContextJournalTurn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if j == nil {
		return nil, errors.New("context journal is not configured")
	}
	key := contextJournalSessionKey(groupID, sessionHash)
	if key == "" {
		return nil, nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	now := j.now().UTC()
	j.cleanupLocked(now)
	session := j.sessions[key]
	if session == nil {
		return nil, nil
	}
	return cloneContextJournalTurns(session.turns), nil
}

func (j *memoryContextJournal) BindResponse(ctx context.Context, groupID int64, responseID string, ref ContextJournalResponseRef, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if j == nil {
		return errors.New("context journal is not configured")
	}
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

	j.mu.Lock()
	defer j.mu.Unlock()
	j.cleanupLocked(now)
	j.responseRef[contextJournalResponseKey(groupID, responseID)] = ref
	return nil
}

func (j *memoryContextJournal) GetResponse(ctx context.Context, groupID int64, responseID string) (*ContextJournalResponseRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if j == nil {
		return nil, errors.New("context journal is not configured")
	}
	key := contextJournalResponseKey(groupID, responseID)
	if key == "" {
		return nil, nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	now := j.now().UTC()
	j.cleanupLocked(now)
	ref, ok := j.responseRef[key]
	if !ok {
		return nil, nil
	}
	out := ref
	return &out, nil
}

func (j *memoryContextJournal) GetResponseBinding(ctx context.Context, groupID int64, responseID string) (*ContextJournalResponseRef, error) {
	return j.GetResponse(ctx, groupID, responseID)
}

func (j *memoryContextJournal) BuildReplay(ctx context.Context, groupID int64, sessionHash string) (*ContextJournalReplay, error) {
	safety, turns, err := j.replaySafetyAndTurns(ctx, groupID, sessionHash)
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

func (j *memoryContextJournal) IsReplaySafe(ctx context.Context, groupID int64, sessionHash string) (ContextJournalReplaySafetyResult, error) {
	safety, _, err := j.replaySafetyAndTurns(ctx, groupID, sessionHash)
	return safety, err
}

func (j *memoryContextJournal) SessionState(ctx context.Context, groupID int64, sessionHash string) (ContextJournalSessionState, bool) {
	if ctx != nil && ctx.Err() != nil {
		return ContextJournalSessionState{}, false
	}
	if j == nil {
		return ContextJournalSessionState{}, false
	}
	key := contextJournalSessionKey(groupID, sessionHash)
	if key == "" {
		return ContextJournalSessionState{}, false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	now := j.now().UTC()
	j.cleanupLocked(now)
	session := j.sessions[key]
	if session == nil {
		return ContextJournalSessionState{}, false
	}
	return session.state, true
}

func (j *memoryContextJournal) replaySafetyAndTurns(ctx context.Context, groupID int64, sessionHash string) (ContextJournalReplaySafetyResult, []ContextJournalTurn, error) {
	if err := ctx.Err(); err != nil {
		return ContextJournalReplaySafetyResult{}, nil, err
	}
	if j == nil {
		return ContextJournalReplaySafetyResult{}, nil, errors.New("context journal is not configured")
	}
	key := contextJournalSessionKey(groupID, sessionHash)
	if key == "" {
		return protectedReplayResult(ContextReplayReasonMissingBody), nil, nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	now := j.now().UTC()
	j.cleanupLocked(now)
	session := j.sessions[key]
	if session == nil {
		return withContextJournalDiagnostics(
			protectedReplayResult(ContextReplayReasonMissingBody),
			ContextJournalBackendMemory,
			nil,
			nil,
			j.maxBytes,
		), nil, nil
	}
	turns := cloneContextJournalTurns(session.turns)
	if session.state.Overflow {
		return withContextJournalDiagnostics(
			protectedReplayResult(ContextReplayReasonJournalOverflow),
			ContextJournalBackendMemory,
			&session.state,
			turns,
			j.maxBytes,
		), turns, nil
	}
	return withContextJournalDiagnostics(
		classifyReplayTurns(turns),
		ContextJournalBackendMemory,
		&session.state,
		turns,
		j.maxBytes,
	), turns, nil
}

func classifyReplayTurns(turns []ContextJournalTurn) ContextJournalReplaySafetyResult {
	if len(turns) == 0 {
		return protectedReplayResult(ContextReplayReasonMissingBody)
	}
	for _, turn := range turns {
		switch {
		case len(turn.RequestBody) == 0:
			return protectedReplayResult(ContextReplayReasonMissingBody)
		case turn.HasFunctionCallOutput:
			return protectedReplayResult(ContextReplayReasonFunctionCallOutput)
		case turn.HasEncryptedReasoning:
			return protectedReplayResult(ContextReplayReasonEncryptedReasoning)
		case turn.ClientOutputStarted:
			return protectedReplayResult(ContextReplayReasonClientOutputStarted)
		case ClassifyContextReplaySafety(turn.RequestBody) != ContextReplaySafe:
			return protectedReplayResult(ContextReplayReasonMissingBody)
		}
	}
	return ContextJournalReplaySafetyResult{Safe: true, Reason: ContextReplayReasonSafe}
}

func protectedReplayResult(reason ContextReplayReason) ContextJournalReplaySafetyResult {
	return ContextJournalReplaySafetyResult{Safe: false, Reason: reason}
}

func withContextJournalDiagnostics(
	result ContextJournalReplaySafetyResult,
	backend string,
	state *ContextJournalSessionState,
	turns []ContextJournalTurn,
	maxBytes int64,
) ContextJournalReplaySafetyResult {
	result.Backend = backend
	result.TurnCount = len(turns)
	result.MaxSessionBytes = maxBytes
	if state != nil {
		result.SessionBytes = state.TotalBytes
		result.Overflow = state.Overflow
		return result
	}
	var total int64
	for _, turn := range turns {
		total += int64(len(turn.RequestBody))
	}
	result.SessionBytes = total
	return result
}

func (j *memoryContextJournal) cleanupLocked(now time.Time) {
	for key, session := range j.sessions {
		if session == nil || !session.state.ExpiresAt.IsZero() && !now.Before(session.state.ExpiresAt) {
			delete(j.sessions, key)
		}
	}
	for key, ref := range j.responseRef {
		if !ref.ExpiresAt.IsZero() && !now.Before(ref.ExpiresAt) {
			delete(j.responseRef, key)
		}
	}
}

func ClassifyContextReplaySafety(body []byte) ContextReplaySafety {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ContextReplayProtected
	}
	if bodyHasFunctionCallOutput(body) || bodyHasEncryptedReasoning(body) {
		return ContextReplayProtected
	}
	if gjson.GetBytes(body, "messages").IsArray() || gjson.GetBytes(body, "input").IsArray() {
		return ContextReplaySafe
	}
	if strings.TrimSpace(gjson.GetBytes(body, "input").String()) != "" {
		return ContextReplaySafe
	}
	return ContextReplayProtected
}

func (s *OpenAIGatewayService) RecordContextJournalTurn(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	account *Account,
	protocol string,
	requestBody []byte,
	result *OpenAIForwardResult,
) (*ContextJournalTurn, error) {
	return s.RecordContextJournalTurnWithInput(ctx, ContextJournalRecordInput{
		GroupID:     groupID,
		SessionHash: sessionHash,
		Account:     account,
		Protocol:    protocol,
		RequestBody: requestBody,
		Result:      result,
	})
}

func (s *OpenAIGatewayService) RecordContextJournalTurnWithInput(ctx context.Context, input ContextJournalRecordInput) (*ContextJournalTurn, error) {
	if s == nil || s.contextJournal == nil {
		return nil, nil
	}
	sessionHash := strings.TrimSpace(input.SessionHash)
	if sessionHash == "" || input.Account == nil || input.Account.ID <= 0 || len(input.RequestBody) == 0 {
		return nil, nil
	}
	groupID := derefGroupID(input.GroupID)
	responseID := ""
	if input.Result != nil {
		responseID = strings.TrimSpace(input.Result.ResponseID)
		if responseID == "" && input.Result.OpenAIWSMode {
			responseID = strings.TrimSpace(input.Result.RequestID)
		}
	}
	turn, err := s.contextJournal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:               groupID,
		SessionHash:           sessionHash,
		AccountID:             input.Account.ID,
		Protocol:              input.Protocol,
		RequestBody:           input.RequestBody,
		ResponseID:            responseID,
		HasFunctionCallOutput: bodyHasFunctionCallOutput(input.RequestBody),
		HasEncryptedReasoning: bodyHasEncryptedReasoning(input.RequestBody),
	})
	if err != nil {
		return nil, err
	}
	if responseID != "" && turn != nil {
		if err := s.contextJournal.BindResponse(ctx, groupID, responseID, ContextJournalResponseRef{
			GroupID:     groupID,
			SessionHash: sessionHash,
			AccountID:   input.Account.ID,
			TurnID:      turn.TurnID,
		}, s.openAIWSSessionStickyTTL()); err != nil {
			return turn, err
		}
	}
	return turn, nil
}

func bodyHasFunctionCallOutput(body []byte) bool {
	return bytesContainsString(body, `"function_call_output"`)
}

func bodyHasEncryptedReasoning(body []byte) bool {
	return bytesContainsString(body, `"encrypted_content"`)
}

func bodyHasPreviousResponseID(body []byte) bool {
	return bytesContainsString(body, `"previous_response_id"`)
}

func bytesContainsString(body []byte, needle string) bool {
	return bytes.Contains(body, []byte(needle))
}

func contextJournalSessionKey(groupID int64, sessionHash string) string {
	sessionHash = strings.TrimSpace(sessionHash)
	if sessionHash == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", groupID, sessionHash)
}

func contextJournalResponseKey(groupID int64, responseID string) string {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", groupID, responseID)
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneContextJournalTurns(turns []ContextJournalTurn) []ContextJournalTurn {
	out := make([]ContextJournalTurn, len(turns))
	for i := range turns {
		out[i] = cloneContextJournalTurn(turns[i])
	}
	return out
}

func cloneContextJournalTurn(turn ContextJournalTurn) ContextJournalTurn {
	turn.RequestBody = cloneBytes(turn.RequestBody)
	return turn
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func newContextJournalTurnID(now time.Time) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err == nil {
		return fmt.Sprintf("turn_%x_%x", now.UnixNano(), random[:])
	}
	return fmt.Sprintf("turn_%x", now.UnixNano())
}
