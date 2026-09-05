package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompositeMessagesDispatchCompatibility 保留 Composite 开关，独立 OpenAI 映射字段仍按既有规则清除。
func TestCompositeMessagesDispatchCompatibility(t *testing.T) {
	for _, platform := range []string{PlatformComposite, PlatformAnthropic} {
		group := &Group{Platform: platform, AllowMessagesDispatch: true, DefaultMappedModel: "gpt-5"}
		sanitizeGroupMessagesDispatchFields(group)
		require.Equal(t, platform == PlatformComposite, group.AllowMessagesDispatch)
		require.Empty(t, group.DefaultMappedModel)
	}
}
