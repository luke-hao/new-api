package model

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func reservationFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db := prepareBatchFlush(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	if os.Getenv("BATCH_FLUSH_TEST_DSN") == "" {
		sqlDB.SetMaxOpenConns(1)
	}
	oldRedis, oldBatch := common.RedisEnabled, common.BatchUpdateEnabled
	common.RedisEnabled, common.BatchUpdateEnabled = false, true
	t.Cleanup(func() { common.RedisEnabled, common.BatchUpdateEnabled = oldRedis, oldBatch })
	return db
}

func TestWalletReservationConcurrentNoOverdraft(t *testing.T) {
	db := reservationFixture(t)
	var accepted atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	start := make(chan struct{})
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := ReserveWalletQuota(1, 1, "batch-test", 100, false)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrWalletReservation) {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(10), accepted.Load())
	var user User
	var token Token
	require.NoError(t, db.First(&user, 1).Error)
	require.NoError(t, db.First(&token, 1).Error)
	require.Zero(t, user.Quota)
	require.Zero(t, token.RemainQuota)
	require.Equal(t, 1000, token.UsedQuota)
	require.Equal(t, BatchQuotaState{}, GetBatchQuotaState())
}

func TestWalletReservationTokenFailureRollsBackWallet(t *testing.T) {
	db := reservationFixture(t)
	require.NoError(t, db.Model(&Token{}).Where("id = ?", 1).Update("remain_quota", 5).Error)
	require.ErrorIs(t, ReserveWalletQuota(1, 1, "batch-test", 10, false), ErrTokenReservation)
	var user User
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 1000, user.Quota)
	require.ErrorIs(t, ReserveWalletQuota(1, 999, "missing", 10, false), ErrTokenReservation)
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 1000, user.Quota)
}

func TestWalletReservationUnlimitedTokenDoesNotBypassWallet(t *testing.T) {
	db := reservationFixture(t)
	require.NoError(t, db.Model(&Token{}).Where("id = ?", 1).Updates(map[string]interface{}{"unlimited_quota": true, "remain_quota": 0}).Error)
	require.NoError(t, ReserveWalletQuota(1, 1, "batch-test", 1000, false))
	require.ErrorIs(t, ReserveWalletQuota(1, 1, "batch-test", 1, false), ErrWalletReservation)
	require.ErrorIs(t, ReserveWalletQuota(1, 1, "batch-test", 0, false), ErrWalletReservation)
	require.Error(t, ReserveWalletQuota(1, 1, "batch-test", -1, false))
}

func TestWalletReservationRejectsDisabledDeletedAndExpired(t *testing.T) {
	db := reservationFixture(t)
	require.NoError(t, db.Model(&User{}).Where("id = ?", 1).Update("status", common.UserStatusDisabled).Error)
	require.ErrorIs(t, ReserveWalletQuota(1, 1, "batch-test", 1, false), ErrWalletReservation)
	require.NoError(t, db.Model(&User{}).Where("id = ?", 1).Update("status", common.UserStatusEnabled).Error)
	require.NoError(t, db.Model(&Token{}).Where("id = ?", 1).Update("expired_time", common.GetTimestamp()-10).Error)
	require.ErrorIs(t, ReserveWalletQuota(1, 1, "batch-test", 1, false), ErrTokenReservation)
	require.NoError(t, db.Delete(&User{}, 1).Error)
	require.ErrorIs(t, ReserveWalletQuota(1, 1, "batch-test", 1, true), ErrWalletReservation)
}

func TestFinancialWritesDurableEvenWithBatchEnabledAndDeletedRows(t *testing.T) {
	db := reservationFixture(t)
	require.NoError(t, db.Delete(&Token{}, 1).Error)
	require.NoError(t, db.Delete(&User{}, 1).Error)
	require.NoError(t, DecreaseUserQuota(1, 25, false))
	require.NoError(t, DecreaseTokenQuota(1, "batch-test", 25))
	require.NoError(t, IncreaseUserQuota(1, 5, false))
	require.NoError(t, IncreaseTokenQuota(1, "batch-test", 5))
	var user User
	var token Token
	require.NoError(t, db.Unscoped().First(&user, 1).Error)
	require.NoError(t, db.Unscoped().First(&token, 1).Error)
	require.Equal(t, 980, user.Quota)
	require.Equal(t, 980, token.RemainQuota)
	require.Equal(t, 20, token.UsedQuota)
	require.Equal(t, BatchQuotaState{}, GetBatchQuotaState())
	require.Error(t, DecreaseUserQuota(999, 1, false))
	require.Error(t, DecreaseTokenQuota(999, "missing", 1))
}
