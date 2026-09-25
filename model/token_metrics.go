package model

import "context"

type TokenDailyUsage struct {
	TokenID int   `gorm:"column:token_id" json:"id"`
	Quota   int64 `json:"quota"`
	Tokens  int64 `json:"tokens"`
}

func GetTokenDailyUsage(ctx context.Context, userID int, ids []int, start, end int64) ([]TokenDailyUsage, error) {
	rows := []TokenDailyUsage{}
	err := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("token_id, COALESCE(SUM(CASE WHEN type = ? THEN quota WHEN type = ? THEN -quota ELSE 0 END), 0) AS quota, COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS tokens", LogTypeConsume, LogTypeRefund, LogTypeConsume).
		Where("user_id = ? AND token_id IN ? AND created_at >= ? AND created_at < ? AND type IN ?", userID, ids, start, end, []int{LogTypeConsume, LogTypeRefund}).
		Group("token_id").Scan(&rows).Error
	return rows, err
}
