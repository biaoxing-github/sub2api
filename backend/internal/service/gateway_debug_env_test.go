package service

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDebugEnvBool(t *testing.T) {
	t.Run("empty is false", func(t *testing.T) {
		if parseDebugEnvBool("") {
			t.Fatalf("expected false for empty string")
		}
	})

	t.Run("true-like values", func(t *testing.T) {
		for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
			t.Run(value, func(t *testing.T) {
				if !parseDebugEnvBool(value) {
					t.Fatalf("expected true for %q", value)
				}
			})
		}
	})

	t.Run("false-like values", func(t *testing.T) {
		for _, value := range []string{"0", "false", "off", "debug"} {
			t.Run(value, func(t *testing.T) {
				if parseDebugEnvBool(value) {
					t.Fatalf("expected false for %q", value)
				}
			})
		}
	})
}

func TestDebugLogGatewaySnapshotWritesLargeFullBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway_debug.log")
	svc := &GatewayService{}
	svc.initDebugGatewayBodyFile(path)
	if f := svc.debugGatewayBodyFile.Load(); f != nil {
		defer func() { _ = f.Close() }()
	}

	largeBody := []byte(`{"payload":"` + strings.Repeat("x", 300*1024) + `"}`)
	svc.debugLogGatewaySnapshot("CLIENT_ORIGINAL", http.Header{"Authorization": []string{"Bearer secret"}}, largeBody, nil)
	if f := svc.debugGatewayBodyFile.Load(); f != nil {
		_ = f.Sync()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read gateway debug log: %v", err)
	}
	logText := string(data)
	if !strings.Contains(logText, strings.Repeat("x", 1024)) {
		t.Fatal("expected explicit gateway debug log to include full large body")
	}
	if strings.Contains(logText, "body_omitted: true") {
		t.Fatal("expected explicit gateway debug log to avoid omitting body")
	}
	if strings.Contains(logText, "Bearer secret") {
		t.Fatal("expected authorization header to remain redacted")
	}
}
