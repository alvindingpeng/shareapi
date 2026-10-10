package model

import (
	"github.com/QuantumNous/new-api/common"
)

// Channel verification statuses (Phase 3 anti-dilution).
const (
	VerificationUnverified = 0 // never probed
	VerificationVerified   = 1 // last probe: model echo matches
	VerificationSuspicious = 2 // last probe: model echo mismatch or fingerprint drift
)

// ChannelVerificationLog records one fingerprint probe run against a channel.
// Table: channel_verification_logs.
// P3-6: probe token usage and cost are recorded so platform ops can account
// for verification overhead as a platform cost (not billed to contributors).
type ChannelVerificationLog struct {
	Id                int    `json:"id"`
	ChannelId         int    `json:"channel_id" gorm:"index"`
	ClaimedModel      string `json:"claimed_model"`
	EchoedModel       string `json:"echoed_model"`
	SystemFingerprint string `json:"system_fingerprint"`
	Verdict           string `json:"verdict" gorm:"type:varchar(32);index"` // verified | mismatch | error
	Detail            string `json:"detail" gorm:"type:text"`
	LatencyMs         int64  `json:"latency_ms"`
	PromptTokens      int    `json:"prompt_tokens"`
	CompletionTokens  int    `json:"completion_tokens"`
	CostQuota         int64  `json:"cost_quota"` // quota units consumed by the probe
	// P7-4: base-URL certification.
	BaseURLHost      string `json:"base_url_host"`
	EndpointOfficial bool   `json:"endpoint_official"`
	// P7-5: L4 behavioral fingerprint.
	IdentityResponse   string `json:"identity_response" gorm:"type:text"`
	IdentityConsistent bool   `json:"identity_consistent"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;index"`
}

// RecordVerificationLog persists a probe result.
func RecordVerificationLog(log *ChannelVerificationLog) error {
	log.CreatedAt = common.GetTimestamp()
	return DB.Create(log).Error
}

// GetLatestVerificationLog returns the most recent probe result for a channel.
func GetLatestVerificationLog(channelID int) (*ChannelVerificationLog, error) {
	var log ChannelVerificationLog
	err := DB.Where("channel_id = ?", channelID).Order("id DESC").First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// UpdateChannelVerification updates the channel's verification state after a
// probe. Consecutive mismatches are counted for the auto-disable threshold.
func UpdateChannelVerification(channelID int, status int, fails int) error {
	return DB.Model(&Channel{}).Where("id = ?", channelID).Updates(map[string]any{
		"verification_status": status,
		"last_verified_at":    common.GetTimestamp(),
		"verification_fails":  fails,
	}).Error
}

// CalculateTrustScore computes a 0-100 trust score for a channel (P7-6).
// Components:
//   - Base: 50
//   - Official endpoint (P7-4): +20
//   - Identity consistent (P7-5): +10
//   - Recent verification success rate (last 10 probes): 0 to +20
//   - Consecutive failures: -10 each (max -30)
//
// The score is clamped to [0, 100].
func CalculateTrustScore(channelID int) int {
	score := 50

	// Latest probe: endpoint certification and identity.
	latest, err := GetLatestVerificationLog(channelID)
	if err == nil && latest != nil {
		if latest.EndpointOfficial {
			score += 20
		}
		if latest.IdentityConsistent {
			score += 10
		}
	}

	// Recent success rate from last 10 probes.
	var logs []ChannelVerificationLog
	if err := DB.Where("channel_id = ?", channelID).
		Order("id DESC").Limit(10).Find(&logs).Error; err == nil && len(logs) > 0 {
		verified := 0
		for _, l := range logs {
			if l.Verdict == "verified" {
				verified++
			}
		}
		score += verified * 20 / len(logs)
	}

	// Consecutive failure penalty.
	var ch Channel
	if err := DB.Select("verification_fails").Where("id = ?", channelID).First(&ch).Error; err == nil {
		penalty := ch.VerificationFails * 10
		if penalty > 30 {
			penalty = 30
		}
		score -= penalty
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// UpdateChannelTrustScore recalculates and persists the trust score.
func UpdateChannelTrustScore(channelID int) error {
	score := CalculateTrustScore(channelID)
	return DB.Model(&Channel{}).Where("id = ?", channelID).
		Update("trust_score", score).Error
}

// GetProbeCostSummary returns aggregate probe costs for platform accounting.
// Groups by day for the given range. All costs are platform-borne (P3-6).
func GetProbeCostSummary(startAt, endAt int64) (map[string]any, error) {
	var result struct {
		TotalProbes int64 `json:"total_probes"`
		TotalCost   int64 `json:"total_cost"`
	}
	err := DB.Model(&ChannelVerificationLog{}).
		Select("COUNT(*) as total_probes, COALESCE(SUM(cost_quota), 0) as total_cost").
		Where("created_at >= ? AND created_at <= ?", startAt, endAt).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total_probes": result.TotalProbes,
		"total_cost":   result.TotalCost,
		"start_at":     startAt,
		"end_at":       endAt,
	}, nil
}
