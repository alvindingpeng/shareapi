package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Contributor ledger accounting (Phase 5 funding layer).
//
// After each relayed request settles, the contributor who owns the channel
// earns a configurable share of the consumed quota. Entries mature after
// 7 days before becoming withdrawable.

// contributorShare returns the fraction of consumed quota credited to the
// contributor. Env-tunable; default 0.7 (70%).
func contributorShare() float64 {
	// GetEnvOrDefault returns int; use basis points for fractional config.
	bps := common.GetEnvOrDefault("CONTRIBUTOR_SHARE_BPS", 7000)
	if bps < 0 {
		bps = 0
	}
	if bps > 10000 {
		bps = 10000
	}
	return float64(bps) / 10000.0
}

// ledgerMatureDays is the maturation window before earnings are withdrawable.
func ledgerMatureDays() int {
	if d := common.GetEnvOrDefault("LEDGER_MATURE_DAYS", 7); d > 0 {
		return d
	}
	return 7
}

// RecordContributorEarning credits a contributor for a settled relay.
// It is fire-and-forget: ledger failures must not break the relay path.
// Call with the channel ID, the settled quota, and the source log ID.
func RecordContributorEarning(channelID int, settledQuota int, logID int) {
	if settledQuota <= 0 || channelID <= 0 {
		return
	}
	channel, err := model.GetChannelById(channelID, false)
	if err != nil {
		return
	}
	if channel.OwnerUserID <= 0 {
		return // operator-owned channel: no contributor to credit
	}
	share := int64(float64(settledQuota) * contributorShare())
	if share <= 0 {
		return
	}
	_ = model.RecordEarning(channel.OwnerUserID, channelID, logID, share, ledgerMatureDays())
}

// SettlementSummary is the per-run outcome of the settlement task.
type SettlementSummary struct {
	Matured int64 `json:"matured"`
}

// RunSettlementTask matures pending ledger entries past their maturation time.
func RunSettlementTask(ctx context.Context) (*SettlementSummary, error) {
	summary := &SettlementSummary{}
	if ctx.Err() != nil {
		return summary, ctx.Err()
	}
	matured, err := model.MatureEntries(common.GetTimestamp())
	if err != nil {
		return nil, fmt.Errorf("mature ledger entries: %w", err)
	}
	summary.Matured = matured
	return summary, nil
}
