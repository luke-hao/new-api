package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

type RedemptionFilter struct {
	Keyword  string
	Name     string
	Quota    *int
	Statuses []string
}

func (filter RedemptionFilter) HasConditions() bool {
	return strings.TrimSpace(filter.Keyword) != "" ||
		strings.TrimSpace(filter.Name) != "" ||
		filter.Quota != nil ||
		len(normalizeRedemptionStatusFilters(filter.Statuses)) > 0
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	return ListRedemptions(RedemptionFilter{}, startIdx, num)
}

func SearchRedemptions(keyword string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	return ListRedemptions(RedemptionFilter{Keyword: keyword}, startIdx, num)
}

func ListRedemptions(filter RedemptionFilter, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	countQuery := applyRedemptionFilters(DB.Model(&Redemption{}), filter)
	err = countQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	listQuery := applyRedemptionFilters(DB.Model(&Redemption{}), filter).Order("id desc")
	if num > 0 {
		listQuery = listQuery.Limit(num).Offset(startIdx)
	}
	err = listQuery.Find(&redemptions).Error
	if err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func normalizeRedemptionStatusFilters(statuses []string) []string {
	normalized := make([]string, 0, len(statuses))
	seen := make(map[string]bool)
	for _, raw := range statuses {
		for _, item := range strings.Split(raw, ",") {
			status := strings.TrimSpace(item)
			if status == "" || status == "all" {
				continue
			}
			if status != "expired" {
				if _, err := strconv.Atoi(status); err != nil {
					continue
				}
			}
			if seen[status] {
				continue
			}
			seen[status] = true
			normalized = append(normalized, status)
		}
	}
	return normalized
}

func applyRedemptionFilters(query *gorm.DB, filter RedemptionFilter) *gorm.DB {
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		pattern := "%" + keyword + "%"
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR name LIKE ?", id, pattern)
		} else {
			query = query.Where("name LIKE ?", pattern)
		}
	}

	name := strings.TrimSpace(filter.Name)
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	if filter.Quota != nil {
		query = query.Where("quota = ?", *filter.Quota)
	}

	statuses := normalizeRedemptionStatusFilters(filter.Statuses)
	if len(statuses) > 0 {
		statusValues := make([]int, 0, len(statuses))
		includeExpired := false
		for _, status := range statuses {
			if status == "expired" {
				includeExpired = true
				continue
			}
			value, err := strconv.Atoi(status)
			if err == nil {
				statusValues = append(statusValues, value)
			}
		}

		now := common.GetTimestamp()
		switch {
		case includeExpired && len(statusValues) > 0:
			query = query.Where(
				"(status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?))",
				statusValues,
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case includeExpired:
			query = query.Where(
				"status = ? AND expired_time != 0 AND expired_time < ?",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		default:
			query = query.Where("status IN ?", statusValues)
		}
	}

	return query
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int) (quota int, err error) {
	if key == "" {
		return 0, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return 0, errors.New("无效的 user id")
	}
	redemption := &Redemption{}
	var rebate *AffiliateRebate

	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}
	common.RandomSleep()
	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用")
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("该兑换码已过期")
		}
		err = tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
		if err != nil {
			return err
		}
		redemption.RedeemedTime = common.GetTimestamp()
		redemption.Status = common.RedemptionCodeStatusUsed
		redemption.UsedUserId = userId
		if err = tx.Save(redemption).Error; err != nil {
			return err
		}
		rebate, err = applyAffiliateRedemptionRebate(tx, redemption, userId)
		return err
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		return 0, ErrRedeemFailed
	}
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	recordAffiliateRebateLog(rebate)
	return redemption.Quota, nil
}

func (redemption *Redemption) Insert() error {
	var err error
	err = DB.Create(redemption).Error
	return err
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select("name", "status", "quota", "redeemed_time", "expired_time").Updates(redemption).Error
	return err
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}

func DeleteRedemptionsByIds(ids []int) (int64, error) {
	cleanIds := make([]int, 0, len(ids))
	seen := make(map[int]bool)
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		cleanIds = append(cleanIds, id)
	}
	if len(cleanIds) == 0 {
		return 0, errors.New("ids cannot be empty")
	}

	result := DB.Where("id IN ?", cleanIds).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}

func DeleteRedemptionsByFilter(filter RedemptionFilter) (int64, error) {
	query := applyRedemptionFilters(DB.Model(&Redemption{}), filter)
	if !filter.HasConditions() {
		query = query.Where("1 = 1")
	}
	result := query.Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
