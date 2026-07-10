//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// redeemAtomicAdjustmentUserRepo 在兑换服务测试中记录原子扣减调用，
// 用于确保负值不再根据读取到的旧快照裁剪。
type redeemAtomicAdjustmentUserRepo struct {
	*mockUserRepo
	balanceAdjustments       []float64
	concurrencyAdjustments   []int
	legacyBalanceUpdates     []float64
	legacyConcurrencyUpdates []int
}

func (r *redeemAtomicAdjustmentUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.legacyBalanceUpdates = append(r.legacyBalanceUpdates, amount)
	return nil
}

func (r *redeemAtomicAdjustmentUserRepo) UpdateConcurrency(_ context.Context, _ int64, amount int) error {
	r.legacyConcurrencyUpdates = append(r.legacyConcurrencyUpdates, amount)
	return nil
}

func (r *redeemAtomicAdjustmentUserRepo) ApplyRedeemBalanceAdjustment(_ context.Context, _ int64, delta float64) error {
	r.balanceAdjustments = append(r.balanceAdjustments, delta)
	return nil
}

func (r *redeemAtomicAdjustmentUserRepo) ApplyRedeemConcurrencyAdjustment(_ context.Context, _ int64, delta int) error {
	r.concurrencyAdjustments = append(r.concurrencyAdjustments, delta)
	return nil
}

func TestRedeemServiceUsesAtomicAdjustmentForNegativeRedeemValues(t *testing.T) {
	tests := []struct {
		name             string
		codeType         string
		value            float64
		assertAdjustment func(t *testing.T, repo *redeemAtomicAdjustmentUserRepo)
	}{
		{
			name:     "balance",
			codeType: RedeemTypeBalance,
			value:    -7,
			assertAdjustment: func(t *testing.T, repo *redeemAtomicAdjustmentUserRepo) {
				require.Equal(t, []float64{-7}, repo.balanceAdjustments)
				require.Empty(t, repo.legacyBalanceUpdates)
			},
		},
		{
			name:     "concurrency",
			codeType: RedeemTypeConcurrency,
			value:    -7,
			assertAdjustment: func(t *testing.T, repo *redeemAtomicAdjustmentUserRepo) {
				require.Equal(t, []int{-7}, repo.concurrencyAdjustments)
				require.Empty(t, repo.legacyConcurrencyUpdates)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPaymentOrderLifecycleTestClient(t)
			userRepo := &redeemAtomicAdjustmentUserRepo{
				mockUserRepo: &mockUserRepo{getByIDUser: &User{
					ID:          42,
					Balance:     3,
					Concurrency: 2,
				}},
			}
			redeemRepo := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{
				"REFUND": {
					ID:     1,
					Code:   "REFUND",
					Type:   tt.codeType,
					Value:  tt.value,
					Status: StatusUnused,
				},
			}}
			svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil)

			_, err := svc.Redeem(context.Background(), 42, "REFUND")
			require.NoError(t, err)
			tt.assertAdjustment(t, userRepo)
		})
	}
}
