package service

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
)

func TestRedisContextJournalUsesDedicatedOperationBudget(t *testing.T) {
	shared := redis.NewClient(&redis.Options{
		Addr:                  "127.0.0.1:0",
		ReadTimeout:           time.Second,
		WriteTimeout:          time.Second,
		MaxRetries:            3,
		ContextTimeoutEnabled: false,
	})
	t.Cleanup(func() { _ = shared.Close() })

	journal := NewRedisContextJournal(shared, ContextJournalOptions{OperationTimeout: 25 * time.Millisecond})
	redisJournal, ok := journal.(*redisContextJournal)
	if !ok {
		t.Fatalf("journal = %T, want *redisContextJournal", journal)
	}
	if redisJournal.operationTimeout != 25*time.Millisecond {
		t.Fatalf("operation timeout = %v, want 25ms", redisJournal.operationTimeout)
	}
	if redisJournal.rdb == shared {
		t.Fatal("journal must use a dedicated Redis client view")
	}
	if redisJournal.rdb.Options().ReadTimeout != 25*time.Millisecond || redisJournal.rdb.Options().WriteTimeout != 25*time.Millisecond {
		t.Fatalf("journal Redis timeouts = read %v write %v", redisJournal.rdb.Options().ReadTimeout, redisJournal.rdb.Options().WriteTimeout)
	}
	if !redisJournal.rdb.Options().ContextTimeoutEnabled {
		t.Fatal("journal Redis client must honor context deadlines")
	}
	if shared.Options().ReadTimeout != time.Second || shared.Options().ContextTimeoutEnabled {
		t.Fatal("journal options must not mutate the shared Redis client")
	}
}

func TestRedisContextJournalOperationBudgetBoundsStalledRedis(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()
	t.Cleanup(func() {
		select {
		case conn := <-accepted:
			_ = conn.Close()
		default:
		}
	})

	shared := redis.NewClient(&redis.Options{
		Addr:         listener.Addr().String(),
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = shared.Close() })
	journal := NewRedisContextJournal(shared, ContextJournalOptions{OperationTimeout: 30 * time.Millisecond})

	started := time.Now()
	_, err = journal.GetResponse(context.Background(), 1, "resp_stalled")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetResponse() error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed >= 200*time.Millisecond {
		t.Fatalf("stalled Redis blocked for %v, want journal operation budget", elapsed)
	}
}

func TestRedisContextJournalDisconnectedRedisReturnsPromptly(t *testing.T) {
	shared := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:0",
		DialTimeout:  20 * time.Millisecond,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = shared.Close() })
	journal := NewRedisContextJournal(shared, ContextJournalOptions{OperationTimeout: 50 * time.Millisecond})

	started := time.Now()
	_, err := journal.GetResponse(context.Background(), 1, "resp_disconnected")
	if err == nil {
		t.Fatal("GetResponse() error = nil, want Redis disconnect error")
	}
	if elapsed := time.Since(started); elapsed >= 250*time.Millisecond {
		t.Fatalf("disconnected Redis blocked for %v", elapsed)
	}
}

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
