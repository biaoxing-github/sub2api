package service

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/stretchr/testify/require"
	"testing"
)

// 付费账号默认端点与显式运维配置的优先级保持可验证。
func TestAntigravityPaidEndpoint(t *testing.T) {
	for _, plan := range []string{"pro", " ULTRA ", "free", "enterprise", ""} {
		t.Setenv(antigravityForwardBaseURLEnv, "")
		a := &Account{Credentials: map[string]any{"plan_type": plan}}
		want := antigravity.BaseURLs[0]
		if plan == "pro" || plan == " ULTRA " {
			want = "https://daily-cloudcode-pa.googleapis.com"
		}
		require.Equal(t, want, resolveAntigravityForwardBaseURL(a))
		t.Setenv(antigravityForwardBaseURLEnv, "prod")
		require.Equal(t, antigravity.BaseURLs[0], resolveAntigravityForwardBaseURL(a))
	}
}
