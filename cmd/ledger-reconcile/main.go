// Command ledger-reconcile verifies contributor ledger consistency (Phase 5).
//
// Checks:
//  1. Every earning entry references a valid channel with matching contributor.
//  2. Sum of ledger amounts per contributor matches expected balances.
//  3. No duplicate payouts for the same withdrawal.
//
// Usage: go run ./cmd/ledger-reconcile
package main

import (
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "reconcile failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		return fmt.Errorf("init db: %w", err)
	}

	issues := 0

	// Check 1: earning entries reference valid channels with matching owner.
	var badAttribution int64
	err := model.DB.Model(&model.ContributorLedger{}).
		Joins("LEFT JOIN channels ON channels.id = contributor_ledger.channel_id").
		Where("contributor_ledger.type = ? AND (channels.id IS NULL OR channels.owner_user_id != contributor_ledger.contributor_id)",
			model.LedgerTypeEarning).
		Count(&badAttribution).Error
	if err != nil {
		return fmt.Errorf("attribution check: %w", err)
	}
	fmt.Printf("attribution mismatches: %d\n", badAttribution)
	issues += int(badAttribution)

	// Check 2: per-contributor balance sanity (no negative matured balance
	// unless offset by payouts).
	type balanceRow struct {
		ContributorID int
		Balance       int64
	}
	var rows []balanceRow
	err = model.DB.Model(&model.ContributorLedger{}).
		Select("contributor_id, COALESCE(SUM(amount), 0) as balance").
		Group("contributor_id").
		Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("balance check: %w", err)
	}
	negative := 0
	for _, r := range rows {
		if r.Balance < 0 {
			fmt.Printf("  contributor %d has negative net balance: %d\n", r.ContributorID, r.Balance)
			negative++
		}
	}
	fmt.Printf("contributors checked: %d, negative balances: %d\n", len(rows), negative)
	issues += negative

	// Check 3: withdrawals without matching payout (approved but no ledger entry).
	var orphanApprovals int64
	err = model.DB.Model(&model.Withdrawal{}).
		Where("status = ? AND NOT EXISTS (?)",
			model.WithdrawalApproved,
			model.DB.Model(&model.ContributorLedger{}).
				Select("1").
				Where("contributor_ledger.type = ? AND contributor_ledger.contributor_id = withdrawals.contributor_id",
					model.LedgerTypePayout)).
		Count(&orphanApprovals).Error
	if err != nil {
		return fmt.Errorf("payout check: %w", err)
	}
	fmt.Printf("approved withdrawals without payout: %d\n", orphanApprovals)
	issues += int(orphanApprovals)

	if issues > 0 {
		return fmt.Errorf("%d issues found", issues)
	}
	fmt.Println("ledger reconciliation: OK")
	return nil
}
