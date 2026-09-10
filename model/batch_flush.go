package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BatchQuotaReceipt commits with the quota changes. Reusing a batch ID after a
// lost database acknowledgement cannot charge it twice. Receipts must not be
// pruned while a process might retry its pending in-memory batch.
type BatchQuotaReceipt struct {
	ID        string `gorm:"type:varchar(36);primaryKey"`
	CreatedAt int64  `gorm:"index"`
}

type quotaBatch struct {
	id     string
	stores []map[int]int
}

var batchFlushMu sync.Mutex
var pendingQuotaBatch *quotaBatch

func sortedBatchIDs(store map[int]int) []int {
	ids := make([]int, 0, len(store))
	for id := range store {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func applyQuotaBatch(ctx context.Context, db *gorm.DB, batch *quotaBatch) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt BatchQuotaReceipt
		err := tx.First(&receipt, "id = ?", batch.id).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&BatchQuotaReceipt{ID: batch.id, CreatedAt: time.Now().Unix()}).Error; err != nil {
			return err
		}
		check := func(result *gorm.DB, kind string, id int) error {
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("batch quota %s %d: expected one existing row", kind, id)
			}
			return nil
		}
		for _, id := range sortedBatchIDs(batch.stores[BatchUpdateTypeTokenQuota]) {
			value := batch.stores[BatchUpdateTypeTokenQuota][id]
			if value == 0 {
				continue
			}
			result := tx.Unscoped().Model(&Token{}).Where("id = ?", id).Updates(map[string]interface{}{
				"remain_quota": gorm.Expr("remain_quota + ?", value), "used_quota": gorm.Expr("used_quota - ?", value), "accessed_time": time.Now().Unix(),
			})
			if err := check(result, "token", id); err != nil {
				return err
			}
		}
		for _, id := range sortedBatchIDs(batch.stores[BatchUpdateTypeChannelUsedQuota]) {
			value := batch.stores[BatchUpdateTypeChannelUsedQuota][id]
			if value == 0 {
				continue
			}
			if err := check(tx.Model(&Channel{}).Where("id = ?", id).Update("used_quota", gorm.Expr("used_quota + ?", value)), "channel", id); err != nil {
				return err
			}
		}
		users := make(map[int]int)
		for _, kind := range []int{BatchUpdateTypeUserQuota, BatchUpdateTypeUsedQuota, BatchUpdateTypeRequestCount} {
			for id := range batch.stores[kind] {
				users[id] = 0
			}
		}
		for _, id := range sortedBatchIDs(users) {
			quota := batch.stores[BatchUpdateTypeUserQuota][id]
			used := batch.stores[BatchUpdateTypeUsedQuota][id]
			count := batch.stores[BatchUpdateTypeRequestCount][id]
			if quota == 0 && used == 0 && count == 0 {
				continue
			}
			result := tx.Unscoped().Model(&User{}).Where("id = ?", id).Updates(map[string]interface{}{
				"quota": gorm.Expr("quota + ?", quota), "used_quota": gorm.Expr("used_quota + ?", used), "request_count": gorm.Expr("request_count + ?", count),
			})
			if err := check(result, "user", id); err != nil {
				return err
			}
		}
		return nil
	})
}

func takeQuotaBatch() *quotaBatch {
	stores := make([]map[int]int, BatchUpdateTypeCount)
	// Snapshot all categories together. Producers hold only one store lock.
	for i := range batchUpdateLocks {
		batchUpdateLocks[i].Lock()
	}
	defer func() {
		for i := len(batchUpdateLocks) - 1; i >= 0; i-- {
			batchUpdateLocks[i].Unlock()
		}
	}()
	hasData := false
	for i := range batchUpdateStores {
		stores[i] = batchUpdateStores[i]
		batchUpdateStores[i] = make(map[int]int)
		if len(stores[i]) > 0 {
			hasData = true
		}
	}
	if !hasData {
		return nil
	}
	return &quotaBatch{id: uuid.NewString(), stores: stores}
}

// FlushBatchQuota persists a stable batch or returns an error while retaining it
// for retry. A nil error is not a whole-application drain guarantee: callers must
// first fence request producers, background jobs and asynchronous refunds.
func FlushBatchQuota(ctx context.Context) error {
	batchFlushMu.Lock()
	defer batchFlushMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if pendingQuotaBatch == nil {
		pendingQuotaBatch = takeQuotaBatch()
	}
	if pendingQuotaBatch == nil {
		return nil
	}
	if err := applyQuotaBatch(ctx, DB, pendingQuotaBatch); err != nil {
		return err
	}
	pendingQuotaBatch = nil
	return nil
}

type BatchQuotaState struct {
	PendingBatch  bool `json:"pending_batch"`
	QueuedEntries int  `json:"queued_entries"`
}

func GetBatchQuotaState() BatchQuotaState {
	batchFlushMu.Lock()
	defer batchFlushMu.Unlock()
	state := BatchQuotaState{PendingBatch: pendingQuotaBatch != nil}
	for i := range batchUpdateStores {
		batchUpdateLocks[i].Lock()
		state.QueuedEntries += len(batchUpdateStores[i])
		batchUpdateLocks[i].Unlock()
	}
	return state
}
