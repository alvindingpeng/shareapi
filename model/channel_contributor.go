package model

import (
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
	return nil
}
