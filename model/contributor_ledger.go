package model

import (
	"github.com/QuantumNous/new-api/common"
)

// Contributor ledger (Phase 5 funding layer).
//
// Every relayed request through a contributor's channel earns them a share of
// the quota consumed. Entries mature after 7 days, then become withdrawable.

// Ledger entry types.
const (
	LedgerTypeEarning    = "earning"    // revenue share from a relayed call
	LedgerTypePayout     = "payout"     // withdrawal paid out
	LedgerTypeAdjustment = "adjustment" // manual correction by admin
)

// Ledger entry statuses.
const (
	LedgerStatusPending = "pending" // within 7-day maturation window
	LedgerStatusMatured = "matured" // available for withdrawal
	LedgerStatusPaid    = "paid"    // withdrawn
)

// ContributorLedger is one ledger row. Table: contributor_ledger.
type ContributorLedger struct {
	Id            int64  `json:"id"`
	ContributorID int    `json:"contributor_id" gorm:"index"`
	ChannelID     int    `json:"channel_id" gorm:"index"`
	LogID         int    `json:"log_id" gorm:"index"` // source relay log, 0 for adjustments/payouts
	Type          string `json:"type" gorm:"type:varchar(32);index"`
	Status        string `json:"status" gorm:"type:varchar(32);index"`
	Amount        int64  `json:"amount"` // quota units, positive for earnings
	Note          string `json:"note" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index"`
	MaturedAt     int64  `json:"matured_at" gorm:"bigint;index"`
}

// RecordEarning creates a pending ledger entry for a relayed call.
func RecordEarning(contributorID, channelID, logID int, amount int64, matureDays int) error {
	now := common.GetTimestamp()
	return DB.Create(&ContributorLedger{
		ContributorID: contributorID,
		ChannelID:     channelID,
		LogID:         logID,
		Type:          LedgerTypeEarning,
		Status:        LedgerStatusPending,
		Amount:        amount,
		CreatedAt:     now,
		MaturedAt:     now + int64(matureDays)*86400,
	}).Error
}

// GetMaturedBalance returns the withdrawable balance for a contributor.
func GetMaturedBalance(contributorID int) (int64, error) {
	var total int64
	err := DB.Model(&ContributorLedger{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("contributor_id = ? AND status = ?", contributorID, LedgerStatusMatured).
		Scan(&total).Error
	return total, err
}

// GetPendingBalance returns the maturing (not yet withdrawable) balance.
func GetPendingBalance(contributorID int) (int64, error) {
	var total int64
	err := DB.Model(&ContributorLedger{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("contributor_id = ? AND status = ?", contributorID, LedgerStatusPending).
		Scan(&total).Error
	return total, err
}

// MatureEntries flips pending entries past their maturation time to matured.
// Returns the number of entries matured.
func MatureEntries(now int64) (int64, error) {
	res := DB.Model(&ContributorLedger{}).
		Where("status = ? AND matured_at <= ?", LedgerStatusPending, now).
		Update("status", LedgerStatusMatured)
	return res.RowsAffected, res.Error
}
