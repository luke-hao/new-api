package model

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func prepareBatchFlush(t *testing.T) *gorm.DB {
	t.Helper()
	var db *gorm.DB
	var err error
	if dsn := os.Getenv("BATCH_FLUSH_TEST_DSN"); dsn != "" {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	} else {
		db, err = gorm.Open(sqlite.Open("file:"+t.TempDir()+"/batch.db"), &gorm.Config{})
	}
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &Channel{}, &BatchQuotaReceipt{}))
	for _, table := range []string{"users", "tokens", "channels", "batch_quota_receipts"} {
		require.NoError(t, db.Exec("DELETE FROM "+table).Error)
	}
	require.NoError(t, db.Create(&User{Id: 1, Username: "batch-test", AffCode: "batch-test", Quota: 1000}).Error)
	require.NoError(t, db.Create(&Token{Id: 1, UserId: 1, Key: "batch-test", RemainQuota: 1000}).Error)
	require.NoError(t, db.Create(&Channel{Id: 1, Name: "batch-test", Key: "fixture"}).Error)
	oldDB := DB
	DB = db
	pendingQuotaBatch = nil
	for i := range batchUpdateStores {
		batchUpdateStores[i] = make(map[int]int)
	}
	t.Cleanup(func() {
		DB = oldDB
		pendingQuotaBatch = nil
		for i := range batchUpdateStores {
			batchUpdateStores[i] = make(map[int]int)
		}
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	return db
}

func queueBatchDebit(amount int) {
	addNewRecord(BatchUpdateTypeUserQuota, 1, -amount)
	addNewRecord(BatchUpdateTypeUsedQuota, 1, amount)
	addNewRecord(BatchUpdateTypeRequestCount, 1, 1)
	addNewRecord(BatchUpdateTypeTokenQuota, 1, -amount)
	addNewRecord(BatchUpdateTypeChannelUsedQuota, 1, amount)
}

func assertBatchLedger(t *testing.T, db *gorm.DB, amount, count int) {
	t.Helper()
	var user User
	var token Token
	var channel Channel
	require.NoError(t, db.First(&user, 1).Error)
	require.NoError(t, db.First(&token, 1).Error)
	require.NoError(t, db.First(&channel, 1).Error)
	require.Equal(t, 1000-amount, user.Quota)
	require.Equal(t, amount, user.UsedQuota)
	require.Equal(t, count, user.RequestCount)
	require.Equal(t, 1000-amount, token.RemainQuota)
	require.Equal(t, amount, token.UsedQuota)
	require.Equal(t, int64(amount), channel.UsedQuota)
}

func TestBatchFlushAtomicRollbackRetryAndConcurrentNewEntries(t *testing.T) {
	db := prepareBatchFlush(t)
	queueBatchDebit(15)
	name := "fail_batch_channel"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "channels" {
			tx.AddError(errors.New("injected write failure"))
		}
	}))
	require.Error(t, FlushBatchQuota(context.Background()))
	assertBatchLedger(t, db, 0, 0)
	require.True(t, GetBatchQuotaState().PendingBatch)
	queueBatchDebit(7)
	require.NoError(t, db.Callback().Update().Remove(name))
	require.NoError(t, FlushBatchQuota(context.Background()))
	assertBatchLedger(t, db, 15, 1)
	require.NoError(t, FlushBatchQuota(context.Background()))
	assertBatchLedger(t, db, 22, 2)
	require.Equal(t, BatchQuotaState{}, GetBatchQuotaState())
}

func TestBatchFlushLostCommitAcknowledgementIsIdempotent(t *testing.T) {
	db := prepareBatchFlush(t)
	queueBatchDebit(15)
	batch := takeQuotaBatch()
	require.NotNil(t, batch)
	require.NoError(t, applyQuotaBatch(context.Background(), db, batch))
	// Pretend the commit acknowledgement was lost and the same batch is retained.
	pendingQuotaBatch = batch
	require.NoError(t, FlushBatchQuota(context.Background()))
	assertBatchLedger(t, db, 15, 1)
	var receipts int64
	require.NoError(t, db.Model(&BatchQuotaReceipt{}).Count(&receipts).Error)
	require.Equal(t, int64(1), receipts)
}

func TestBatchFlushCancelledContextRetainsQueue(t *testing.T) {
	db := prepareBatchFlush(t)
	queueBatchDebit(15)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, FlushBatchQuota(ctx), context.Canceled)
	require.Positive(t, GetBatchQuotaState().QueuedEntries)
	require.NoError(t, FlushBatchQuota(context.Background()))
	assertBatchLedger(t, db, 15, 1)
}

func TestBatchFlushConcurrentProducers(t *testing.T) {
	db := prepareBatchFlush(t)
	var producers sync.WaitGroup
	for i := 0; i < 10; i++ {
		producers.Add(1)
		go func() {
			defer producers.Done()
			for j := 0; j < 10; j++ {
				queueBatchDebit(1)
			}
		}()
	}
	producers.Wait()
	require.NoError(t, FlushBatchQuota(context.Background()))
	assertBatchLedger(t, db, 100, 100)
}

func TestBatchFlushMissingRowFailsWholeBatch(t *testing.T) {
	db := prepareBatchFlush(t)
	queueBatchDebit(15)
	require.NoError(t, db.Delete(&Channel{}, 1).Error)
	require.Error(t, FlushBatchQuota(context.Background()))
	var u User
	require.NoError(t, db.First(&u, 1).Error)
	require.Equal(t, 1000, u.Quota)
	require.True(t, GetBatchQuotaState().PendingBatch)
}

func TestBatchFlushDuplicateIDNotDoubleApplied(t *testing.T) {
	db := prepareBatchFlush(t)
	queueBatchDebit(3)
	batch := takeQuotaBatch()
	batch.id = uuid.NewString()
	require.NoError(t, applyQuotaBatch(context.Background(), db, batch))
	require.NoError(t, applyQuotaBatch(context.Background(), db, batch))
	assertBatchLedger(t, db, 3, 1)
}
