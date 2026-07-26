//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "non-apikey type returns empty",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformAnthropic,
			},
			expected: "",
		},
		{
			name: "apikey without base_url returns default anthropic",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAnthropic,
				Credentials: map[string]any{},
			},
			expected: "https://api.anthropic.com",
		},
		{
			name: "apikey with custom base_url",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAnthropic,
				Credentials: map[string]any{"base_url": "https://custom.example.com"},
			},
			expected: "https://custom.example.com",
		},
		{
			name: "antigravity apikey auto-appends /antigravity",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity apikey trims trailing slash before appending",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com/"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity non-apikey returns empty",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.account.GetBaseURL()
			if result != tt.expected {
				t.Errorf("GetBaseURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetGeminiBaseURL(t *testing.T) {
	const defaultGeminiURL = "https://generativelanguage.googleapis.com"

	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "apikey without base_url returns default",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformGemini,
				Credentials: map[string]any{},
			},
			expected: defaultGeminiURL,
		},
		{
			name: "apikey with custom base_url",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformGemini,
				Credentials: map[string]any{"base_url": "https://custom-gemini.example.com"},
			},
			expected: "https://custom-gemini.example.com",
		},
		{
			name: "antigravity apikey auto-appends /antigravity",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity apikey trims trailing slash",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com/"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity oauth does NOT append /antigravity",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com",
		},
		{
			name: "oauth without base_url returns default",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{},
			},
			expected: defaultGeminiURL,
		},
		{
			name: "nil credentials returns default",
			account: Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformGemini,
			},
			expected: defaultGeminiURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.account.GetGeminiBaseURL(defaultGeminiURL)
			if result != tt.expected {
				t.Errorf("GetGeminiBaseURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestOpenAIRequestBaseURLs(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected []string
	}{
		{
			name: "legacy base_url remains the only request URL",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformOpenAI,
				Credentials: map[string]any{"base_url": "https://legacy.example.com/v1/"},
			},
			expected: []string{"https://legacy.example.com/v1"},
		},
		{
			name: "request_base_urls are normalized and deduped with base_url first",
			account: Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"base_url":             "https://primary.example.com/v1/",
					"request_base_urls":    []any{" https://primary.example.com/v1 ", "https://fast.example.com/", "", "https://fast.example.com"},
					"balance_base_url":     "https://balance.example.com/v1",
					"unrelated_credential": "kept",
				},
			},
			expected: []string{"https://primary.example.com/v1", "https://fast.example.com"},
		},
		{
			name: "non openai account has no openai request URLs",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAnthropic,
				Credentials: map[string]any{"base_url": "https://anthropic.example.com"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.account.GetOpenAIRequestBaseURLs()
			if len(got) != len(tt.expected) {
				t.Fatalf("GetOpenAIRequestBaseURLs() = %#v, want %#v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Fatalf("GetOpenAIRequestBaseURLs()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetGrokBaseURLHonorsManualEndpointSwitch(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{name: "empty falls back to CLI proxy", expected: xai.DefaultCLIBaseURL},
		{name: "official API is honored", baseURL: xai.DefaultBaseURL, expected: xai.DefaultBaseURL},
		{name: "regional API is honored", baseURL: "https://us-west-2.api.x.ai/v1", expected: "https://us-west-2.api.x.ai/v1"},
		{name: "custom relay is honored", baseURL: "https://relay.example.com/v1", expected: "https://relay.example.com/v1"},
		{name: "unparseable value falls back to CLI proxy", baseURL: "not a url", expected: xai.DefaultCLIBaseURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformGrok,
				Credentials: map[string]any{"base_url": tt.baseURL},
			}
			if got := account.GetGrokBaseURL(); got != tt.expected {
				t.Fatalf("GetGrokBaseURL() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAnthropicRequestBaseURLs(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected []string
	}{
		{
			name: "apikey without base_url returns default anthropic request URL",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAnthropic,
				Credentials: map[string]any{},
			},
			expected: []string{"https://api.anthropic.com"},
		},
		{
			name: "request_base_urls are normalized and deduped with base_url first",
			account: Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformAnthropic,
				Credentials: map[string]any{
					"base_url":          "https://primary-anthropic.example.com/",
					"request_base_urls": []any{" https://primary-anthropic.example.com ", "https://backup-anthropic.example.com/", "", "https://backup-anthropic.example.com"},
				},
			},
			expected: []string{"https://primary-anthropic.example.com", "https://backup-anthropic.example.com"},
		},
		{
			name: "non anthropic account has no anthropic request URLs",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformOpenAI,
				Credentials: map[string]any{"base_url": "https://openai.example.com"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.account.GetAnthropicRequestBaseURLs()
			if len(got) != len(tt.expected) {
				t.Fatalf("GetAnthropicRequestBaseURLs() = %#v, want %#v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Fatalf("GetAnthropicRequestBaseURLs()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestOpenAIBalanceBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "explicit balance_base_url wins",
			account: Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"base_url":          "https://request.example.com/v1",
					"request_base_urls": []string{"https://request.example.com/v1", "https://fast.example.com/v1"},
					"balance_base_url":  "https://balance.example.com/v1/",
				},
			},
			expected: "https://balance.example.com/v1",
		},
		{
			name: "empty balance_base_url follows first request URL",
			account: Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"base_url":          "https://request.example.com/v1",
					"request_base_urls": []string{"https://request.example.com/v1", "https://fast.example.com/v1"},
				},
			},
			expected: "https://request.example.com/v1",
		},
		{
			name: "oauth keeps default openai base url",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformOpenAI,
			},
			expected: "https://api.openai.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.GetOpenAIBalanceBaseURL(); got != tt.expected {
				t.Fatalf("GetOpenAIBalanceBaseURL() = %q, want %q", got, tt.expected)
			}
		})
	}
}
