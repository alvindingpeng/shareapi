package controller

import (
	"sort"
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
	TrustScore       float64 `json:"trust_score"`
	Price            float64 `json:"price"`
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
		for _, m := range ch.GetModels() {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			entry, ok := agg[m]
			if !ok {
				entry = &MarketplaceModel{Model: m}
				agg[m] = entry
			}
			entry.Channels++
			if ch.VerificationStatus == model.VerificationVerified {
				entry.VerifiedChannels++
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
			entry.TrustScore = float64(entry.VerifiedChannels) / float64(entry.Channels)
			entry.Available = true
		}
		if price, ok := priceByModel[entry.Model]; ok {
			entry.Price = price
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
