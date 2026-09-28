package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"os"
	"path/filepath"
	"time"
)

// BillingAdjustment is a durable settlement intent. Its completion and all
// financial writes commit together; failed intents remain pending for retry.
type BillingAdjustment struct {
	ID             string `gorm:"primaryKey;type:varchar(191)"`
	UserID         int
	TokenID        int
	SubscriptionID int
	Delta          int
	TaskID         int64 `gorm:"index"`
	MidjourneyID   int
	ChannelID      int
	UsedQuota      int
	RequestCount   int
	ActualQuota    int
	Status         string `gorm:"type:varchar(20);index"`
	Attempts       int
	LastError      string `gorm:"type:text"`
	CreatedAt      int64
	UpdatedAt      int64 `gorm:"index"`
}

func QueueBillingAdjustment(a *BillingAdjustment) error {
	if a.ID == "" || a.UserID <= 0 {
		return fmt.Errorf("invalid billing adjustment")
	}
	a.Status = "pending"
	a.CreatedAt = common.GetTimestamp()
	a.UpdatedAt = a.CreatedAt
	err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(a).Error
	if err == nil {
		return nil
	}
	if spoolErr := spoolBillingAdjustment(a); spoolErr != nil {
		return fmt.Errorf("billing intent database failure: %v; local journal failure: %w", err, spoolErr)
	}
	return fmt.Errorf("billing intent saved to local journal for retry: %w", err)
}

// AdjustBillingQuotaTx never uses cache or a batch queue for money. Existing
// soft-deleted rows are settled; a physically missing row rolls back everything.
func AdjustBillingQuotaTx(tx *gorm.DB, userID, tokenID, subscriptionID, delta int) (string, error) {
	if delta == 0 {
		return "", nil
	}
	var user User
	if err := tx.Unscoped().Select("id").First(&user, userID).Error; err != nil {
		return "", err
	}
	if subscriptionID > 0 {
		var sub UserSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", subscriptionID, userID).First(&sub).Error; err != nil {
			return "", err
		}
		next, err := common.SafeAddInt("subscription settlement", int(sub.AmountUsed), delta)
		if err != nil {
			return "", err
		}
		if next < 0 {
			next = 0
		}
		// Completed supplier usage is payable even if it exceeded the reservation.
		if err := tx.Model(&sub).Update("amount_used", int64(next)).Error; err != nil {
			return "", err
		}
	} else {
		result := tx.Unscoped().Model(&User{}).Where("id = ?", userID).UpdateColumn("quota", gorm.Expr("quota - ?", delta))
		if result.Error != nil {
			return "", result.Error
		}
		if result.RowsAffected != 1 {
			return "", fmt.Errorf("wallet %d missing", userID)
		}
	}
	key := ""
	if tokenID > 0 {
		var token Token
		if err := tx.Unscoped().Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
			return "", err
		}
		result := tx.Unscoped().Model(&Token{}).Where("id = ? AND user_id = ?", tokenID, userID).Updates(map[string]interface{}{
			"remain_quota": gorm.Expr("remain_quota - ?", delta), "used_quota": gorm.Expr("used_quota + ?", delta), "accessed_time": common.GetTimestamp(),
		})
		if result.Error != nil {
			return "", result.Error
		}
		if result.RowsAffected != 1 {
			return "", fmt.Errorf("token %d missing", tokenID)
		}
		key = token.Key
	}
	return key, nil
}

func ApplyBillingAdjustment(id string) error {
	var a BillingAdjustment
	var tokenKey string
	applied := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Taking the write lock first also serializes workers on SQLite.
		claim := tx.Model(&BillingAdjustment{}).Where("id = ? AND status = ?", id, "pending").UpdateColumn("status", "applying")
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return nil
		}
		if err := tx.First(&a, "id = ?", id).Error; err != nil {
			return err
		}
		var err error
		tokenKey, err = AdjustBillingQuotaTx(tx, a.UserID, a.TokenID, a.SubscriptionID, a.Delta)
		if err != nil {
			return err
		}
		if a.UsedQuota != 0 || a.RequestCount != 0 {
			result := tx.Unscoped().Model(&User{}).Where("id = ?", a.UserID).Updates(map[string]interface{}{
				"used_quota": gorm.Expr("used_quota + ?", a.UsedQuota), "request_count": gorm.Expr("request_count + ?", a.RequestCount),
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("billing statistics user missing")
			}
			// A deleted channel has no statistics row; it must never cancel payment.
			if a.ChannelID > 0 && a.UsedQuota != 0 {
				if err := tx.Model(&Channel{}).Where("id = ?", a.ChannelID).UpdateColumn("used_quota", gorm.Expr("used_quota + ?", a.UsedQuota)).Error; err != nil {
					return err
				}
			}
		}
		if a.MidjourneyID > 0 {
			result := tx.Model(&Midjourney{}).Where("id = ?", a.MidjourneyID).UpdateColumn("quota", 0)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("midjourney billing task missing")
			}
		}
		if a.TaskID > 0 {
			result := tx.Model(&Task{}).Where("id = ?", a.TaskID).UpdateColumn("quota", a.ActualQuota)
			if result.Error != nil {
				return result.Error
			}

			var count int64
			if err := tx.Model(&Task{}).Where("id = ?", a.TaskID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("billing task missing")
			}
		}
		if err := tx.Model(&a).Updates(map[string]interface{}{"status": "completed", "last_error": "", "updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		DB.Model(&BillingAdjustment{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]interface{}{"attempts": gorm.Expr("attempts + 1"), "last_error": err.Error(), "updated_at": common.GetTimestamp()})
		return err
	}
	if applied {
		invalidateReservationCaches(a.UserID, tokenKey, a.TokenID <= 0)
		if a.MidjourneyID > 0 && a.Delta < 0 {
			RecordTaskBillingLog(RecordTaskBillingLogParams{UserId: a.UserID, LogType: LogTypeRefund, Content: "Midjourney task failed", ChannelId: a.ChannelID, Quota: -a.Delta, TokenId: a.TokenID, Other: map[string]interface{}{"billing_adjustment_id": id, "midjourney_id": a.MidjourneyID}})
		}
		if a.TaskID > 0 && a.Delta != 0 {
			var task Task
			if DB.First(&task, a.TaskID).Error == nil {
				typ, amount := LogTypeConsume, a.Delta
				if amount < 0 {
					typ, amount = LogTypeRefund, -amount
				}
				RecordTaskBillingLog(RecordTaskBillingLogParams{UserId: a.UserID, LogType: typ, Content: "task final settlement", ChannelId: task.ChannelId, ModelName: task.Properties.OriginModelName, Quota: amount, TokenId: a.TokenID, Group: task.Group, Other: map[string]interface{}{"task_id": task.TaskID, "billing_adjustment_id": id, "actual_quota": a.ActualQuota}})
			}
		}
	}
	return nil
}

func RetryBillingAdjustments(limit int) error {
	journalErr := replayBillingJournal(limit)
	var pending []BillingAdjustment
	if err := DB.Where("status = ?", "pending").Order("updated_at asc, id asc").Limit(limit).Find(&pending).Error; err != nil {
		return err
	}
	first := journalErr
	for _, a := range pending {
		if err := ApplyBillingAdjustment(a.ID); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func BillingAdjustmentLoop() {
	for {
		if err := RetryBillingAdjustments(200); err != nil {
			common.SysLog("pending billing adjustment: " + err.Error())
		}
		time.Sleep(15 * time.Second)
	}
}

// QueueTaskBilling commits a terminal transition and its settlement intent in
// the same transaction. The fixed task key prevents duplicate callbacks.
func QueueTaskBilling(task *Task, fromStatus *TaskStatus, actual int) (bool, error) {
	if task.ID <= 0 || actual < 0 {
		return false, fmt.Errorf("invalid task settlement")
	}
	won := false
	id := fmt.Sprintf("task:%d:final", task.ID)
	err := DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&Task{}).Where("id = ?", task.ID)
		if fromStatus != nil {
			if *fromStatus == TaskStatusSuccess || *fromStatus == TaskStatusFailure {
				return nil
			}
			q = q.Where("status = ?", *fromStatus)
		}

		var current Task
		if err := q.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var count int64
		if err := tx.Model(&BillingAdjustment{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if current.Quota < 0 {
			return fmt.Errorf("negative task reservation")
		}
		delta, err := common.SafeAddInt("task settlement", actual, -current.Quota)
		if err != nil {
			return err
		}
		if fromStatus != nil {
			task.Quota = current.Quota
			if err := tx.Model(&Task{}).Where("id = ?", task.ID).Select("*").Updates(task).Error; err != nil {
				return err
			}
		}
		subID := 0
		if current.PrivateData.BillingSource == "subscription" {
			subID = current.PrivateData.SubscriptionId
			if subID <= 0 {
				return fmt.Errorf("missing task subscription")
			}
		}
		a := BillingAdjustment{ID: id, UserID: current.UserId, TokenID: current.PrivateData.TokenId, SubscriptionID: subID, Delta: delta, TaskID: task.ID, ActualQuota: actual, Status: "pending", CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp()}
		if delta > 0 {
			a.UsedQuota = delta
			a.ChannelID = current.ChannelId
		}
		if err := tx.Create(&a).Error; err != nil {
			return err
		}
		won = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if err := ApplyBillingAdjustment(id); err != nil {
		return won, err
	}
	if err := DB.Model(&Task{}).Select("quota").Where("id = ?", task.ID).Scan(&task.Quota).Error; err != nil {
		return won, err
	}
	return won, nil
}

func creditUserQuotaTx(tx *gorm.DB, userID, amount int) error {
	if amount <= 0 {
		return fmt.Errorf("credit amount must be positive")
	}
	result := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("quota", gorm.Expr("quota + ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("credit user %d missing", userID)
	}
	return nil
}

// The mounted /data working directory is the fallback when the database cannot
// even accept an intent. No token keys, prompts or credentials enter this journal.
func billingJournalDir() string {
	if dir := os.Getenv("BILLING_JOURNAL_DIR"); dir != "" {
		return dir
	}
	return "billing-pending"
}

func spoolBillingAdjustment(a *BillingAdjustment) error {
	dir := billingJournalDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := common.Marshal(a)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".intent-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	name := fmt.Sprintf("%x.json", sha256.Sum256([]byte(a.ID)))
	if err = os.Rename(f.Name(), filepath.Join(dir, name)); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func replayBillingJournal(limit int) error {
	entries, err := os.ReadDir(billingJournalDir())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var first error
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if count >= limit {
			break
		}
		count++
		path := filepath.Join(billingJournalDir(), entry.Name())
		data, readErr := os.ReadFile(path)
		var a BillingAdjustment
		if readErr == nil {
			readErr = common.Unmarshal(data, &a)
		}
		if readErr == nil && (a.ID == "" || a.UserID <= 0 || a.TaskID > 0 || a.MidjourneyID > 0) {
			readErr = fmt.Errorf("invalid journal intent")
		}
		if readErr == nil {
			a.Status = "pending"
			readErr = DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error
		}
		if readErr == nil {
			readErr = os.Remove(path)
		}
		if readErr != nil && first == nil {
			first = readErr
		}
	}
	return first
}
