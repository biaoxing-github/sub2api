package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
)

func TestRedisContextJournalDisabledWithoutRedis(t *testing.T) {
	journal := NewRedisContextJournal(nil, ContextJournalOptions{})
	_, err := journal.AppendTurn(context.Background(), ContextJournalAppendInput{
		GroupID:     1,
		SessionHash: "session",
		AccountID:   10,
		Protocol:    ContextJournalProtocolOpenAIResponses,
		RequestBody: []byte(`{"input":"hello"}`),
	})
	if err == nil {
		t.Fatal("expected error when redis client is nil")
	}
}

func TestProvideContextJournalFallsBackToMemoryWhenRedisUnavailable(t *testing.T) {
	cfg := testConfigWithRedisContextJournal()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer rdb.Close()

	journal := ProvideContextJournal(rdb, cfg)
	if _, ok := journal.(*memoryContextJournal); !ok {
		t.Fatalf("journal = %T, want *memoryContextJournal", journal)
	}
}

func testConfigWithRedisContextJournal() *config.Config {
	return &config.Config{
		Gateway: config.GatewayConfig{
			ContextJournal: config.GatewayContextJournalConfig{
				Backend:         "redis",
				TTLHours:        24,
				MaxSessionBytes: 1024 * 1024,
			},
		},
	}
}
