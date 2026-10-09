package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
)

// Withdrawal flow (Phase 5 funding layer).
//
// Contributors request withdrawals of their matured balance. Admins review
// manually. On approval, a payout ledger entry is recorded.

// Withdrawal statuses.
const (
	WithdrawalPending  = "pending"
	WithdrawalApproved = "approved"
	WithdrawalRejected = "rejected"
)

// Withdrawal is one payout request. Table: withdrawals.
type Withdrawal struct {
	Id            int64  `json:"id"`
	ContributorID int    `json:"contributor_id" gorm:"index"`
	Amount        int64  `json:"amount"`
	Status        string `json:"status" gorm:"type:varchar(32);index"`
	Note          string `json:"note" gorm:"type:text"`
	ReviewedBy    int    `json:"reviewed_by"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index"`
	ReviewedAt    int64  `json:"reviewed_at" gorm:"bigint"`
}

// RequestWithdrawal creates a pending withdrawal if the matured balance covers it.
func RequestWithdrawal(contributorID int, amount int64, note string) (*Withdrawal, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}
	balance, err := GetMaturedBalance(contributorID)
	if err != nil {
		return nil, err
	}
	if balance < amount {
		return nil, errors.New("insufficient matured balance")
	}
	w := &Withdrawal{
		ContributorID: contributorID,
		Amount:        amount,
		Status:        WithdrawalPending,
		Note:          note,
		CreatedAt:     common.GetTimestamp(),
	}
	if err := DB.Create(w).Error; err != nil {
		return nil, err
	}
	return w, nil
}

// ReviewWithdrawal approves or rejects a withdrawal. On approval, records a
// payout ledger entry and flips the corresponding matured entries to paid.
func ReviewWithdrawal(id int64, reviewerID int, approve bool) error {
	var w Withdrawal
	if err := DB.First(&w, id).Error; err != nil {
		return err
	}
	if w.Status != WithdrawalPending {
		return errors.New("withdrawal already reviewed")
	}

	tx := DB.Begin()
	newStatus := WithdrawalRejected
	if approve {
		// Re-check balance under transaction.
		var balance int64
		if err := tx.Model(&ContributorLedger{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("contributor_id = ? AND status = ?", w.ContributorID, LedgerStatusMatured).
			Scan(&balance).Error; err != nil {
			tx.Rollback()
			return err
		}
		if balance < w.Amount {
			tx.Rollback()
			return errors.New("insufficient matured balance")
		}
		newStatus = WithdrawalApproved
		// Record payout.
		if err := tx.Create(&ContributorLedger{
			ContributorID: w.ContributorID,
			Type:          LedgerTypePayout,
			Status:        LedgerStatusPaid,
			Amount:        -w.Amount,
			Note:          "withdrawal payout",
			CreatedAt:     common.GetTimestamp(),
			MaturedAt:     common.GetTimestamp(),
		}).Error; err != nil {
			tx.Rollback()
			return err
		}
		// Mark matured entries as paid (FIFO by id).
		var entries []ContributorLedger
		remaining := w.Amount
		if err := tx.Where("contributor_id = ? AND status = ?", w.ContributorID, LedgerStatusMatured).
			Order("id ASC").Find(&entries).Error; err != nil {
			tx.Rollback()
			return err
		}
		for _, e := range entries {
			if remaining <= 0 {
				break
			}
			if e.Amount <= remaining {
				tx.Model(&ContributorLedger{}).Where("id = ?", e.Id).Update("status", LedgerStatusPaid)
				remaining -= e.Amount
			} else {
				// Partial: split the entry.
				tx.Model(&ContributorLedger{}).Where("id = ?", e.Id).Updates(map[string]any{
					"amount": e.Amount - remaining,
				})
				tx.Create(&ContributorLedger{
					ContributorID: e.ContributorID,
					ChannelID:     e.ChannelID,
					LogID:         e.LogID,
					Type:          e.Type,
					Status:        LedgerStatusPaid,
					Amount:        remaining,
					Note:          "partial payout",
					CreatedAt:     e.CreatedAt,
					MaturedAt:     e.MaturedAt,
				})
				remaining = 0
			}
		}
	}
	if err := tx.Model(&Withdrawal{}).Where("id = ?", id).Updates(map[string]any{
		"status":      newStatus,
		"reviewed_by": reviewerID,
		"reviewed_at": common.GetTimestamp(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
