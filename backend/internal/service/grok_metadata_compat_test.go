package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestGrokMetadataCompatibility 清除 xAI 不支持的元数据，同时保留客户端内容与缓存键。
func TestGrokMetadataCompatibility(t *testing.T) {
	body := []byte(`{"model":"grok-4.5","metadata":{"user_id":"session"},"prompt_cache_key":"cache-key","input":"hello"}`)
	patched, err := patchGrokResponsesBodyBase(body, "grok-4.5")
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(patched, "metadata").Exists())
	require.Equal(t, "hello", gjson.GetBytes(patched, "input").String())
	require.Equal(t, "cache-key", gjson.GetBytes(patched, "prompt_cache_key").String())
	require.True(t, gjson.GetBytes(body, "metadata").Exists())
}
