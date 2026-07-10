package service

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestWithSubscriptionUpdateTxReusesExistingEntTransaction(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, client, nil)
	err = svc.withSubscriptionUpdateTx(dbent.NewTxContext(ctx, tx), func(txCtx context.Context) error {
		require.Same(t, tx, dbent.TxFromContext(txCtx), "subscription write must reuse the payment transaction")
		return nil
	})
	require.NoError(t, err)
}
