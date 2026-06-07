package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPaymentOrderExpiryService_RunOnceSkipsWhenNotLeader(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	_, _ = cache.TryAcquireLeaderLock(context.Background(), paymentOrderExpiryLeaderLockKey, "peer", time.Minute)

	svc := &PaymentOrderExpiryService{
		lockCache:  cache,
		instanceID: "local",
	}

	require.NotPanics(t, svc.runOnce)
}
