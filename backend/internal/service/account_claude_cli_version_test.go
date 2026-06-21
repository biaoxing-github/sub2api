//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
)

func TestAccountGetClaudeCLIVersion(t *testing.T) {
	t.Run("empty credentials falls back to global version", func(t *testing.T) {
		account := &Account{}
		require.Equal(t, claude.GetCurrentCLIVersion(), account.GetClaudeCLIVersion())
	})

	t.Run("missing override falls back to global version", func(t *testing.T) {
		account := &Account{
			Credentials: map[string]any{
				"api_key": "sk-ant-test",
			},
		}
		require.Equal(t, claude.GetCurrentCLIVersion(), account.GetClaudeCLIVersion())
	})

	t.Run("valid override uses account configured version", func(t *testing.T) {
		account := &Account{
			Credentials: map[string]any{
				"claude_cli_version": " 2.1.126 ",
			},
		}
		require.Equal(t, "2.1.126", account.GetClaudeCLIVersion())
	})

	t.Run("invalid override falls back to global version", func(t *testing.T) {
		account := &Account{
			Credentials: map[string]any{
				"claude_cli_version": "claude-cli/2.1.126",
			},
		}
		require.Equal(t, claude.GetCurrentCLIVersion(), account.GetClaudeCLIVersion())
	})
}
