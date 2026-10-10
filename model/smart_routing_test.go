package model

import (
	"testing"
)

func TestSelectSmartChannel(t *testing.T) {
	mkChannel := func(id, owner, trust int, mult float64, status int) *Channel {
		return &Channel{
			Id:                 id,
			OwnerUserID:          owner,
			TrustScore:           trust,
			PriceMultiplier:      mult,
			VerificationStatus:   status,
		}
	}

	// Fewer than 2 contributor channels: no smart routing.
	single := []*Channel{
		mkChannel(1, 1, 80, 1.0, VerificationVerified),
		mkChannel(2, 0, 50, 1.0, VerificationVerified),
	}
	if got := SelectSmartChannel(single, 1.0, RoutingBalanced); got != nil {
		t.Errorf("expected nil for single contributor channel, got %v", got.Id)
	}

	// Cheapest: picks lowest multiplier.
	cheap := []*Channel{
		mkChannel(1, 1, 90, 1.2, VerificationVerified),
		mkChannel(2, 2, 70, 0.8, VerificationVerified),
	}
	if got := SelectSmartChannel(cheap, 1.0, RoutingCheapest); got == nil || got.Id != 2 {
		t.Errorf("cheapest: expected channel 2, got %v", got)
	}

	// Trust first: picks highest trust, skips suspicious.
	trust := []*Channel{
		mkChannel(1, 1, 95, 1.5, VerificationVerified),
		mkChannel(2, 2, 60, 0.7, VerificationSuspicious),
		mkChannel(3, 3, 80, 1.0, VerificationVerified),
	}
	if got := SelectSmartChannel(trust, 1.0, RoutingTrustFirst); got == nil || got.Id != 1 {
		t.Errorf("trust_first: expected channel 1, got %v", got)
	}

	// Balanced: weights both.
	balanced := []*Channel{
		mkChannel(1, 1, 100, 2.0, VerificationVerified), // high trust, high price
		mkChannel(2, 2, 60, 0.6, VerificationVerified),  // low trust, low price
	}
	got := SelectSmartChannel(balanced, 1.0, RoutingBalanced)
	if got == nil {
		t.Errorf("balanced: expected a channel, got nil")
	}
}
