package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// Fingerprint probe (Phase 3 anti-dilution).
//
// A contributor claims their channel serves model X. The probe sends a fixed
// prompt directly to the upstream (bypassing relay/billing) and checks the
// echoed `model` field. A mismatch means the channel is "diluted": serving a
// cheaper/different model than advertised.

// verifyProbePrompt is fixed so responses are comparable across runs.
const verifyProbePrompt = "Reply with exactly: VERIFY-OK"

// FingerprintVerdict is the probe outcome.
type FingerprintVerdict string

const (
	FingerprintVerified FingerprintVerdict = "verified"
	FingerprintMismatch FingerprintVerdict = "mismatch"
	FingerprintError    FingerprintVerdict = "error"
)

// FingerprintResult is one probe outcome.
type FingerprintResult struct {
	ChannelID         int
	ClaimedModel      string
	EchoedModel       string
	SystemFingerprint string
	Verdict           FingerprintVerdict
	Detail            string
	LatencyMs         int64
	PromptTokens      int
	CompletionTokens  int
}

// ProbeChannelFingerprint runs one fingerprint probe against a channel.
// It returns a result with Verdict=error (never a Go error) when the probe
// itself fails, so transient upstream issues don't look like dilution.
func ProbeChannelFingerprint(ctx context.Context, channel *model.Channel) *FingerprintResult {
	result := &FingerprintResult{ChannelID: channel.Id}
	models := channel.GetModels()
	if len(models) == 0 {
		result.Verdict = FingerprintError
		result.Detail = "channel has no models configured"
		return result
	}
	result.ClaimedModel = strings.TrimSpace(models[0])

	key, err := channel.DecryptedKey()
	if err != nil {
		result.Verdict = FingerprintError
		result.Detail = fmt.Sprintf("decrypt key: %v", err)
		return result
	}
	baseURL := strings.TrimSuffix(channel.GetBaseURL(), "/")
	if baseURL == "" {
		result.Verdict = FingerprintError
		result.Detail = "channel has no base URL"
		return result
	}

	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":%q}],"max_tokens":16,"temperature":0}`,
		result.ClaimedModel, verifyProbePrompt)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewBufferString(body))
	if err != nil {
		result.Verdict = FingerprintError
		result.Detail = fmt.Sprintf("build request: %v", err)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	result.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		result.Verdict = FingerprintError
		result.Detail = fmt.Sprintf("upstream request: %v", err)
		return result
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		result.Verdict = FingerprintError
		result.Detail = fmt.Sprintf("read response: %v", err)
		return result
	}
	if resp.StatusCode != http.StatusOK {
		result.Verdict = FingerprintError
		result.Detail = fmt.Sprintf("upstream status %d: %s", resp.StatusCode, truncateForLog(respBody, 200))
		return result
	}

	var parsed struct {
		Model             string `json:"model"`
		SystemFingerprint string `json:"system_fingerprint"`
		Usage             struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := common.Unmarshal(respBody, &parsed); err != nil {
		result.Verdict = FingerprintError
		result.Detail = fmt.Sprintf("parse response: %v", err)
		return result
	}
	result.EchoedModel = parsed.Model
	result.SystemFingerprint = parsed.SystemFingerprint
	result.PromptTokens = parsed.Usage.PromptTokens
	result.CompletionTokens = parsed.Usage.CompletionTokens

	if result.EchoedModel == "" {
		result.Verdict = FingerprintError
		result.Detail = "upstream response has no model field"
		return result
	}
	if modelsMatch(result.ClaimedModel, result.EchoedModel) {
		result.Verdict = FingerprintVerified
	} else {
		result.Verdict = FingerprintMismatch
		result.Detail = fmt.Sprintf("claimed %q but upstream served %q", result.ClaimedModel, result.EchoedModel)
	}
	return result
}

// modelsMatch compares the claimed model against the echoed one.
// Upstreams often return dated variants (gpt-4o-2024-05-13 for gpt-4o), so
// the date suffix is stripped before comparison. Anything else must match
// exactly: "gpt-4o" vs "gpt-4o-mini" is dilution, not a variant.
func modelsMatch(claimed, echoed string) bool {
	c := strings.ToLower(strings.TrimSpace(claimed))
	e := strings.ToLower(strings.TrimSpace(echoed))
	if c == "" || e == "" {
		return false
	}
	if c == e {
		return true
	}
	// Strip date suffix: gpt-4o-2024-05-13 -> gpt-4o
	if idx := strings.Index(e, "-20"); idx > 0 && idx+5 < len(e) {
		if base := e[:idx]; base == c {
			return true
		}
	}
	return false
}

func truncateForLog(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// ModelVerifySummary is the per-run outcome recorded on the system task.
type ModelVerifySummary struct {
	Probed     int `json:"probed"`
	Verified   int `json:"verified"`
	Mismatched int `json:"mismatched"`
	Errors     int `json:"errors"`
	Disabled   int `json:"disabled"`
}

// verifyDisableThreshold is the number of consecutive mismatches before a
// channel is auto-disabled. Env override for ops tuning.
func verifyDisableThreshold() int {
	if n := common.GetEnvOrDefault("MODEL_VERIFY_DISABLE_THRESHOLD", 3); n > 0 {
		return n
	}
	return 3
}

// RunModelVerifyTask probes every enabled contributor channel and records the
// outcome. It is idempotent: re-running only appends new log rows.
func RunModelVerifyTask(ctx context.Context) (*ModelVerifySummary, error) {
	summary := &ModelVerifySummary{}
	var channels []*model.Channel
	if err := model.DB.Where("status = ? AND owner_user_id > 0", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("list contributor channels: %w", err)
	}
	for _, ch := range channels {
		if ctx.Err() != nil {
			break
		}
		summary.Probed++
		result := ProbeChannelFingerprint(ctx, ch)

		// P3-6: calculate probe cost in quota units for platform accounting.
		costQuota := calculateProbeCost(result.ClaimedModel, result.PromptTokens, result.CompletionTokens)

		_ = model.RecordVerificationLog(&model.ChannelVerificationLog{
			ChannelId:         ch.Id,
			ClaimedModel:      result.ClaimedModel,
			EchoedModel:       result.EchoedModel,
			SystemFingerprint: result.SystemFingerprint,
			Verdict:           string(result.Verdict),
			Detail:            result.Detail,
			LatencyMs:         result.LatencyMs,
			PromptTokens:      result.PromptTokens,
			CompletionTokens:  result.CompletionTokens,
			CostQuota:         costQuota,
		})

		switch result.Verdict {
		case FingerprintVerified:
			summary.Verified++
			_ = model.UpdateChannelVerification(ch.Id, model.VerificationVerified, 0)
		case FingerprintMismatch:
			summary.Mismatched++
			fails := ch.VerificationFails + 1
			_ = model.UpdateChannelVerification(ch.Id, model.VerificationSuspicious, fails)
			if fails >= verifyDisableThreshold() {
				model.CacheUpdateChannelStatus(ch.Id, common.ChannelStatusManuallyDisabled)
				summary.Disabled++
				_ = model.RecordVerificationLog(&model.ChannelVerificationLog{
					ChannelId: ch.Id,
					Verdict:   string(FingerprintMismatch),
					Detail:    fmt.Sprintf("auto-disabled after %d consecutive mismatches", fails),
				})
			}
		default:
			summary.Errors++
			// Transient probe errors don't change verification state.
		}
	}
	return summary, nil
}

// calculateProbeCost computes the quota cost of a probe for platform
// accounting (P3-6). Uses the same ratio logic as billing: prompt tokens at
// model ratio, completion tokens at model ratio * completion ratio.
func calculateProbeCost(modelName string, promptTokens, completionTokens int) int64 {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}
	modelRatio, _, _ := ratio_setting.GetModelRatio(modelName)
	completionRatio := ratio_setting.GetCompletionRatio(modelName)
	cost := float64(promptTokens)*modelRatio + float64(completionTokens)*modelRatio*completionRatio
	return int64(cost)
}
