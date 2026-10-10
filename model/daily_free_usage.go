package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
)

// DailyFreeUsage tracks per-user daily usage of free channels (P9-5).
// Prevents abuse of free (price_multiplier=0) channels.
type DailyFreeUsage struct {
	Id        int    `json:"id"`
	UserId    int    `json:"user_id" gorm:"index"`
	Date      string `json:"date" gorm:"type:varchar(10);index"` // YYYY-MM-DD
	Count     int    `json:"count" gorm:"default:0"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (DailyFreeUsage) TableName() string {
	return "daily_free_usage"
}

// GetDailyFreeLimit returns the max free requests per user per day.
// 0 = no limit (default for backward compatibility).
func GetDailyFreeLimit() int {
	return common.GetEnvOrDefault("DAILY_FREE_LIMIT", 0)
}

// CheckAndIncrementFreeUsage checks if the user can make a free request today,
// and increments the counter if so. Returns false if the daily limit is reached.
func CheckAndIncrementFreeUsage(userID int) bool {
	limit := GetDailyFreeLimit()
	if limit <= 0 {
		return true // no limit configured
	}
	today := time.Now().Format("2006-01-02")
	var usage DailyFreeUsage
	err := DB.Where("user_id = ? AND date = ?", userID, today).First(&usage).Error
	now := common.GetTimestamp()
	if err != nil {
		// First request today.
		usage = DailyFreeUsage{
			UserId:    userID,
			Date:      today,
			Count:     1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_ = DB.Create(&usage).Error
		return true
	}
	if usage.Count >= limit {
		return false
	}
	_ = DB.Model(&usage).Updates(map[string]any{
		"count":      usage.Count + 1,
		"updated_at": now,
	}).Error
	return true
}
