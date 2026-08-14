package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type identityUserAgentCacheStub struct {
	fingerprint *Fingerprint
	setCalls    int
}

func (s *identityUserAgentCacheStub) GetFingerprint(_ context.Context, _ int64) (*Fingerprint, error) {
	if s.fingerprint == nil {
		return nil, nil
	}
	clone := *s.fingerprint
	return &clone, nil
}

func (s *identityUserAgentCacheStub) SetFingerprint(_ context.Context, _ int64, fp *Fingerprint) error {
	s.setCalls++
	clone := *fp
	s.fingerprint = &clone
	return nil
}

func (s *identityUserAgentCacheStub) GetMaskedSessionID(_ context.Context, _ int64) (string, error) {
	return "", nil
}

func (s *identityUserAgentCacheStub) SetMaskedSessionID(_ context.Context, _ int64, _ string) error {
	return nil
}

func fingerprintHeadersWithUA(ua string) http.Header {
	header := http.Header{}
	header.Set("User-Agent", ua)
	return header
}

func TestIsAcceptableFingerprintUserAgent(t *testing.T) {
	require.True(t, isAcceptableFingerprintUserAgent("claude-cli/2.1.220 (external, cli)"))
	require.True(t, isAcceptableFingerprintUserAgent("some-sdk/1.2.3 (node)"))
	require.False(t, isAcceptableFingerprintUserAgent("claude-cli/999.0.0-local (undefined, cli)"))
	require.False(t, isAcceptableFingerprintUserAgent("claude-cli/999.0.0 (external, cli)"))
	require.False(t, isAcceptableFingerprintUserAgent("Mozilla/5.0"))
}

func TestGetOrCreateFingerprintRejectsMalformedUserAgentOnCreate(t *testing.T) {
	cache := &identityUserAgentCacheStub{}
	svc := NewIdentityService(cache)

	fingerprint, err := svc.GetOrCreateFingerprint(context.Background(), 1, fingerprintHeadersWithUA("claude-cli/999.0.0-local"))

	require.NoError(t, err)
	require.Equal(t, defaultFingerprint.UserAgent, fingerprint.UserAgent)
	require.Equal(t, 1, cache.setCalls)
}

func TestGetOrCreateFingerprintHealsPoisonedCache(t *testing.T) {
	cache := &identityUserAgentCacheStub{fingerprint: &Fingerprint{
		UserAgent: "claude-cli/999.0.0-local",
		ClientID:  "cid-1",
		UpdatedAt: time.Now().Unix(),
	}}
	svc := NewIdentityService(cache)

	fingerprint, err := svc.GetOrCreateFingerprint(context.Background(), 1, fingerprintHeadersWithUA("claude-cli/2.1.220 (external, cli)"))

	require.NoError(t, err)
	require.Equal(t, "claude-cli/2.1.220 (external, cli)", fingerprint.UserAgent)
	require.Equal(t, "cid-1", fingerprint.ClientID)
	require.Equal(t, 1, cache.setCalls)
}
