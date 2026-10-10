package controller

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// MarketplaceModel is one aggregated model listing on the public market.
type MarketplaceModel struct {
	Model            string  `json:"model"`
	Channels         int     `json:"channels"`
	VerifiedChannels int     `json:"verified_channels"`
	TrustScore       float64 `json:"trust_score"`       // avg 0-100 trust score (P7-6)
	TrustScoreSum    float64 `json:"-"`                 // internal accumulator
	Price            float64 `json:"price"`             // best (lowest) contributor price
	BasePrice        float64 `json:"base_price"`        // platform base price
	MinMultiplier    float64 `json:"min_multiplier"`    // lowest price multiplier
	Available        bool    `json:"available"`
}

// GetMarketplaceModels aggregates enabled contributor channels by model.
// Public endpoint (no auth): powers the marketplace browse page.
// Query params: sort=trust|price|channels (default trust).
func GetMarketplaceModels(c *gin.Context) {
	var channels []*model.Channel
	if err := model.DB.Where("status = ? AND owner_user_id > 0", common.ChannelStatusEnabled).
		Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	agg := map[string]*MarketplaceModel{}
	for _, ch := range channels {
		mult := ch.PriceMultiplier
		if mult <= 0 {
			mult = 1.0
		}
		for _, m := range ch.GetModels() {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			entry, ok := agg[m]
			if !ok {
				entry = &MarketplaceModel{Model: m, MinMultiplier: mult}
				agg[m] = entry
			}
			entry.Channels++
			if ch.VerificationStatus == model.VerificationVerified {
				entry.VerifiedChannels++
			}
			// P7-6: accumulate trust scores for averaging.
			entry.TrustScoreSum += float64(ch.TrustScore)
			// Track the lowest multiplier for best-price display.
			if mult < entry.MinMultiplier {
				entry.MinMultiplier = mult
			}
		}
	}

	pricing := model.GetPricing()
	priceByModel := map[string]float64{}
	for _, p := range pricing {
		if _, ok := priceByModel[p.ModelName]; !ok {
			priceByModel[p.ModelName] = p.ModelPrice
		}
	}

	list := make([]*MarketplaceModel, 0, len(agg))
	for _, entry := range agg {
		if entry.Channels > 0 {
			// P7-6: use average trust score across channels for this model.
			entry.TrustScore = entry.TrustScoreSum / float64(entry.Channels)
			entry.Available = true
		}
		if price, ok := priceByModel[entry.Model]; ok {
			entry.BasePrice = price
			// Effective price = base * contributor's multiplier (best price).
			entry.Price = price * entry.MinMultiplier
		}
		list = append(list, entry)
	}

	switch strings.ToLower(c.Query("sort")) {
	case "price":
		sort.Slice(list, func(i, j int) bool { return list[i].Price < list[j].Price })
	case "channels":
		sort.Slice(list, func(i, j int) bool { return list[i].Channels > list[j].Channels })
	default: // trust
		sort.Slice(list, func(i, j int) bool {
			if list[i].TrustScore != list[j].TrustScore {
				return list[i].TrustScore > list[j].TrustScore
			}
			return list[i].Channels > list[j].Channels
		})
	}

	c.JSON(200, gin.H{"success": true, "message": "", "data": list})
}

// GetProbeCostSummary handles GET /api/marketplace/probe-costs (admin only).
// Returns aggregate fingerprint probe costs for platform accounting (P3-6).
// Query params: start_at, end_at (Unix timestamps, default last 30 days).
func GetProbeCostSummary(c *gin.Context) {
	now := common.GetTimestamp()
	startAt := now - 30*86400
	endAt := now
	if v := c.Query("start_at"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil && ts > 0 {
			startAt = ts
		}
	}
	if v := c.Query("end_at"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil && ts > 0 {
			endAt = ts
		}
	}
	summary, err := model.GetProbeCostSummary(startAt, endAt)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "message": "", "data": summary})
}
