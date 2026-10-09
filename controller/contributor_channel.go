package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Contributor-facing channel APIs (Phase 2 self-onboarding).
//
// Contributors authenticate as ordinary users (UserAuth, not AdminAuth) and
// can only ever touch their own channels (OwnerUserID == caller). Submitted
// channels enter ChannelStatusPendingReview and never take part in relay
// until an administrator approves them. Credentials are envelope-encrypted
// into the vault on write; API responses never carry key material.

// contributorChannelInput is the simplified submission form. Contributors
// cannot set type after creation, nor touch group/priority/weight.
type contributorChannelInput struct {
	Type    int     `json:"type"`
	Key     string  `json:"key"`
	Name    string  `json:"name"`
	BaseURL *string `json:"base_url"`
	Models  string  `json:"models"`
}

func contributorID(c *gin.Context) int {
	return c.GetInt("id")
}

// loadOwnChannel loads a channel and enforces contributor ownership.
// It returns nil (after writing the error response) when the channel does
// not exist or belongs to someone else; the two cases are deliberately
// indistinguishable.
func loadOwnChannel(c *gin.Context) *model.Channel {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, err)
		return nil
	}
	channel, err := model.GetContributorChannel(id, contributorID(c))
	if err != nil {
		common.ApiError(c, err)
		return nil
	}
	if channel == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return nil
	}
	return channel
}

func sanitizeContributorChannel(channel *model.Channel) {
	if channel != nil {
		clearChannelInfo(channel)
	}
}

// ContributorAddChannel handles POST /api/contributor/channels.
func ContributorAddChannel(c *gin.Context) {
	var input contributorChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if !constant.IsContributorAllowedType(input.Type) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not open for contributor onboarding"})
		return
	}
	if strings.TrimSpace(input.Key) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "key cannot be empty"})
		return
	}
	if strings.TrimSpace(input.Name) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "name cannot be empty"})
		return
	}
	channel := &model.Channel{
		Type:        input.Type,
		Key:         strings.TrimSpace(input.Key),
		Name:        strings.TrimSpace(input.Name),
		BaseURL:     input.BaseURL,
		Models:      strings.TrimSpace(input.Models),
		Group:       "default",
		Status:      common.ChannelStatusPendingReview,
		OwnerUserID: contributorID(c),
		CreatedTime: common.GetTimestamp(),
	}
	if err := validateChannel(channel, true); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.BatchInsertChannels([]model.Channel{*channel}); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "submitted for review"})
}

// ContributorListChannels handles GET /api/contributor/channels.
func ContributorListChannels(c *gin.Context) {
	channels, err := model.GetContributorChannels(contributorID(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, ch := range channels {
		sanitizeContributorChannel(ch)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channels})
}

// ContributorGetChannel handles GET /api/contributor/channels/:id.
func ContributorGetChannel(c *gin.Context) {
	channel := loadOwnChannel(c)
	if channel == nil {
		return
	}
	sanitizeContributorChannel(channel)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channel})
}

// ContributorUpdateChannel handles PUT /api/contributor/channels/:id.
// Any change re-queues an enabled channel for review.
func ContributorUpdateChannel(c *gin.Context) {
	channel := loadOwnChannel(c)
	if channel == nil {
		return
	}
	var input contributorChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if input.Type != 0 && input.Type != channel.Type {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type cannot be changed"})
		return
	}
	changed := false
	if input.Name != "" && input.Name != channel.Name {
		channel.Name = strings.TrimSpace(input.Name)
		changed = true
	}
	if input.BaseURL != nil && (channel.BaseURL == nil || *input.BaseURL != *channel.BaseURL) {
		channel.BaseURL = input.BaseURL
		changed = true
	}
	if input.Models != "" && input.Models != channel.Models {
		channel.Models = strings.TrimSpace(input.Models)
		changed = true
	}
	keyChanged := strings.TrimSpace(input.Key) != ""
	if keyChanged {
		if err := channel.RotateKey(strings.TrimSpace(input.Key)); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		changed = true
	}
	if !changed {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "no changes"})
		return
	}
	// Re-queue for review: the new config/credential is unverified.
	// RotateKey already persisted the credential columns; persist the rest
	// plus the status transition in one update.
	channel.Status = common.ChannelStatusPendingReview
	updates := map[string]any{
		"name":     channel.Name,
		"base_url": channel.BaseURL,
		"models":   channel.Models,
		"status":   channel.Status,
	}
	if err := model.UpdateContributorChannel(channel.Id, channel.OwnerUserID, updates); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "updated, re-queued for review"})
}

// ContributorDeleteChannel handles DELETE /api/contributor/channels/:id.
func ContributorDeleteChannel(c *gin.Context) {
	channel := loadOwnChannel(c)
	if channel == nil {
		return
	}
	if err := model.DeleteContributorChannel(channel.Id, channel.OwnerUserID); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// ContributorRotateKey handles POST /api/contributor/channels/:id/rotate-key.
func ContributorRotateKey(c *gin.Context) {
	channel := loadOwnChannel(c)
	if channel == nil {
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Key) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "key cannot be empty"})
		return
	}
	if err := channel.RotateKey(strings.TrimSpace(req.Key)); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// A new credential is unverified: re-queue for review.
	if err := model.UpdateContributorChannel(channel.Id, channel.OwnerUserID,
		map[string]any{"status": common.ChannelStatusPendingReview}); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "key rotated, re-queued for review"})
}

// reviewTransition moves a channel out of pending review. Only the
// PendingReview -> Enabled/ManuallyDisabled transitions are legal here;
// anything else is rejected so approved channels cannot be silently
// re-reviewed through this path.
func reviewTransition(c *gin.Context, targetStatus int, action string) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, err)
		return
	}
	channel, err := model.GetChannelById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channel == nil || channel.Status != common.ChannelStatusPendingReview {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel is not pending review"})
		return
	}
	// NB: UpdateChannelStatus is cache-first and silently no-ops for channels
	// that were never cached (pending review is excluded from the relay
	// cache by design), so review uses the dedicated ReviewChannel instead.
	changed, err := model.ReviewChannel(id, targetStatus, "review_"+action)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if changed {
		model.InitChannelCache()
		if targetStatus != common.ChannelStatusEnabled {
			closeActiveChannelWebSockets([]int{id})
		}
	}
	recordManageAudit(c, "channel.review_"+action, map[string]any{
		"id":       id,
		"owner_id": channel.OwnerUserID,
		"changed":  changed,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": changed})
}

// ApproveChannel handles POST /api/channel/:id/approve (admin review).
func ApproveChannel(c *gin.Context) {
	reviewTransition(c, common.ChannelStatusEnabled, "approve")
}

// RejectChannel handles POST /api/channel/:id/reject (admin review).
func RejectChannel(c *gin.Context) {
	reviewTransition(c, common.ChannelStatusManuallyDisabled, "reject")
}
