package model

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

// OAuth-based channel types whose keys are JSON credential blobs (not raw
// API keys). Phase 6 tracks their token lifecycle for auto-refresh.
var oauthChannelTypes = map[int]bool{
	constant.ChannelTypeCodex: true,
}

// IsOAuthChannelType reports whether the channel type uses OAuth subscription
// credentials instead of a plain API key.
func IsOAuthChannelType(channelType int) bool {
	return oauthChannelTypes[channelType]
}

// ParseOAuthExpiry extracts the access token expiry Unix timestamp from a
// Codex-style OAuth JSON key. Returns 0 when the expiry cannot be determined.
func ParseOAuthExpiry(oauthJSON string) int64 {
	var parsed struct {
		Expired string `json:"expired"`
	}
	if err := common.Unmarshal([]byte(oauthJSON), &parsed); err != nil {
		return 0
	}
	s := strings.TrimSpace(parsed.Expired)
	if s == "" {
		return 0
	}
	// Try Unix timestamp first.
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		// Heuristic: 10-digit = seconds, 13-digit = milliseconds.
		if ts > 1e12 {
			ts /= 1000
		}
		if ts > time.Now().Unix()-86400*365 {
			return ts
		}
		return 0
	}
	// Try RFC3339.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	return 0
}

// UpdateOAuthLifecycle records the token expiry and last refresh time for a
// channel. Used by channel creation and the OAuth refresh task.
func UpdateOAuthLifecycle(channelID int, expiresAt int64) error {
	updates := map[string]any{
		"oauth_expires_at":   expiresAt,
		"oauth_last_refresh": common.GetTimestamp(),
	}
	return DB.Model(&Channel{}).Where("id = ?", channelID).Updates(updates).Error
}
