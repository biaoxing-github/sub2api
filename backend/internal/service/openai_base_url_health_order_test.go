package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestOpenAIGatewayServiceOrderedRequestBaseURLsUsesPathHealth(t *testing.T) {
	account := &Account{
		ID:       77,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":          "https://primary.example.com/v1",
			"request_base_urls": []any{"https://primary.example.com/v1", "https://fast.example.com/v1", "https://bad.example.com/v1"},
		},
	}
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:               true,
		CircuitBreakerEnabled: true,
		OpenFailureThreshold:  1,
		Cooldown:              time.Minute,
	})
	svc := &OpenAIGatewayService{
		cfg:              &config.Config{},
		openaiPathHealth: tracker,
	}
	svc.cfg.Gateway.OpenAIPathHealth.Enabled = true
	svc.cfg.Gateway.OpenAIFastLane.Enabled = true
	svc.cfg.Gateway.OpenAIFastLane.MinSamples = 1
	svc.cfg.Gateway.OpenAIFastLane.TTFTWeight = 0
	svc.cfg.Gateway.OpenAIFastLane.HeaderWaitWeight = 1

	primaryWait := int64(1800)
	fastWait := int64(300)
	tracker.RecordSuccess(OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), "https://primary.example.com/v1"), nil, &primaryWait)
	tracker.RecordSuccess(OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), "https://fast.example.com/v1"), nil, &fastWait)
	tracker.RecordFailure(OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), "https://bad.example.com/v1"), "unexpected EOF", nil)

	got := svc.orderedOpenAIRequestBaseURLsForForward(account, OpenAIUpstreamTransportHTTPSSE)
	want := []string{"https://fast.example.com/v1", "https://primary.example.com/v1", "https://bad.example.com/v1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered urls = %#v, want %#v", got, want)
	}
}

func TestOpenAIGatewayServiceOrderedRequestBaseURLsKeepsConfigOrderWithoutSamples(t *testing.T) {
	account := &Account{
		ID:       78,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":          "https://primary.example.com/v1",
			"request_base_urls": []any{"https://primary.example.com/v1", "https://backup.example.com/v1"},
		},
	}
	svc := &OpenAIGatewayService{
		cfg:              &config.Config{},
		openaiPathHealth: NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{Enabled: true}),
	}
	svc.cfg.Gateway.OpenAIPathHealth.Enabled = true
	svc.cfg.Gateway.OpenAIFastLane.Enabled = true
	svc.cfg.Gateway.OpenAIFastLane.MinSamples = 1
	svc.cfg.Gateway.OpenAIFastLane.HeaderWaitWeight = 1

	got := svc.orderedOpenAIRequestBaseURLsForForward(account, OpenAIUpstreamTransportHTTPSSE)
	want := []string{"https://primary.example.com/v1", "https://backup.example.com/v1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered urls = %#v, want %#v", got, want)
	}
}

func TestOpenAIGatewayServiceOrderedRequestBaseURLsAvoidsOpenCircuitWhenFastLaneDisabled(t *testing.T) {
	account := &Account{
		ID:       79,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":          "https://primary.example.com/v1",
			"request_base_urls": []any{"https://primary.example.com/v1", "https://backup.example.com/v1"},
		},
	}
	tracker := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:               true,
		CircuitBreakerEnabled: true,
		OpenFailureThreshold:  1,
		Cooldown:              time.Minute,
	})
	svc := &OpenAIGatewayService{
		cfg:              &config.Config{},
		openaiPathHealth: tracker,
	}
	svc.cfg.Gateway.OpenAIPathHealth.Enabled = true
	svc.cfg.Gateway.OpenAIFastLane.Enabled = false

	tracker.RecordFailure(OpenAIPathHealthKeyForAccountBaseURL(account, string(OpenAIUpstreamTransportHTTPSSE), "https://primary.example.com/v1"), "unexpected EOF", nil)

	got := svc.orderedOpenAIRequestBaseURLsForForward(account, OpenAIUpstreamTransportHTTPSSE)
	want := []string{"https://backup.example.com/v1", "https://primary.example.com/v1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered urls = %#v, want %#v", got, want)
	}
}
