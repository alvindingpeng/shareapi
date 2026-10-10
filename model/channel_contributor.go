package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Contributor-scoped channel access (Phase 2 self-onboarding).
//
// Every function takes the caller's user ID and refuses to touch channels
// owned by someone else. Lookups return (nil, nil) when the channel does not
// exist or is not owned by the caller, so the two cases are indistinguishable
// to the API layer.

// GetContributorChannel loads one channel owned by ownerUserID, without the
// key column (key material never leaves the vault through this path).
func GetContributorChannel(id int, ownerUserID int) (*Channel, error) {
	channel := &Channel{}
	err := DB.Omit("key").Where("id = ? AND owner_user_id = ?", id, ownerUserID).First(channel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return channel, nil
}

// GetContributorChannels lists all channels owned by ownerUserID, newest
// first, without key material.
func GetContributorChannels(ownerUserID int) ([]*Channel, error) {
	var channels []*Channel
	err := DB.Omit("key").Where("owner_user_id = ?", ownerUserID).Order("id DESC").Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

// UpdateContributorChannel applies updates to a channel owned by ownerUserID.
// The caller must have validated the fields; ownership is re-checked here.
func UpdateContributorChannel(id int, ownerUserID int, updates map[string]any) error {
	result := DB.Model(&Channel{}).Where("id = ? AND owner_user_id = ?", id, ownerUserID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteContributorChannel deletes a channel owned by ownerUserID.
func DeleteContributorChannel(id int, ownerUserID int) error {
	result := DB.Where("id = ? AND owner_user_id = ?", id, ownerUserID).Delete(&Channel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	// F1 fix: remove stale ability rows so the deleted channel can no longer
	// take relay traffic, then refresh the channel cache to drop it from
	// memory (including any decrypted key material).
	if err := DB.Where("channel_id = ?", id).Delete(&Ability{}).Error; err != nil {
		return err
	}
	InitChannelCache()
	return nil
}

// ReviewChannel transitions a pending-review channel to targetStatus
// (Enabled on approve, ManuallyDisabled on reject). Unlike
// UpdateChannelStatus, it works on channels that were never in the relay
// cache (pending channels are excluded from cache by design). It returns
// false when the channel is not pending review, so illegal transitions are
// rejected without touching data.
func ReviewChannel(id int, targetStatus int, reason string) (bool, error) {
	pollingLock := GetChannelPollingLock(id)
	pollingLock.Lock()
	defer pollingLock.Unlock()

	channel, err := GetChannelById(id, true)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	if channel.Status != common.ChannelStatusPendingReview {
		return false, nil
	}
	if channel.ChannelInfo.IsMultiKey {
		beforeStatus := channel.Status
		handlerMultiKeyUpdate(channel, "", targetStatus, reason)
		if beforeStatus == channel.Status {
			return false, nil
		}
	} else {
		info := channel.GetOtherInfo()
		info["status_reason"] = reason
		info["status_time"] = common.GetTimestamp()
		channel.SetOtherInfo(info)
		channel.Status = targetStatus
	}
	if err := channel.saveStatusState(); err != nil {
		return false, err
	}
	if err := UpdateAbilityStatus(id, targetStatus == common.ChannelStatusEnabled); err != nil {
		common.SysLog(fmt.Sprintf("review channel: failed to update ability status: channel_id=%d, error=%v", id, err))
	}
	return true, nil
}
