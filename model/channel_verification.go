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
type ChannelVerificationLog struct {
	Id               int    `json:"id"`
	ChannelId        int    `json:"channel_id" gorm:"index"`
	ClaimedModel     string `json:"claimed_model"`
	EchoedModel      string `json:"echoed_model"`
	SystemFingerprint string `json:"system_fingerprint"`
	Verdict          string `json:"verdict" gorm:"type:varchar(32);index"` // verified | mismatch | error
	Detail           string `json:"detail" gorm:"type:text"`
	LatencyMs        int64  `json:"latency_ms"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index"`
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
