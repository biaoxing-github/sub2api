package service

import (
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestParseGatewayRequest_AnthropicNormalizesClaudeCodeLongContextModelSuffix(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string
	}{
		{name: "lowercase suffix", model: "claude-opus-4-8[1m]", want: "claude-opus-4-8"},
		{name: "uppercase suffix", model: "claude-opus-4-8[1M]", want: "claude-opus-4-8"},
		{name: "duplicated suffix", model: "claude-opus-4-8[1M][1m]", want: "claude-opus-4-8"},
		{name: "suffix in middle", model: "claude-opus-4-8[1m]-preview", want: "claude-opus-4-8[1m]-preview"},
		{name: "suffix only", model: "[1m]", want: "[1m]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"system":"test","messages":[{"role":"user","content":"hi"}]}`, tt.model))
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), domain.PlatformAnthropic)
			require.NoError(t, err)
			require.Equal(t, tt.want, parsed.Model)
			require.Equal(t, tt.want, gjson.GetBytes(parsed.Body.Bytes(), "model").String())
			require.Equal(t, `"test"`, string(parsed.SystemRaw()))
			require.NotEmpty(t, parsed.MessagesRaw())
		})
	}
}

func TestParseGatewayRequest_NonAnthropicPreservesClaudeCodeLongContextModelSuffix(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-8[1m]","input":"hi"}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), "responses")
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8[1m]", parsed.Model)
	require.Equal(t, "claude-opus-4-8[1m]", gjson.GetBytes(parsed.Body.Bytes(), "model").String())
}
