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

// MarketplaceChannel is one channel listing on the public market (P9-1).
// Exposes per-channel pricing, trust, and sharer info so consumers can
// pick specific channels like a flea market.
type MarketplaceChannel struct {
	ChannelID         int      `json:"channel_id"`
	Name              string   `json:"name"`
	Type              int      `json:"type"`
	Models            []string `json:"models"`
	Sharer            string   `json:"sharer"` // anonymized display name
	PriceMultiplier   float64  `json:"price_multiplier"`
	TrustScore        int      `json:"trust_score"`
	VerificationStatus int     `json:"verification_status"`
	EndpointOfficial  bool     `json:"endpoint_official"`
}

// GetMarketplaceChannels lists individual contributor channels.
// Public endpoint (no auth): powers the channel-level marketplace browse.
// Query params: sort=trust|price (default trust), q=search (name/sharer/channel id),
// brand=channel type filter, free_only=true to show only free channels.
func GetMarketplaceChannels(c *gin.Context) {
	var channels []*model.Channel
	if err := model.DB.Where("status = ? AND owner_user_id > 0", common.ChannelStatusEnabled).
		Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// Batch-load sharer display names.
	ownerIDs := make([]int, 0, len(channels))
	seen := map[int]bool{}
	for _, ch := range channels {
		if !seen[ch.OwnerUserID] {
			seen[ch.OwnerUserID] = true
			ownerIDs = append(ownerIDs, ch.OwnerUserID)
		}
	}
	sharerNames := map[int]string{}
	if len(ownerIDs) > 0 {
		var users []model.User
		_ = model.DB.Select("id", "username", "display_name").Where("id IN ?", ownerIDs).Find(&users).Error
		for _, u := range users {
			name := u.DisplayName
			if name == "" {
				name = u.Username
			}
			// Anonymize: show first 2 chars + ***.
			sharerNames[u.Id] = anonymizeSharer(name)
		}
	}

	// Get latest verification for endpoint official flag.
	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	brandFilter := strings.TrimSpace(c.Query("brand"))
	freeOnly := strings.ToLower(strings.TrimSpace(c.Query("free_only"))) == "true"
	list := make([]*MarketplaceChannel, 0, len(channels))
	for _, ch := range channels {
		// P9-4: brand filter (channel type).
		if brandFilter != "" {
			brandID, err := strconv.Atoi(brandFilter)
			if err != nil || ch.Type != brandID {
				continue
			}
		}
		mult := ch.PriceMultiplier
		// P9-4: free_only filter (checked before normalization).
		if freeOnly && ch.PriceMultiplier != 0 {
			continue
		}
		if mult <= 0 {
			mult = 1.0
		}
		entry := &MarketplaceChannel{
			ChannelID:          ch.Id,
			Name:               ch.Name,
			Type:               ch.Type,
			Models:             ch.GetModels(),
			Sharer:             sharerNames[ch.OwnerUserID],
			PriceMultiplier:    mult,
			TrustScore:         ch.TrustScore,
			VerificationStatus: ch.VerificationStatus,
		}
		// Filter by search query.
		if q != "" {
			haystack := strings.ToLower(entry.Name + " " + entry.Sharer + " " + strconv.Itoa(entry.ChannelID))
			matched := strings.Contains(haystack, q)
			if !matched {
				for _, m := range entry.Models {
					if strings.Contains(strings.ToLower(m), q) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}
		// Endpoint official flag from latest verification log.
		if log, err := model.GetLatestVerificationLog(ch.Id); err == nil && log != nil {
			entry.EndpointOfficial = log.EndpointOfficial
		}
		list = append(list, entry)
	}

	switch strings.ToLower(c.Query("sort")) {
	case "price":
		sort.Slice(list, func(i, j int) bool { return list[i].PriceMultiplier < list[j].PriceMultiplier })
	default: // trust
		sort.Slice(list, func(i, j int) bool {
			if list[i].TrustScore != list[j].TrustScore {
				return list[i].TrustScore > list[j].TrustScore
			}
			return list[i].ChannelID < list[j].ChannelID
		})
	}

	c.JSON(200, gin.H{"success": true, "message": "", "data": list})
}

// anonymizeSharer returns a privacy-preserving display name.
func anonymizeSharer(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "***"
	}
	runes := []rune(name)
	if len(runes) <= 2 {
		return string(runes[:1]) + "***"
	}
	return string(runes[:2]) + "***"
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

// GetMarketplaceStats returns public platform statistics (P9-6).
// Public endpoint (no auth): today's calls, today's tokens, shared models.
func GetMarketplaceStats(c *gin.Context) {
	now := common.GetTimestamp()
	dayStart := now - (now % 86400)

	var todayCalls int64
	_ = model.DB.Model(&model.Log{}).
		Where("created_at >= ?", dayStart).
		Count(&todayCalls).Error

	var todayTokens struct {
		Prompt     int64
		Completion int64
	}
	_ = model.DB.Model(&model.Log{}).
		Select("COALESCE(SUM(prompt_tokens), 0) as prompt, COALESCE(SUM(completion_tokens), 0) as completion").
		Where("created_at >= ?", dayStart).
		Scan(&todayTokens).Error

	var sharedModels int64
	_ = model.DB.Model(&model.Channel{}).
		Where("status = ? AND owner_user_id > 0", common.ChannelStatusEnabled).
		Count(&sharedModels).Error

	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"today_calls":   todayCalls,
			"today_tokens":  todayTokens.Prompt + todayTokens.Completion,
			"shared_models": sharedModels,
		},
	})
}

// GetMarketplaceChannelDetail returns detailed info for one channel (P9-8).
// Public endpoint (no auth): channel info, verification history, stats.
func GetMarketplaceChannelDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(200, gin.H{"success": false, "message": "invalid channel id"})
		return
	}
	ch, err := model.CacheGetChannel(id)
	if err != nil || ch == nil {
		c.JSON(200, gin.H{"success": false, "message": "channel not found"})
		return
	}
	// Only show enabled contributor channels publicly.
	if ch.Status != common.ChannelStatusEnabled || ch.OwnerUserID <= 0 {
		c.JSON(200, gin.H{"success": false, "message": "channel not found"})
		return
	}

	mult := ch.PriceMultiplier
	if mult < 0 {
		mult = 1.0
	}

	// Verification history (last 10).
	var history []model.ChannelVerificationLog
	_ = model.DB.Where("channel_id = ?", id).
		Order("id DESC").Limit(10).Find(&history).Error

	// Stats: total probes, verified count.
	var totalProbes, verifiedCount int64
	_ = model.DB.Model(&model.ChannelVerificationLog{}).
		Where("channel_id = ?", id).Count(&totalProbes).Error
	_ = model.DB.Model(&model.ChannelVerificationLog{}).
		Where("channel_id = ? AND verdict = ?", id, "verified").Count(&verifiedCount).Error

	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"channel_id":          ch.Id,
			"name":                ch.Name,
			"type":                ch.Type,
			"models":              ch.GetModels(),
			"price_multiplier":    mult,
			"trust_score":         ch.TrustScore,
			"verification_status": ch.VerificationStatus,
			"last_verified_at":    ch.LastVerifiedAt,
			"total_probes":        totalProbes,
			"verified_count":      verifiedCount,
			"history":             history,
		},
	})
}

// GetMarketplaceModelChannels lists all enabled contributor channels serving
// a specific model, for price/trust comparison on the model detail page.
// Public endpoint (no auth).
// Query params: sort=price|trust (default price).
func GetMarketplaceModelChannels(c *gin.Context) {
	modelName := strings.TrimSpace(c.Param("name"))
	if modelName == "" {
		c.JSON(400, gin.H{"success": false, "message": "model name required"})
		return
	}

	var channels []*model.Channel
	if err := model.DB.Where("status = ? AND owner_user_id > 0", common.ChannelStatusEnabled).
		Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// Batch-load sharer display names.
	ownerIDs := make([]int, 0, len(channels))
	seen := map[int]bool{}
	for _, ch := range channels {
		if !seen[ch.OwnerUserID] {
			seen[ch.OwnerUserID] = true
			ownerIDs = append(ownerIDs, ch.OwnerUserID)
		}
	}
	sharerNames := map[int]string{}
	if len(ownerIDs) > 0 {
		var users []model.User
		_ = model.DB.Select("id", "username", "display_name").Where("id IN ?", ownerIDs).Find(&users).Error
		for _, u := range users {
			name := u.DisplayName
			if name == "" {
				name = u.Username
			}
			sharerNames[u.Id] = anonymizeSharer(name)
		}
	}

	list := make([]*MarketplaceChannel, 0)
	for _, ch := range channels {
		serves := false
		for _, m := range ch.GetModels() {
			if m == modelName {
				serves = true
				break
			}
		}
		if !serves {
			continue
		}
		mult := ch.PriceMultiplier
		if mult <= 0 {
			mult = 1.0
		}
		entry := &MarketplaceChannel{
			ChannelID:          ch.Id,
			Name:               ch.Name,
			Type:               ch.Type,
			Models:             ch.GetModels(),
			Sharer:             sharerNames[ch.OwnerUserID],
			PriceMultiplier:    mult,
			TrustScore:         ch.TrustScore,
			VerificationStatus: ch.VerificationStatus,
		}
		if log, err := model.GetLatestVerificationLog(ch.Id); err == nil && log != nil {
			entry.EndpointOfficial = log.EndpointOfficial
		}
		list = append(list, entry)
	}

	// Sort: price ascending (free first) by default, or trust descending.
	sortParam := strings.ToLower(strings.TrimSpace(c.Query("sort")))
	if sortParam == "trust" {
		sort.Slice(list, func(i, j int) bool {
			if list[i].TrustScore != list[j].TrustScore {
				return list[i].TrustScore > list[j].TrustScore
			}
			return list[i].PriceMultiplier < list[j].PriceMultiplier
		})
	} else {
		sort.Slice(list, func(i, j int) bool {
			if list[i].PriceMultiplier != list[j].PriceMultiplier {
				return list[i].PriceMultiplier < list[j].PriceMultiplier
			}
			return list[i].TrustScore > list[j].TrustScore
		})
	}

	c.JSON(200, gin.H{
		"success": true,
		"data":    list,
	})
}
