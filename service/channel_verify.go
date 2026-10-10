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
	// P7-4: base-URL certification.
	BaseURLHost      string
	EndpointOfficial bool // base URL terminates at an official endpoint
	EndpointKnown    bool // channel type has a whitelist entry
	// P7-5: L4 behavioral fingerprint (identity probe).
	IdentityResponse   string
	IdentityConsistent bool // response mentions the claimed model family
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

	// P7-4: base-URL certification check. A channel claiming an official
	// model must terminate at the official endpoint; otherwise it cannot
	// be certified and is a shell risk.
	result.BaseURLHost = extractHost(baseURL)
	official, known := IsOfficialEndpoint(channel.Type, baseURL)
	result.EndpointOfficial = official
	result.EndpointKnown = known
	if known && !official {
		result.Verdict = FingerprintMismatch
		result.Detail = fmt.Sprintf("base URL host %q is not an official endpoint for this channel type (shell risk)", result.BaseURLHost)
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
		return result
	}

	// P7-5: L4 behavioral fingerprint. Ask "who are you?" and check the
	// response mentions the claimed model family. A shell that swaps in a
	// different model will answer with the wrong identity.
	identityResp, identityTokens := probeIdentity(ctx, baseURL, key, result.ClaimedModel)
	result.IdentityResponse = identityResp
	result.PromptTokens += identityTokens.Prompt
	result.CompletionTokens += identityTokens.Completion
	if identityResp == "" {
		// Identity probe failed; don't fail the whole verification on it.
		result.IdentityConsistent = true
		return result
	}
	result.IdentityConsistent = identityMatchesModel(identityResp, result.ClaimedModel)
	if !result.IdentityConsistent {
		result.Verdict = FingerprintMismatch
		result.Detail = fmt.Sprintf("identity probe: claimed %q but model identifies as %q",
			result.ClaimedModel, truncateForLog([]byte(identityResp), 120))
	}
	return result
}

// identityPrompt asks the model to identify itself in one sentence.
const identityPrompt = "Who are you? Answer in one short sentence."

// modelFamilyKeywords maps model name patterns to expected identity keywords.
var modelFamilyKeywords = []struct {
	pattern  string
	keywords []string
}{
	{"gpt", []string{"openai", "gpt"}},
	{"o1", []string{"openai"}},
	{"o3", []string{"openai"}},
	{"claude", []string{"anthropic", "claude"}},
	{"gemini", []string{"google", "gemini"}},
	{"deepseek", []string{"deepseek"}},
	{"llama", []string{"meta", "llama"}},
	{"mistral", []string{"mistral"}},
	{"mixtral", []string{"mistral"}},
	{"grok", []string{"xai", "grok"}},
	{"qwen", []string{"alibaba", "qwen", "tongyi"}},
	{"glm", []string{"zhipu", "glm"}},
	{"moonshot", []string{"moonshot", "kimi"}},
	{"kimi", []string{"moonshot", "kimi"}},
	{"doubao", []string{"bytedance", "doubao"}},
	{"ernie", []string{"baidu", "ernie", "wenxin"}},
	{"spark", []string{"xfyun", "spark", "xunfei"}},
	{"hunyuan", []string{"tencent", "hunyuan"}},
	{"minimax", []string{"minimax"}},
	{"yi-", []string{"lingyiwanwu", "yi"}},
	{"phi", []string{"microsoft", "phi"}},
	{"gemma", []string{"google", "gemma"}},
	{"command", []string{"cohere", "command"}},
	{"palm", []string{"google", "palm"}},
	{"falcon", []string{"tii", "falcon"}},
	{"vicuna", []string{"vicuna"}},
	{"wizardlm", []string{"wizardlm"}},
	{"codellama", []string{"meta", "codellama"}},
	{"starling", []string{"starling"}},
	{"zephyr", []string{"zephyr"}},
	{"solar", []string{"upstage", "solar"}},
}

// identityMatchesModel checks if the identity response mentions the expected
// model family keywords for the claimed model.
func identityMatchesModel(response, claimedModel string) bool {
	resp := strings.ToLower(response)
	claimed := strings.ToLower(claimedModel)
	for _, mf := range modelFamilyKeywords {
		if strings.Contains(claimed, mf.pattern) {
			for _, kw := range mf.keywords {
				if strings.Contains(resp, kw) {
					return true
				}
			}
			// Claimed model matched a family but response mentions none of
			// its keywords: inconsistent.
			return false
		}
	}
	// Unknown model family: can't judge, treat as consistent.
	return true
}

type tokenPair struct {
	Prompt     int
	Completion int
}

// probeIdentity sends the identity question to the channel and returns the
// model's self-identification text plus token usage.
func probeIdentity(ctx context.Context, baseURL, key, claimedModel string) (string, tokenPair) {
	var tokens tokenPair
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":%q}],"max_tokens":64,"temperature":0}`,
		claimedModel, identityPrompt)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewBufferString(body))
	if err != nil {
		return "", tokens
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", tokens
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", tokens
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := common.Unmarshal(respBody, &parsed); err != nil {
		return "", tokens
	}
	tokens.Prompt = parsed.Usage.PromptTokens
	tokens.Completion = parsed.Usage.CompletionTokens
	if len(parsed.Choices) == 0 {
		return "", tokens
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), tokens
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
			ChannelId:          ch.Id,
			ClaimedModel:       result.ClaimedModel,
			EchoedModel:        result.EchoedModel,
			SystemFingerprint:  result.SystemFingerprint,
			Verdict:            string(result.Verdict),
			Detail:             result.Detail,
			LatencyMs:          result.LatencyMs,
			PromptTokens:       result.PromptTokens,
			CompletionTokens:   result.CompletionTokens,
			CostQuota:          costQuota,
			BaseURLHost:        result.BaseURLHost,
			EndpointOfficial:   result.EndpointOfficial,
			IdentityResponse:   result.IdentityResponse,
			IdentityConsistent: result.IdentityConsistent,
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
		// P7-6: recalculate trust score after each probe.
		_ = model.UpdateChannelTrustScore(ch.Id)
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
