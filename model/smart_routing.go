package model

import (
	"sort"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// smartRoutingEnabled reports whether smart routing is active.
func smartRoutingEnabled() bool {
	return operation_setting.GetSmartRoutingSetting().Enabled
}

// smartRoutingStrategy returns the active routing strategy.
func smartRoutingStrategy() string {
	s := operation_setting.GetSmartRoutingSetting().Strategy
	switch s {
	case RoutingCheapest, RoutingTrustFirst, RoutingBalanced:
		return s
	default:
		return RoutingBalanced
	}
}

// getBasePriceForModel returns the platform base price for a model.
func getBasePriceForModel(modelName string) float64 {
	for _, p := range GetPricing() {
		if p.ModelName == modelName {
			return p.ModelPrice
		}
	}
	return 0
}

// Smart routing strategies (P8-2).
const (
	RoutingCheapest   = "cheapest"
	RoutingTrustFirst = "trust_first"
	RoutingBalanced   = "balanced"
)

// SelectSmartChannel picks the best channel from candidates using the given
// strategy. Returns nil if there are fewer than 2 contributor channels
// (smart routing doesn't apply).
//
// Scoring:
//   - cheapest: lowest effective price (base_price * price_multiplier)
//   - trust_first: highest trust score (skips suspicious channels)
//   - balanced: 50/50 weighted combination (default)
func SelectSmartChannel(candidates []*Channel, basePrice float64, strategy string) *Channel {
	if len(candidates) < 2 {
		return nil
	}
	contribCount := 0
	for _, ch := range candidates {
		if ch.OwnerUserID > 0 {
			contribCount++
		}
	}
	if contribCount < 2 {
		return nil
	}

	type scored struct {
		ch    *Channel
		score float64
	}
	var list []scored
	for _, ch := range candidates {
		if strategy != RoutingCheapest && ch.VerificationStatus == VerificationSuspicious {
			continue
		}
		list = append(list, scored{ch, scoreChannel(ch, basePrice, strategy)})
	}
	if len(list) == 0 {
		return nil
	}
	sort.Slice(list, func(i, j int) bool { return list[i].score > list[j].score })
	return list[0].ch
}

func scoreChannel(ch *Channel, basePrice float64, strategy string) float64 {
	mult := ch.PriceMultiplier
	if mult <= 0 {
		mult = 1.0
	}
	effectivePrice := basePrice * mult
	if effectivePrice <= 0 {
		effectivePrice = basePrice
		if effectivePrice <= 0 {
			effectivePrice = 1.0
		}
	}

	trust := float64(ch.TrustScore) / 100.0
	if trust < 0 {
		trust = 0
	}
	if trust > 1 {
		trust = 1
	}

	priceScore := 1.0 - (effectivePrice/basePrice-1.0)/9.0
	if priceScore < 0 {
		priceScore = 0
	}
	if priceScore > 1 {
		priceScore = 1
	}

	switch strategy {
	case RoutingCheapest:
		return priceScore
	case RoutingTrustFirst:
		return trust
	default:
		return trust*0.5 + priceScore*0.5
	}
}
