package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrWalletReservation = errors.New("insufficient wallet quota or inactive account")
var ErrTokenReservation = errors.New("insufficient token quota or inactive token")

// ReserveWalletQuota checks and reserves both balances in one durable transaction.
// Cached balances never authorize a reservation. An unlimited token still spends
// the user's wallet, and a token failure rolls back the wallet debit.
func ReserveWalletQuota(userID, tokenID int, tokenKey string, amount int, playground bool) error {
	if amount < 0 {
		return fmt.Errorf("reservation amount cannot be negative")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&User{}).Where("id = ? AND status = ? AND quota > 0 AND quota >= ?", userID, common.UserStatusEnabled, amount)
		if amount == 0 {
			var count int64
			if err := q.Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return ErrWalletReservation
			}
		} else {
			result := q.UpdateColumn("quota", gorm.Expr("quota - ?", amount))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrWalletReservation
			}
		}
		if playground {
			return nil
		}
		return reserveTokenQuota(tx, userID, tokenID, amount)
	})
	if err != nil {
		return err
	}
	invalidateReservationCaches(userID, tokenKey, playground)
	return nil
}

func reserveTokenQuota(tx *gorm.DB, userID, tokenID, amount int) error {
	q := tx.Model(&Token{}).
		Where("id = ? AND user_id = ? AND status = ?", tokenID, userID, common.TokenStatusEnabled).
		Where("(expired_time = ? OR expired_time >= ?)", -1, common.GetTimestamp()).
		Where("(unlimited_quota = ? OR remain_quota >= ?)", true, amount)
	if amount == 0 {
		var count int64
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrTokenReservation
		}
		return nil
	}
	result := q.Updates(map[string]interface{}{
		"remain_quota":  gorm.Expr("remain_quota - ?", amount),
		"used_quota":    gorm.Expr("used_quota + ?", amount),
		"accessed_time": common.GetTimestamp(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrTokenReservation
	}
	return nil
}

// ReserveTokenQuota also protects subscription-funded requests from concurrent
// reuse of a limited token balance.
func ReserveTokenQuota(userID, tokenID int, tokenKey string, amount int) error {
	if amount < 0 {
		return fmt.Errorf("reservation amount cannot be negative")
	}
	if err := reserveTokenQuota(DB, userID, tokenID, amount); err != nil {
		return err
	}
	invalidateReservationCaches(0, tokenKey, false)
	return nil
}

func invalidateReservationCaches(userID int, tokenKey string, playground bool) {
	if !common.RedisEnabled {
		return
	}
	if userID > 0 {
		if err := invalidateUserCache(userID); err != nil {
			common.SysLog("failed to invalidate reserved wallet cache: " + err.Error())
		}
	}
	if !playground {
		if err := cacheDeleteToken(tokenKey); err != nil {
			common.SysLog("failed to invalidate reserved token cache: " + err.Error())
		}
	}
}

func ReserveSubscriptionAdditional(userID, tokenID int, tokenKey string, subID, amount int, playground bool) error {
	if amount <= 0 {
		return fmt.Errorf("reservation must be positive")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		r := tx.Model(&UserSubscription{}).Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", subID, userID, "active", GetDBTimestamp()).Where("amount_total <= 0 OR amount_used <= amount_total - ?", amount).UpdateColumn("amount_used", gorm.Expr("amount_used + ?", amount))
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return fmt.Errorf("subscription quota insufficient")
		}
		if playground {
			return nil
		}
		return reserveTokenQuota(tx, userID, tokenID, amount)
	})
	if err == nil {
		invalidateReservationCaches(userID, tokenKey, playground)
	}
	return err
}
