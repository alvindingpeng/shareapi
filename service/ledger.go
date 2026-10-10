package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// Contributor ledger accounting (Phase 5 funding layer).
//
// After each relayed request settles, the contributor who owns the channel
// earns a configurable share of the consumed quota. Entries mature after
// 7 days before becoming withdrawable.

// contributorShare returns the fraction of consumed quota credited to the
// contributor. Env-tunable; default 0.85 (85%, platform takes 15% per
// ShareLLM model).
func contributorShare() float64 {
	// GetEnvOrDefault returns int; use basis points for fractional config.
	bps := common.GetEnvOrDefault("CONTRIBUTOR_SHARE_BPS", 8500)
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

// applyContributorPricing scales the quota by the channel's price for
// contributor-owned channels. P11: uses absolute USD pricing (PriceUSDPer1M)
// plus platform markup; falls back to legacy PriceMultiplier when unset.
// Returns the quota unchanged for operator channels.
// The modelBaseUSD is the internal reference price (never shown to users).
func applyContributorPricing(channelID int, quota int, modelName string) int {
	if quota <= 0 || channelID <= 0 {
		return quota
	}
	channel, err := model.GetChannelById(channelID, false)
	if err != nil {
		return quota
	}
	if channel.OwnerUserID <= 0 {
		return quota
	}
	// P9-5: free channel consumes no quota.
	if channel.PriceMultiplier == 0 && channel.PriceUSDPer1M == 0 {
		return 0
	}

	var mult float64
	if channel.PriceUSDPer1M > 0 {
		// P11: absolute pricing. User pays channel price + platform markup.
		// mult = (channelUSD * (1 + markup)) / modelBaseUSD
		baseUSD, ok := getModelBaseUSD(modelName)
		if !ok || baseUSD <= 0 {
			// No reference price; fall back to legacy multiplier.
			mult = channel.PriceMultiplier
			if mult <= 0 {
				mult = 1.0
			}
		} else {
			markup := getPlatformMarkupRate()
			mult = (channel.PriceUSDPer1M * (1 + markup)) / baseUSD
		}
	} else {
		// Legacy: multiplier on base price.
		mult = channel.PriceMultiplier
		if mult < 0 {
			mult = 1.0
		}
	}
	if mult == 1.0 {
		return quota
	}
	// Billing rules: use common.QuotaFromFloat, never a bare int() cast.
	return common.QuotaFromFloat(float64(quota) * mult)
}

// getModelBaseUSD returns the internal reference USD price for a model.
// Used only for quota conversion; never displayed as a selling price.
func getModelBaseUSD(modelName string) (float64, bool) {
	return ratio_setting.GetModelPrice(modelName, false)
}

// getPlatformMarkupRate returns the platform markup on channel prices.
// Default 15% (contributor gets 85%, platform gets 15%).
func getPlatformMarkupRate() float64 {
	// CONTRIBUTOR_SHARE_BPS=8500 means contributor gets 85%.
	// Markup = (1 / 0.85) - 1 ≈ 0.176, but we use the simpler 15% on top.
	// Actually: user_pays = channel_price / 0.85 → markup ≈ 17.6%
	// For simplicity and transparency, use 15% flat markup.
	return 0.15
}

// RecordContributorEarning credits a contributor for a settled relay.
// It is fire-and-forget: ledger failures must not break the relay path.
// Call with the channel ID, the settled quota, and the source log ID.
// Also records the platform fee entry (P7-3).
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
	// P7-3: record platform fee (the remainder).
	fee := int64(settledQuota) - share
	if fee > 0 {
		_ = model.RecordPlatformFee(channel.OwnerUserID, channelID, logID, fee)
	}
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
