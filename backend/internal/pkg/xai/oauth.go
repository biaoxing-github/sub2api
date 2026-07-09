package xai

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	OAuthIssuer         = "https://auth.x.ai"
	DiscoveryURL        = OAuthIssuer + "/.well-known/openid-configuration"
	DefaultAuthorizeURL = OAuthIssuer + "/oauth2/authorize"
	DefaultTokenURL     = OAuthIssuer + "/oauth2/token"
	DefaultBaseURL      = "https://api.x.ai/v1"
	DefaultClientID     = "b1a00492-073a-47ea-816f-4c329264a828"
	DefaultScope        = "openid profile email offline_access grok-cli:access api:access"
	DefaultRedirectURI  = "http://127.0.0.1:56121/callback"
	SessionTTL          = 30 * time.Minute

	EnvAuthorizeURL = "XAI_OAUTH_AUTHORIZE_URL"
	EnvTokenURL     = "XAI_OAUTH_TOKEN_URL"
	EnvClientID     = "XAI_OAUTH_CLIENT_ID"
	EnvScope        = "XAI_OAUTH_SCOPE"
	EnvRedirectURI  = "XAI_OAUTH_REDIRECT_URI"
	EnvBaseURL      = "XAI_BASE_URL"
)

// OAuthSession stores one Grok PKCE OAuth flow.
type OAuthSession struct {
	State         string    `json:"state"`
	CodeVerifier  string    `json:"code_verifier"`
	CodeChallenge string    `json:"code_challenge"`
	ClientID      string    `json:"client_id,omitempty"`
	Scope         string    `json:"scope,omitempty"`
	ProxyURL      string    `json:"proxy_url,omitempty"`
	RedirectURI   string    `json:"redirect_uri"`
	CreatedAt     time.Time `json:"created_at"`
}

// SessionStore manages Grok OAuth sessions in memory.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*OAuthSession
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewSessionStore creates a session store and starts expiration cleanup.
func NewSessionStore() *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*OAuthSession),
		stopCh:   make(chan struct{}),
	}
	go store.cleanup()
	return store
}

// Set stores one temporary OAuth session.
func (s *SessionStore) Set(sessionID string, session *OAuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

// Get returns a non-expired OAuth session.
func (s *SessionStore) Get(sessionID string) (*OAuthSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(session.CreatedAt) > SessionTTL {
		return nil, false
	}
	return session, true
}

// Delete removes a completed OAuth session.
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// Stop stops the background cleanup goroutine.
func (s *SessionStore) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// cleanup periodically deletes expired OAuth sessions.
func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, session := range s.sessions {
				if time.Since(session.CreatedAt) > SessionTTL {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

// EffectiveAuthorizeURL returns the configured Grok authorization endpoint.
func EffectiveAuthorizeURL() string {
	return envOrDefault(EnvAuthorizeURL, DefaultAuthorizeURL)
}

// EffectiveTokenURL returns the configured Grok token endpoint.
func EffectiveTokenURL() string {
	return envOrDefault(EnvTokenURL, DefaultTokenURL)
}

// EffectiveClientID returns the configured Grok OAuth client ID.
func EffectiveClientID() string {
	return envOrDefault(EnvClientID, DefaultClientID)
}

// EffectiveScope returns the configured Grok OAuth scope.
func EffectiveScope() string {
	return envOrDefault(EnvScope, DefaultScope)
}

// EffectiveRedirectURI returns an explicit redirect URI or the configured default.
func EffectiveRedirectURI(override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	return envOrDefault(EnvRedirectURI, DefaultRedirectURI)
}

// EffectiveBaseURL returns an explicit xAI API base URL or the configured default.
func EffectiveBaseURL(override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return strings.TrimRight(trimmed, "/")
	}
	return strings.TrimRight(envOrDefault(EnvBaseURL, DefaultBaseURL), "/")
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// GenerateRandomBytes returns cryptographically random bytes for OAuth values.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateState creates a random OAuth state value.
func GenerateState() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateNonce creates a random OIDC nonce value.
func GenerateNonce() (string, error) {
	bytes, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateSessionID creates an internal OAuth session ID.
func GenerateSessionID() (string, error) {
	bytes, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCodeVerifier creates a PKCE verifier.
func GenerateCodeVerifier() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64URLEncode(bytes), nil
}

// GenerateCodeChallenge creates the S256 PKCE challenge for a verifier.
func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64URLEncode(hash[:])
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// BuildAuthorizationURL builds a Grok OAuth authorization URL.
func BuildAuthorizationURL(state, codeChallenge, redirectURI, nonce string) string {
	redirectURI = EffectiveRedirectURI(redirectURI)

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", EffectiveClientID())
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", EffectiveScope())
	params.Set("state", state)
	params.Set("nonce", nonce)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("plan", "generic")
	params.Set("referrer", "sub2api")

	return fmt.Sprintf("%s?%s", EffectiveAuthorizeURL(), params.Encode())
}

// AuthorizationInput is a parsed manual OAuth callback input.
type AuthorizationInput struct {
	Code  string
	State string
}

// ParseAuthorizationInput accepts a full callback URL, query string, or bare code.
func ParseAuthorizationInput(raw string) AuthorizationInput {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return AuthorizationInput{}
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed != nil {
		values := parsed.Query()
		if code := strings.TrimSpace(values.Get("code")); code != "" {
			return AuthorizationInput{
				Code:  code,
				State: strings.TrimSpace(values.Get("state")),
			}
		}
	}

	queryCandidate := strings.TrimPrefix(trimmed, "?")
	if strings.Contains(queryCandidate, "=") {
		if values, err := url.ParseQuery(queryCandidate); err == nil {
			if code := strings.TrimSpace(values.Get("code")); code != "" {
				return AuthorizationInput{
					Code:  code,
					State: strings.TrimSpace(values.Get("state")),
				}
			}
		}
	}

	return AuthorizationInput{Code: trimmed}
}

// BuildResponsesURL builds the xAI Responses endpoint URL.
func BuildResponsesURL(baseURL string) string {
	return EffectiveBaseURL(baseURL) + "/responses"
}

// TokenResponse represents xAI OAuth token responses.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}
