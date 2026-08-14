//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCreateAPIKeyRequest(t *testing.T) {
	positiveDays, zeroDays := 30, 0
	require.NoError(t, validateCreateAPIKeyRequest(CreateAPIKeyRequest{Quota: 0, RateLimit5h: 1e100, ExpiresInDays: &positiveDays}))

	for _, req := range []CreateAPIKeyRequest{
		{Quota: -1},
		{Quota: math.NaN()},
		{RateLimit5h: math.Inf(1)},
		{RateLimit1d: -1},
		{RateLimit7d: -1},
		{ExpiresInDays: &zeroDays},
	} {
		require.Error(t, validateCreateAPIKeyRequest(req))
	}
}

func TestValidateUpdateAPIKeyRequest(t *testing.T) {
	zero, large, negative, nan, inf := 0.0, 1e100, -1.0, math.NaN(), math.Inf(-1)
	require.NoError(t, validateUpdateAPIKeyRequest(UpdateAPIKeyRequest{Quota: &zero, RateLimit7d: &large}))

	for _, req := range []UpdateAPIKeyRequest{
		{Quota: &negative},
		{RateLimit5h: &nan},
		{RateLimit1d: &inf},
		{RateLimit7d: &negative},
	} {
		require.Error(t, validateUpdateAPIKeyRequest(req))
	}
}
