package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAIRequestSnapshotDefaultRetentionHours = 72
	openAIRequestSnapshotCleanupInterval       = time.Hour
)

// OpenAIRequestSnapshotRepository 定义 OpenAI 请求快照的持久化能力。
// 快照只保存请求体和路由判定信号，不保存 Authorization/Cookie 等请求头。
type OpenAIRequestSnapshotRepository interface {
	SaveOpenAIRequestSnapshot(ctx context.Context, snapshot *OpenAIRequestSnapshot) error
	DeleteExpiredOpenAIRequestSnapshots(ctx context.Context, now time.Time, limit int) (int64, error)
}

// OpenAIRequestSnapshot 描述一次 OpenAI /responses 请求的本地上下文快照。
// 字段命名与数据库列保持一致，方便后续做路由追踪和重放诊断。
type OpenAIRequestSnapshot struct {
	RequestID               string
	ClientRequestID         string
	SessionHash             string
	PromptCacheKey          string
	UserID                  *int64
	APIKeyID                *int64
	AccountID               *int64
	GroupID                 *int64
	Model                   string
	RequestBodySHA256       string
	RequestBodyBytes        int
	RequestBody             json.RawMessage
	ContextMigrationClass   string
	ContextMigrationReason  string
	SnapshotReplayable      bool
	ReplayBlockReason       string
	HasPreviousResponseID   bool
	PreviousResponseIDKind  string
	PreviousResponseIDLen   int
	InputItemCount          int
	MessageItemCount        int
	FunctionCallOutputCount int
	CustomToolOutputCount   int
	ReasoningItemCount      int
	InputTypeCounts         map[string]int
	CreatedAt               time.Time
	ExpiresAt               time.Time
}

// OpenAIRequestSnapshotService 负责把热路径请求快照异步写入本地数据库。
// 写入失败只记录日志，不阻塞用户请求。
type OpenAIRequestSnapshotService struct {
	repo           OpenAIRequestSnapshotRepository
	settingService *SettingService
	queue          chan OpenAIRequestSnapshot
	stopCh         chan struct{}
	doneCh         chan struct{}
}

func NewOpenAIRequestSnapshotService(repo OpenAIRequestSnapshotRepository, settingService *SettingService) *OpenAIRequestSnapshotService {
	if repo == nil {
		return nil
	}
	s := &OpenAIRequestSnapshotService{
		repo:           repo,
		settingService: settingService,
		queue:          make(chan OpenAIRequestSnapshot, 512),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
	go s.loop()
	return s
}

func (s *OpenAIRequestSnapshotService) Close() {
	if s == nil {
		return
	}
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	select {
	case <-s.doneCh:
	case <-time.After(5 * time.Second):
	}
}

func (s *OpenAIRequestSnapshotService) Enabled(ctx context.Context) bool {
	return s != nil && s.repo != nil && s.settingService != nil && s.settingService.IsOpenAIRequestSnapshotEnabled(ctx)
}

func (s *OpenAIRequestSnapshotService) RetentionHours(ctx context.Context) int {
	if s == nil || s.settingService == nil {
		return openAIRequestSnapshotDefaultRetentionHours
	}
	hours := s.settingService.GetOpenAIRequestSnapshotRetentionHours(ctx)
	if hours <= 0 {
		return openAIRequestSnapshotDefaultRetentionHours
	}
	return hours
}

func (s *OpenAIRequestSnapshotService) Enqueue(snapshot OpenAIRequestSnapshot) {
	if s == nil || s.repo == nil {
		return
	}
	select {
	case s.queue <- snapshot:
	default:
		logger.LegacyPrintf("service.openai_request_snapshot", "[OpenAI] request snapshot queue full; drop request_id=%s", snapshot.RequestID)
	}
}

func (s *OpenAIRequestSnapshotService) loop() {
	defer close(s.doneCh)
	cleanupTicker := time.NewTicker(openAIRequestSnapshotCleanupInterval)
	defer cleanupTicker.Stop()
	for {
		select {
		case snapshot := <-s.queue:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.repo.SaveOpenAIRequestSnapshot(ctx, &snapshot); err != nil {
				logger.LegacyPrintf("service.openai_request_snapshot", "[OpenAI] save request snapshot failed request_id=%s err=%v", snapshot.RequestID, err)
			}
			cancel()
		case <-cleanupTicker.C:
			s.cleanupExpired()
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpenAIRequestSnapshotService) cleanupExpired() {
	if s == nil || s.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		deleted, err := s.repo.DeleteExpiredOpenAIRequestSnapshots(ctx, time.Now(), 1000)
		if err != nil {
			logger.LegacyPrintf("service.openai_request_snapshot", "[OpenAI] cleanup expired request snapshots failed err=%v", err)
			return
		}
		if deleted < 1000 {
			return
		}
	}
}

func buildOpenAIRequestSnapshotFromGin(c *gin.Context, account *Account, body []byte, model, promptCacheKey string, obs OpenAIContextMigrationObservation, retentionHours int) (OpenAIRequestSnapshot, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return OpenAIRequestSnapshot{}, false
	}
	if retentionHours <= 0 {
		retentionHours = openAIRequestSnapshotDefaultRetentionHours
	}
	now := time.Now()
	sum := sha256.Sum256(body)
	requestID := ""
	if c != nil {
		requestID = strings.TrimSpace(c.GetString("request_id"))
	}
	if requestID == "" {
		requestID = hex.EncodeToString(sum[:8])
	}
	var accountID *int64
	if account != nil && account.ID > 0 {
		id := account.ID
		accountID = &id
	}
	return OpenAIRequestSnapshot{
		RequestID:               requestID,
		ClientRequestID:         strings.TrimSpace(ginString(c, "client_request_id")),
		SessionHash:             strings.TrimSpace(ginString(c, "session_hash")),
		PromptCacheKey:          strings.TrimSpace(promptCacheKey),
		UserID:                  ginInt64Ptr(c, "user_id"),
		APIKeyID:                ginInt64Ptr(c, "api_key_id"),
		AccountID:               accountID,
		GroupID:                 ginInt64Ptr(c, "group_id"),
		Model:                   strings.TrimSpace(model),
		RequestBodySHA256:       hex.EncodeToString(sum[:]),
		RequestBodyBytes:        len(body),
		RequestBody:             append(json.RawMessage(nil), body...),
		ContextMigrationClass:   strings.TrimSpace(obs.Class),
		ContextMigrationReason:  strings.TrimSpace(obs.Reason),
		SnapshotReplayable:      obs.IsPortable(),
		ReplayBlockReason:       openAIRequestSnapshotReplayBlockReason(obs),
		HasPreviousResponseID:   obs.HasPreviousResponseID,
		PreviousResponseIDKind:  strings.TrimSpace(obs.PreviousResponseIDKind),
		PreviousResponseIDLen:   obs.PreviousResponseIDLen,
		InputItemCount:          obs.InputItemCount,
		MessageItemCount:        obs.MessageItemCount,
		FunctionCallOutputCount: obs.FunctionCallOutputCount,
		CustomToolOutputCount:   obs.CustomToolOutputCount,
		ReasoningItemCount:      obs.ReasoningItemCount,
		InputTypeCounts:         obs.InputTypeCounts,
		CreatedAt:               now,
		ExpiresAt:               now.Add(time.Duration(retentionHours) * time.Hour),
	}, true
}

func openAIRequestSnapshotReplayBlockReason(obs OpenAIContextMigrationObservation) string {
	if obs.IsPortable() {
		return ""
	}
	if strings.TrimSpace(obs.Reason) != "" {
		return obs.Reason
	}
	return "context_not_portable"
}

func ginString(c *gin.Context, key string) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func ginInt64Ptr(c *gin.Context, key string) *int64 {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(key); ok {
		switch n := v.(type) {
		case int64:
			return &n
		case int:
			out := int64(n)
			return &out
		case uint:
			out := int64(n)
			return &out
		case uint64:
			out := int64(n)
			return &out
		}
	}
	return nil
}
