package model

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type TokenListOptions struct {
	Status       []int
	Group        *string
	Sort         string
	Desc         bool
	ContainsName bool
}

func tokenStatusSQL() string {
	return "CASE WHEN status = 2 THEN 2 WHEN status = 3 OR (expired_time != -1 AND expired_time < ?) THEN 3 WHEN status = 4 OR (unlimited_quota = ? AND remain_quota <= 0) THEN 4 ELSE status END"
}
func EffectiveTokenStatus(t *Token, now int64) int {
	if t.Status == common.TokenStatusDisabled {
		return t.Status
	}
	if t.Status == common.TokenStatusExpired || (t.ExpiredTime != -1 && t.ExpiredTime < now) {
		return common.TokenStatusExpired
	}
	if t.Status == common.TokenStatusExhausted || (!t.UnlimitedQuota && t.RemainQuota <= 0) {
		return common.TokenStatusExhausted
	}
	return t.Status
}
func applyTokenListOptions(q *gorm.DB, opts TokenListOptions) (*gorm.DB, error) {
	if len(opts.Status) > 0 {
		q = q.Where("("+tokenStatusSQL()+") IN ?", time.Now().Unix(), false, opts.Status)
	}
	if opts.Group != nil {
		q = q.Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: *opts.Group})
	}
	columns := map[string]string{"id": "id", "name": "name", "status": "status", "remain_quota": "remain_quota", "used_quota": "used_quota", "group": q.Statement.Quote("group"), "created_time": "created_time", "accessed_time": "accessed_time", "expired_time": "expired_time"}
	if opts.Sort == "" {
		return q.Order("id desc"), nil
	}
	col, ok := columns[opts.Sort]
	if !ok {
		return nil, fmt.Errorf("invalid token sort field")
	}
	direction := " ASC"
	if opts.Desc {
		direction = " DESC"
	}
	return q.Order(col + direction).Order("id desc"), nil
}
func tokenNameContainsPattern(s string) string {
	s = strings.ReplaceAll(s, "!", "!!")
	s = strings.ReplaceAll(s, "%", "!%")
	s = strings.ReplaceAll(s, "_", "!_")
	return "%" + s + "%"
}

// UpdateTokenStatusOnly preserves quota changes made by concurrent requests.
func UpdateTokenStatusOnly(id, userID, status int) (*Token, error) {
	if status != common.TokenStatusEnabled && status != common.TokenStatusDisabled {
		return nil, fmt.Errorf("invalid token status")
	}
	token, err := GetTokenByIds(id, userID)
	if err != nil {
		return nil, err
	}
	q := DB.Model(&Token{}).Where("id = ? AND user_id = ?", id, userID)
	if status == common.TokenStatusEnabled {
		if token.ExpiredTime != -1 && token.ExpiredTime < time.Now().Unix() {
			return nil, fmt.Errorf("token has expired")
		}
		if !token.UnlimitedQuota && token.RemainQuota <= 0 {
			return nil, fmt.Errorf("token quota is exhausted")
		}
		q = q.Where("(expired_time = -1 OR expired_time >= ?) AND (unlimited_quota = ? OR remain_quota > 0)", time.Now().Unix(), true)
	}
	result := q.Update("status", status)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 && token.Status != status {
		return nil, fmt.Errorf("token status changed; refresh and retry")
	}
	if common.RedisEnabled {
		if err := cacheDeleteToken(token.Key); err != nil {
			common.SysError("token status cache invalidation failed: " + err.Error())
		}
	}
	return GetTokenByIds(id, userID)
}
