package service

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type subscriptionPaymentAffiliateRepoStub struct {
	AffiliateRepository

	invitee *AffiliateSummary
	inviter *AffiliateSummary
	accrual []subscriptionPaymentAffiliateAccrual
}

type subscriptionPaymentAffiliateAccrual struct {
	inviterID     int64
	inviteeUserID int64
	amount        float64
	sourceOrderID *int64
}

type paymentRecoveryGroupRepoStub struct {
	GroupRepository
	group *Group
}

func (s *paymentRecoveryGroupRepoStub) GetByID(context.Context, int64) (*Group, error) {
	return s.group, nil
}

type paymentRecoverySubscriptionRepoStub struct {
	UserSubscriptionRepository

	nextID      int64
	byID        map[int64]*UserSubscription
	byUserGroup map[string]*UserSubscription
}

type paymentRecoveryRedeemRepoStub struct {
	RedeemCodeRepository

	code       *RedeemCode
	getByCalls int
}

func (s *paymentRecoveryRedeemRepoStub) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	s.getByCalls++
	if s.code == nil || s.code.Code != code {
		return nil, ErrRedeemCodeNotFound
	}
	cp := *s.code
	return &cp, nil
}

func newPaymentRecoverySubscriptionRepoStub() *paymentRecoverySubscriptionRepoStub {
	return &paymentRecoverySubscriptionRepoStub{
		nextID:      1,
		byID:        make(map[int64]*UserSubscription),
		byUserGroup: make(map[string]*UserSubscription),
	}
}

func (s *paymentRecoverySubscriptionRepoStub) key(userID, groupID int64) string {
	return strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(groupID, 10)
}

func (s *paymentRecoverySubscriptionRepoStub) seed(sub *UserSubscription) {
	if sub == nil {
		return
	}
	cp := *sub
	if cp.ID == 0 {
		cp.ID = s.nextID
		s.nextID++
	}
	s.byID[cp.ID] = &cp
	s.byUserGroup[s.key(cp.UserID, cp.GroupID)] = &cp
}

func (s *paymentRecoverySubscriptionRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	sub := s.byID[id]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (s *paymentRecoverySubscriptionRepoStub) GetByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	sub := s.byUserGroup[s.key(userID, groupID)]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (s *paymentRecoverySubscriptionRepoStub) Create(_ context.Context, sub *UserSubscription) error {
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	if sub.ID == 0 {
		sub.ID = s.nextID
		s.nextID++
	}
	cp := *sub
	s.byID[cp.ID] = &cp
	s.byUserGroup[s.key(cp.UserID, cp.GroupID)] = &cp
	return nil
}

func (s *subscriptionPaymentAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if s.invitee != nil && s.invitee.UserID == userID {
		cp := *s.invitee
		return &cp, nil
	}
	if s.inviter != nil && s.inviter.UserID == userID {
		cp := *s.inviter
		return &cp, nil
	}
	return &AffiliateSummary{UserID: userID, CreatedAt: time.Now().Add(-time.Hour)}, nil
}

func (s *subscriptionPaymentAffiliateRepoStub) AccrueQuota(_ context.Context, inviterID, inviteeUserID int64, amount float64, _ int, sourceOrderID *int64) (bool, error) {
	var sourceCopy *int64
	if sourceOrderID != nil {
		value := *sourceOrderID
		sourceCopy = &value
	}
	s.accrual = append(s.accrual, subscriptionPaymentAffiliateAccrual{
		inviterID:     inviterID,
		inviteeUserID: inviteeUserID,
		amount:        amount,
		sourceOrderID: sourceCopy,
	})
	return true, nil
}

func TestExecuteSubscriptionFulfillmentRecoversStaleAssignmentWithoutExtendingAgain(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentRecoverySubscriptionOrder(t, ctx, client, OrderStatusRecharging, time.Now().UTC().Add(-6*time.Minute))

	expiresAt := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	subRepo := newPaymentRecoverySubscriptionRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        99,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		StartsAt:  time.Now().Add(-time.Hour),
		ExpiresAt: expiresAt,
		Status:    SubscriptionStatusActive,
		Notes:     "manual note\npayment order " + strconv.FormatInt(order.ID, 10) + "\nretained note",
	})
	groupRepo := &paymentRecoveryGroupRepoStub{
		group: &Group{ID: *order.SubscriptionGroupID, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       groupRepo,
		subscriptionSvc: NewSubscriptionService(groupRepo, subRepo, nil, client, nil),
	}
	require.NoError(t, svc.ExecuteSubscriptionFulfillment(ctx, order.ID))

	sub, err := subRepo.GetByUserIDAndGroupID(ctx, order.UserID, *order.SubscriptionGroupID)
	require.NoError(t, err)
	require.True(t, sub.ExpiresAt.Equal(expiresAt), "recovery must not extend an entitlement already marked by the payment note")

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)

	assignmentAuditCount, err := client.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
			paymentauditlog.ActionEQ("SUBSCRIPTION_ASSIGNED"),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, assignmentAuditCount)
}

func TestPaymentFulfillmentLeaseRejectsFreshAndStaleWorkerCannotComplete(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	freshOrder := createPaymentRecoverySubscriptionOrder(t, ctx, client, OrderStatusRecharging, time.Now().UTC())
	svc := &PaymentService{entClient: client}
	err := svc.RetryFulfillment(ctx, freshOrder.ID)
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))

	staleAt := time.Now().UTC().Add(-paymentFulfillmentLeaseDuration - time.Minute)
	staleOrder := createPaymentRecoverySubscriptionOrder(t, ctx, client, OrderStatusRecharging, staleAt)
	firstLease, err := svc.acquirePaymentFulfillmentLease(ctx, staleOrder)
	require.NoError(t, err)
	require.NotNil(t, firstLease)

	_, err = client.PaymentOrder.UpdateOneID(staleOrder.ID).SetUpdatedAt(staleAt).Save(ctx)
	require.NoError(t, err)
	time.Sleep(time.Millisecond)
	secondOrder, err := client.PaymentOrder.Get(ctx, staleOrder.ID)
	require.NoError(t, err)
	secondLease, err := svc.acquirePaymentFulfillmentLease(ctx, secondOrder)
	require.NoError(t, err)
	require.NotNil(t, secondLease)
	require.False(t, firstLease.version.Equal(secondLease.version))

	err = svc.markCompleted(ctx, staleOrder, firstLease, "SUBSCRIPTION_SUCCESS")
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))
	svc.markFailed(ctx, staleOrder.ID, firstLease, context.DeadlineExceeded)

	reloaded, err := client.PaymentOrder.Get(ctx, staleOrder.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRecharging, reloaded.Status)
	require.NoError(t, svc.markCompleted(ctx, staleOrder, secondLease, "SUBSCRIPTION_SUCCESS"))
}

func TestExecuteBalanceFulfillmentRecoversStaleUsedRedeemCodeWithoutRedeemingAgain(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	staleAt := time.Now().UTC().Add(-paymentFulfillmentLeaseDuration - time.Minute)
	order := createPaymentRecoverySubscriptionOrder(t, ctx, client, OrderStatusRecharging, staleAt)
	order, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetOrderType(payment.OrderTypeBalance).
		ClearPlanID().
		ClearSubscriptionGroupID().
		ClearSubscriptionDays().
		SetUpdatedAt(staleAt).
		Save(ctx)
	require.NoError(t, err)
	redeemRepo := &paymentRecoveryRedeemRepoStub{code: &RedeemCode{
		ID:     101,
		Code:   order.RechargeCode,
		Type:   RedeemTypeBalance,
		Value:  order.Amount,
		Status: StatusUsed,
	}}
	svc := &PaymentService{
		entClient:     client,
		redeemService: &RedeemService{redeemRepo: redeemRepo},
	}

	require.NoError(t, svc.ExecuteBalanceFulfillment(ctx, order.ID))
	require.Equal(t, 1, redeemRepo.getByCalls)
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
}

func TestExecuteSubscriptionFulfillmentAppliesAffiliateRebateFromOrderAmount(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentRecoverySubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now().UTC())
	order, err := client.PaymentOrder.UpdateOneID(order.ID).SetAmount(9.99).SetPayAmount(71.36).Save(ctx)
	require.NoError(t, err)

	inviterID := int64(9001)
	affiliateRepo := &subscriptionPaymentAffiliateRepoStub{
		invitee: &AffiliateSummary{
			UserID:    order.UserID,
			InviterID: &inviterID,
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
		inviter: &AffiliateSummary{UserID: inviterID, CreatedAt: time.Now().Add(-48 * time.Hour)},
	}
	settingSvc := NewSettingService(&paymentConfigSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "15",
	}}, nil)
	groupRepo := &paymentRecoveryGroupRepoStub{
		group: &Group{ID: *order.SubscriptionGroupID, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}
	svc := &PaymentService{
		entClient:        client,
		groupRepo:        groupRepo,
		subscriptionSvc:  NewSubscriptionService(groupRepo, newPaymentRecoverySubscriptionRepoStub(), nil, client, nil),
		affiliateService: NewAffiliateService(affiliateRepo, settingSvc, nil, nil),
	}

	require.NoError(t, svc.ExecuteSubscriptionFulfillment(ctx, order.ID))
	require.Len(t, affiliateRepo.accrual, 1)
	require.Equal(t, inviterID, affiliateRepo.accrual[0].inviterID)
	require.Equal(t, order.UserID, affiliateRepo.accrual[0].inviteeUserID)
	require.InDelta(t, order.Amount*0.15, affiliateRepo.accrual[0].amount, 0.00000001)
	require.NotNil(t, affiliateRepo.accrual[0].sourceOrderID)
	require.Equal(t, order.ID, *affiliateRepo.accrual[0].sourceOrderID)

	applied, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("AFFILIATE_REBATE_APPLIED")).
		Only(ctx)
	require.NoError(t, err)
	require.Contains(t, applied.Detail, `"baseAmount":9.99`)
}

func TestSubscriptionPaymentAffiliateRebateIsClaimedOnlyOnce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentRecoverySubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now().UTC())
	inviterID := int64(9002)
	affiliateRepo := &subscriptionPaymentAffiliateRepoStub{
		invitee: &AffiliateSummary{UserID: order.UserID, InviterID: &inviterID, CreatedAt: time.Now().Add(-time.Hour)},
		inviter: &AffiliateSummary{UserID: inviterID, CreatedAt: time.Now().Add(-time.Hour)},
	}
	settingSvc := NewSettingService(&paymentConfigSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "15",
	}}, nil)
	svc := &PaymentService{
		entClient:        client,
		affiliateService: NewAffiliateService(affiliateRepo, settingSvc, nil, nil),
	}

	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.Len(t, affiliateRepo.accrual, 1)

	count, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("AFFILIATE_REBATE_APPLIED")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBuildAffiliateRebateAuditClaimQuerySupportsSQLiteAndPostgres(t *testing.T) {
	sqliteClient := newPaymentConfigServiceTestClient(t)
	sqliteQuery, sqliteArgs := buildAffiliateRebateAuditClaimQuery(sqliteClient, "42", `{"baseAmount":9.99}`)
	require.Contains(t, sqliteQuery, "CURRENT_TIMESTAMP")
	require.NotContains(t, sqliteQuery, "NOW()")
	require.NotContains(t, sqliteQuery, "::text")
	require.Equal(t, []any{"42", `{"baseAmount":9.99}`, "42"}, sqliteArgs)

	db, err := sql.Open("sqlite", "file:payment-rebate-postgres-dialect?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	postgresClient := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = postgresClient.Close() })
	postgresQuery, postgresArgs := buildAffiliateRebateAuditClaimQuery(postgresClient, "42", `{"baseAmount":9.99}`)
	require.Contains(t, postgresQuery, "$1::text")
	require.Contains(t, postgresQuery, "NOW()")
	require.True(t, strings.Contains(postgresQuery, "ON CONFLICT (order_id, action) DO NOTHING"))
	require.Equal(t, []any{"42", `{"baseAmount":9.99}`}, postgresArgs)
}

func ensurePaymentAuditOrderActionUniqueIndex(t *testing.T, ctx context.Context, client *dbent.Client) {
	t.Helper()
	_, err := client.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_audit_logs_order_action_uniq ON payment_audit_logs(order_id, action)")
	require.NoError(t, err)
}

func createPaymentRecoverySubscriptionOrder(
	t *testing.T,
	ctx context.Context,
	client *dbent.Client,
	status string,
	updatedAt time.Time,
) *dbent.PaymentOrder {
	t.Helper()
	uniqueID := strconv.FormatInt(time.Now().UnixNano(), 10)
	user, err := client.User.Create().
		SetEmail("payment-recovery-" + uniqueID + "@example.com").
		SetPasswordHash("hash").
		SetUsername("payment-recovery-user-" + uniqueID).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(80).
		SetPayAmount(80).
		SetFeeRate(0).
		SetRechargeCode("PAY-RECOVERY-" + uniqueID).
		SetOutTradeNo("sub2_recovery_" + uniqueID).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-recovery").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(100).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(status).
		SetPaidAt(time.Now().Add(-time.Hour)).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetUpdatedAt(updatedAt).
		Save(ctx)
	require.NoError(t, err)
	return order
}
