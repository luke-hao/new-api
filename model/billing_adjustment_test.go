package model

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"os"
	"sync"
	"testing"
)

func adjustmentFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db := reservationFixture(t)
	t.Setenv("BILLING_JOURNAL_DIR", t.TempDir())
	require.NoError(t, db.AutoMigrate(&BillingAdjustment{}, &Task{}, &UserSubscription{}, &SubscriptionPlan{}, &SubscriptionPreConsumeRecord{}, &TopUp{}, &Redemption{}, &Log{}, &Midjourney{}, &Checkin{}))
	for _, table := range []string{"billing_adjustments", "tasks", "user_subscriptions", "subscription_plans", "subscription_pre_consume_records", "top_ups", "redemptions", "logs", "midjourneys", "checkins"} {
		require.NoError(t, db.Exec("DELETE FROM "+table).Error)
	}
	oldLog, oldPG, oldSQLite, oldConsume := LOG_DB, common.UsingPostgreSQL, common.UsingSQLite, common.LogConsumeEnabled
	LOG_DB = db
	common.UsingPostgreSQL = db.Dialector.Name() == "postgres"
	common.UsingSQLite = db.Dialector.Name() == "sqlite"
	common.LogConsumeEnabled = false
	t.Cleanup(func() {
		LOG_DB = oldLog
		common.UsingPostgreSQL = oldPG
		common.UsingSQLite = oldSQLite
		common.LogConsumeEnabled = oldConsume
	})
	return db
}

func TestBillingAdjustmentMissingRowRollsBackAndRetriesOnce(t *testing.T) {
	for _, missing := range []string{"user", "token"} {
		for _, delta := range []int{100, -100} {
			t.Run(fmt.Sprintf("%s_%d", missing, delta), func(t *testing.T) {
				db := adjustmentFixture(t)
				uid, tid := 1, 1
				if missing == "user" {
					uid = 2
				} else {
					tid = 2
				}
				a := BillingAdjustment{ID: "retry", UserID: uid, TokenID: tid, Delta: delta}
				require.NoError(t, QueueBillingAdjustment(&a))
				require.Error(t, ApplyBillingAdjustment(a.ID))
				var u User
				var token Token
				require.NoError(t, db.First(&u, 1).Error)
				require.NoError(t, db.First(&token, 1).Error)
				require.Equal(t, 1000, u.Quota)
				require.Equal(t, 1000, token.RemainQuota)
				if missing == "user" {
					require.NoError(t, db.Create(&User{Id: 2, Username: "restored", AffCode: "restored", Quota: 1000}).Error)
					require.NoError(t, db.Model(&Token{}).Where("id = ?", 1).Update("user_id", 2).Error)
				} else {
					require.NoError(t, db.Create(&Token{Id: 2, UserId: 1, Key: "restored", RemainQuota: 1000}).Error)
				}
				require.NoError(t, RetryBillingAdjustments(100))
				require.NoError(t, ApplyBillingAdjustment(a.ID))
				u = User{}
				token = Token{}
				require.NoError(t, db.First(&u, uid).Error)
				require.NoError(t, db.First(&token, tid).Error)
				require.Equal(t, 1000-delta, u.Quota)
				require.Equal(t, 1000-delta, token.RemainQuota)
				require.NoError(t, db.First(&a, "id = ?", a.ID).Error)
				require.Equal(t, "completed", a.Status)
			})
		}
	}
}

func TestBillingAdjustmentConcurrentApplyOnce(t *testing.T) {
	db := adjustmentFixture(t)
	a := BillingAdjustment{ID: "concurrent", UserID: 1, TokenID: 1, Delta: 300}
	require.NoError(t, QueueBillingAdjustment(&a))
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- ApplyBillingAdjustment(a.ID) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var u User
	var token Token
	require.NoError(t, db.First(&u, 1).Error)
	require.NoError(t, db.First(&token, 1).Error)
	require.Equal(t, 700, u.Quota)
	require.Equal(t, 700, token.RemainQuota)
}

func TestBillingAdjustmentTaskTerminalIntentSurvivesFailedRefund(t *testing.T) {
	db := adjustmentFixture(t)
	task := Task{TaskID: "terminal-retry", UserId: 1, ChannelId: 1, Quota: 200, Status: TaskStatusInProgress, PrivateData: TaskPrivateData{TokenId: 2}}
	require.NoError(t, db.Create(&task).Error)
	old := task.Status
	task.Status = TaskStatusFailure
	won, err := QueueTaskBilling(&task, &old, 0)
	require.True(t, won)
	require.Error(t, err)
	var stored Task
	var user User
	require.NoError(t, db.First(&stored, task.ID).Error)
	require.Equal(t, TaskStatus(TaskStatusFailure), stored.Status)
	require.Equal(t, 200, stored.Quota)
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 1000, user.Quota)
	require.NoError(t, db.Create(&Token{Id: 2, UserId: 1, Key: "restored", RemainQuota: 800, UsedQuota: 200}).Error)
	require.NoError(t, RetryBillingAdjustments(100))
	won, err = QueueTaskBilling(&task, &old, 0)
	require.False(t, won)
	require.NoError(t, err)
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 1200, user.Quota)
	require.NoError(t, db.First(&stored, task.ID).Error)
	require.Zero(t, stored.Quota)
}

func TestBillingAdjustmentTaskConcurrentTerminalOnce(t *testing.T) {
	db := adjustmentFixture(t)
	task := Task{TaskID: "terminal-concurrent", UserId: 1, ChannelId: 1, Quota: 200, Status: TaskStatusInProgress, PrivateData: TaskPrivateData{TokenId: 1}}
	require.NoError(t, db.Create(&task).Error)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := task
			old := copy.Status
			copy.Status = TaskStatusSuccess
			_, err := QueueTaskBilling(&copy, &old, 350)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var u User
	var token Token
	require.NoError(t, db.First(&u, 1).Error)
	require.NoError(t, db.First(&token, 1).Error)
	require.Equal(t, 850, u.Quota)
	require.Equal(t, 850, token.RemainQuota)
	var count int64
	require.NoError(t, db.Model(&BillingAdjustment{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestBillingAdjustmentSubscriptionAndTokenAtomic(t *testing.T) {
	db := adjustmentFixture(t)
	require.NoError(t, db.Create(&UserSubscription{Id: 1, UserId: 1, AmountUsed: 200, AmountTotal: 1000, Status: "active", EndTime: common.GetTimestamp() + 3600}).Error)
	a := BillingAdjustment{ID: "sub-retry", UserID: 1, TokenID: 2, SubscriptionID: 1, Delta: 100}
	require.NoError(t, QueueBillingAdjustment(&a))
	require.Error(t, ApplyBillingAdjustment(a.ID))
	var sub UserSubscription
	require.NoError(t, db.First(&sub, 1).Error)
	require.Equal(t, int64(200), sub.AmountUsed)
	require.NoError(t, db.Create(&Token{Id: 2, UserId: 1, Key: "sub-restored", RemainQuota: 800}).Error)
	require.NoError(t, RetryBillingAdjustments(100))
	require.NoError(t, ApplyBillingAdjustment(a.ID))
	require.NoError(t, db.First(&sub, 1).Error)
	require.Equal(t, int64(300), sub.AmountUsed)
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 1000, u.Quota)
}

func TestBillingAdjustmentSoftDeletionStillSettles(t *testing.T) {
	db := adjustmentFixture(t)
	require.NoError(t, db.Delete(&User{}, 1).Error)
	require.NoError(t, db.Delete(&Token{}, 1).Error)
	a := BillingAdjustment{ID: "deleted", UserID: 1, TokenID: 1, Delta: 100}
	require.NoError(t, QueueBillingAdjustment(&a))
	require.NoError(t, ApplyBillingAdjustment(a.ID))
	var u User
	var token Token
	require.NoError(t, db.Unscoped().First(&u, 1).Error)
	require.NoError(t, db.Unscoped().First(&token, 1).Error)
	require.Equal(t, 900, u.Quota)
	require.Equal(t, 900, token.RemainQuota)
}

func TestBillingAdjustmentSubscriptionReservationRollsBackOnTokenFailure(t *testing.T) {
	db := adjustmentFixture(t)
	require.NoError(t, db.Create(&SubscriptionPlan{Id: 1, Title: "atomic", QuotaResetPeriod: "never"}).Error)
	require.NoError(t, db.Create(&UserSubscription{Id: 1, UserId: 1, PlanId: 1, AmountTotal: 1000, Status: "active", EndTime: common.GetTimestamp() + 3600}).Error)
	require.NoError(t, db.Model(&Token{}).Where("id = ?", 1).Update("remain_quota", 50).Error)
	_, err := ReserveSubscriptionQuota("sub-reserve", 1, "fixture", 100, 1)
	require.ErrorIs(t, err, ErrTokenReservation)
	var sub UserSubscription
	require.NoError(t, db.First(&sub, 1).Error)
	require.Zero(t, sub.AmountUsed)
	var count int64
	require.NoError(t, db.Model(&SubscriptionPreConsumeRecord{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestBillingAdjustmentPaymentCallbackConcurrentOnce(t *testing.T) {
	db := adjustmentFixture(t)
	setAffiliateRebateTestPercent(t, "0")
	order := TopUp{UserId: 1, Amount: 1, Money: 1, TradeNo: "concurrent-payment", PaymentProvider: PaymentProviderWaffo, Status: common.TopUpStatusPending}
	require.NoError(t, db.Create(&order).Error)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- RechargeWaffo(order.TradeNo, "") }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 1000+int(common.QuotaPerUnit), u.Quota)
}

func TestBillingAdjustmentCreditMissingUserLeavesOrderPending(t *testing.T) {
	db := adjustmentFixture(t)
	setAffiliateRebateTestPercent(t, "0")
	order := TopUp{UserId: 999, Amount: 1, Money: 1, TradeNo: "missing-credit-user", PaymentProvider: PaymentProviderWaffo, Status: common.TopUpStatusPending}
	require.NoError(t, db.Create(&order).Error)
	require.Error(t, RechargeWaffo(order.TradeNo, ""))
	require.NoError(t, db.First(&order, order.Id).Error)
	require.Equal(t, common.TopUpStatusPending, order.Status)
}

func TestBillingAdjustmentRedemptionConcurrentOnce(t *testing.T) {
	db := adjustmentFixture(t)
	redemption := Redemption{Key: "concurrent-redemption", Quota: 100, Status: common.RedemptionCodeStatusEnabled}
	require.NoError(t, db.Create(&redemption).Error)
	var wg sync.WaitGroup
	results := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			quota, err := Redeem(redemption.Key, 1)
			if err == nil {
				results <- quota
			}
		}()
	}
	wg.Wait()
	close(results)
	total := 0
	for amount := range results {
		total += amount
	}
	require.Equal(t, 100, total)
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 1100, u.Quota)
}

func TestBillingAdjustmentStatisticsAtomicAndDeletedChannel(t *testing.T) {
	db := adjustmentFixture(t)
	require.NoError(t, db.Delete(&Channel{}, 1).Error)
	a := BillingAdjustment{ID: "stats", UserID: 1, TokenID: 2, Delta: 75, UsedQuota: 175, RequestCount: 1, ChannelID: 1}
	require.NoError(t, QueueBillingAdjustment(&a))
	require.Error(t, ApplyBillingAdjustment(a.ID))
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Zero(t, u.UsedQuota)
	require.NoError(t, db.Create(&Token{Id: 2, UserId: 1, Key: "stats-token", RemainQuota: 1000}).Error)
	require.NoError(t, RetryBillingAdjustments(100))
	require.NoError(t, ApplyBillingAdjustment(a.ID))
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 925, u.Quota)
	require.Equal(t, 175, u.UsedQuota)
	require.Equal(t, 1, u.RequestCount)
}

func TestBillingAdjustmentZeroDeltaRecordsUsageOnce(t *testing.T) {
	db := adjustmentFixture(t)
	a := BillingAdjustment{ID: "matched", UserID: 1, TokenID: 1, UsedQuota: 100, RequestCount: 1}
	require.NoError(t, QueueBillingAdjustment(&a))
	require.NoError(t, ApplyBillingAdjustment(a.ID))
	require.NoError(t, ApplyBillingAdjustment(a.ID))
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 1000, u.Quota)
	require.Equal(t, 100, u.UsedQuota)
	require.Equal(t, 1, u.RequestCount)
}

func TestBillingAdjustmentMidjourneyRefundRetryOnce(t *testing.T) {
	db := adjustmentFixture(t)
	mj := Midjourney{UserId: 1, TokenId: 2, Quota: 200, MjId: "retry", Status: "IN_PROGRESS"}
	require.NoError(t, db.Create(&mj).Error)
	before := mj.Status
	mj.Status = "FAILURE"
	mj.Progress = "100%"
	won, err := mj.UpdateWithRefund(before, true)
	require.True(t, won)
	require.Error(t, err)
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 1000, u.Quota)
	require.NoError(t, db.Create(&Token{Id: 2, UserId: 1, Key: "mj-refund", RemainQuota: 800, UsedQuota: 200}).Error)
	require.NoError(t, RetryBillingAdjustments(100))
	won, err = mj.UpdateWithRefund(before, true)
	require.NoError(t, err)
	require.False(t, won)
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 1200, u.Quota)
	var token Token
	require.NoError(t, db.First(&token, 2).Error)
	require.Equal(t, 1000, token.RemainQuota)
	require.Zero(t, token.UsedQuota)
	require.NoError(t, db.First(&mj, mj.Id).Error)
	require.Zero(t, mj.Quota)
}

func TestBillingAdjustmentCheckinMissingUserRollsBack(t *testing.T) {
	db := adjustmentFixture(t)
	_, err := userCheckinWithTransaction(&Checkin{UserId: 99, CheckinDate: "2026-09-28", QuotaAwarded: 100}, 99, 100)
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&Checkin{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestBillingAdjustmentAdminOverrideRejectsStaleBalance(t *testing.T) {
	db := adjustmentFixture(t)
	require.NoError(t, ReserveWalletQuota(1, 1, "batch-test", 100, false))
	require.Error(t, OverrideUserQuota(1, 1000, 2000))
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 900, u.Quota)
}

func TestBillingAdjustmentDatabaseUnavailableJournalReplay(t *testing.T) {
	db := adjustmentFixture(t)
	const callback = "test:billing_database_unavailable"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "billing_adjustments" {
			tx.AddError(fmt.Errorf("database unavailable"))
		}
	}))
	a := BillingAdjustment{ID: "journal-retry", UserID: 1, TokenID: 1, Delta: 100, UsedQuota: 100, RequestCount: 1}
	err := QueueBillingAdjustment(&a)
	require.NoError(t, db.Callback().Create().Remove(callback))
	require.ErrorContains(t, err, "saved to local journal")
	files, err := os.ReadDir(billingJournalDir())
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.NoError(t, RetryBillingAdjustments(100))
	require.NoError(t, RetryBillingAdjustments(100))
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 900, u.Quota)
	require.Equal(t, 100, u.UsedQuota)
	require.Equal(t, 1, u.RequestCount)
	files, err = os.ReadDir(billingJournalDir())
	require.NoError(t, err)
	require.Empty(t, files)
}
