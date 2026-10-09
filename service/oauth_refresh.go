package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// OAuth token auto-refresh (Phase 6).
//
// Subscription credentials (e.g. Codex) use short-lived access tokens. This
// task refreshes tokens approaching expiry so contributor channels don't go
// dark. Refresh failures are logged; the channel keeps serving until the
// token actually expires, at which point health monitoring (P6-4) flags it.

// oauthRefreshWindow is how far ahead of expiry we proactively refresh.
const oauthRefreshWindow = 30 * time.Minute

// codexTokenURL is the ChatGPT OAuth token endpoint for Codex refresh.
const codexTokenURL = "https://auth.openai.com/oauth/token"

// OAuthRefreshSummary is the per-run outcome.
type OAuthRefreshSummary struct {
	Checked   int `json:"checked"`
	Refreshed int `json:"refreshed"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

// RunOAuthRefreshTask refreshes OAuth tokens expiring within the window.
// Already-expired tokens are also attempted; persistent failures mark the
// channel suspicious (P6-4 health monitoring).
func RunOAuthRefreshTask(ctx context.Context) (*OAuthRefreshSummary, error) {
	summary := &OAuthRefreshSummary{}
	cutoff := common.GetTimestamp() + int64(oauthRefreshWindow.Seconds())

	var channels []*model.Channel
	if err := model.DB.Where("status = ? AND type = ? AND oauth_expires_at > 0 AND oauth_expires_at <= ?",
		common.ChannelStatusEnabled, constant.ChannelTypeCodex, cutoff).
		Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("list expiring oauth channels: %w", err)
	}

	now := common.GetTimestamp()
	for _, ch := range channels {
		if ctx.Err() != nil {
			break
		}
		summary.Checked++
		alreadyExpired := ch.OAuthExpiresAt <= now
		if err := refreshChannelOAuth(ctx, ch); err != nil {
			summary.Failed++
			common.SysLog(fmt.Sprintf("oauth refresh failed for channel %d: %v", ch.Id, err))
			// P6-4: expired token that won't refresh = dead credential.
			if alreadyExpired {
				_ = model.UpdateChannelVerification(ch.Id, model.VerificationSuspicious, ch.VerificationFails+1)
			}
			continue
		}
		summary.Refreshed++
	}
	return summary, nil
}

// refreshChannelOAuth performs one token refresh for a Codex channel.
func refreshChannelOAuth(ctx context.Context, ch *model.Channel) error {
	keyJSON, err := ch.DecryptedKey()
	if err != nil {
		return fmt.Errorf("decrypt key: %w", err)
	}

	var oauthKey struct {
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		AccountID    string `json:"account_id"`
		Email        string `json:"email"`
		Type         string `json:"type"`
	}
	if err := common.Unmarshal([]byte(keyJSON), &oauthKey); err != nil {
		return fmt.Errorf("parse oauth key: %w", err)
	}
	if oauthKey.RefreshToken == "" {
		return fmt.Errorf("no refresh_token in oauth key")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", oauthKey.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexTokenURL,
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("read refresh response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh status %d: %s", resp.StatusCode, truncateForLog(body, 200))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := common.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("parse refresh response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return fmt.Errorf("refresh response has no access_token")
	}

	// Preserve fields the refresh response doesn't return.
	newRefresh := tokenResp.RefreshToken
	if newRefresh == "" {
		newRefresh = oauthKey.RefreshToken
	}
	updated := map[string]string{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": newRefresh,
		"account_id":    oauthKey.AccountID,
		"email":         oauthKey.Email,
		"type":          oauthKey.Type,
		"last_refresh":  time.Now().UTC().Format(time.RFC3339),
	}
	if tokenResp.IDToken != "" {
		updated["id_token"] = tokenResp.IDToken
	}
	if tokenResp.ExpiresIn > 0 {
		updated["expired"] = fmt.Sprintf("%d", time.Now().Unix()+tokenResp.ExpiresIn)
	}
	newKeyJSON, err := common.Marshal(updated)
	if err != nil {
		return fmt.Errorf("marshal updated key: %w", err)
	}

	if err := ch.RotateKey(string(newKeyJSON)); err != nil {
		return fmt.Errorf("store refreshed key: %w", err)
	}

	expiresAt := int64(0)
	if tokenResp.ExpiresIn > 0 {
		expiresAt = time.Now().Unix() + tokenResp.ExpiresIn
	}
	_ = model.UpdateOAuthLifecycle(ch.Id, expiresAt)
	return nil
}
