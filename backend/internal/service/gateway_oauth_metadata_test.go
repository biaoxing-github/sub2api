package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOAuthMetadataUserID_FallbackWithoutAccountUUID(t *testing.T) {
	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
		System:         nil,
		Messages:       nil,
	}

	account := &Account{
		ID:    123,
		Type:  AccountTypeOAuth,
		Extra: map[string]any{}, // intentionally missing account_uuid / claude_user_id
	}

	fp := &Fingerprint{ClientID: "deadbeef"} // should be used as user id in legacy format

	got := svc.buildOAuthMetadataUserID(parsed, account, fp)
	require.NotEmpty(t, got)

	parsedUserID := ParseMetadataUserID(got)
	require.NotNil(t, parsedUserID, "unexpected user_id format: %s", got)
	require.Equal(t, "deadbeef", parsedUserID.DeviceID)
	require.Empty(t, parsedUserID.AccountUUID)
	require.NotEmpty(t, parsedUserID.SessionID)
}

func TestBuildOAuthMetadataUserID_UsesAccountUUIDWhenPresent(t *testing.T) {
	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
	}

	account := &Account{
		ID:   123,
		Type: AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid":      "acc-uuid",
			"claude_user_id":    "clientid123",
			"anthropic_user_id": "",
		},
	}

	got := svc.buildOAuthMetadataUserID(parsed, account, nil)
	require.NotEmpty(t, got)

	parsedUserID := ParseMetadataUserID(got)
	require.NotNil(t, parsedUserID, "unexpected user_id format: %s", got)
	require.Equal(t, "clientid123", parsedUserID.DeviceID)
	require.Equal(t, "acc-uuid", parsedUserID.AccountUUID)
	require.NotEmpty(t, parsedUserID.SessionID)
}
