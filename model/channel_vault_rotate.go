package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/vault"
)

// RotateChannelKEK re-encrypts every vaulted channel credential from oldKEK to
// newKEK and stamps the rows with the new KEK's ID.
//
// It is idempotent: rows already carrying the new KEK's ID are skipped, so a
// crashed or interrupted rotation can simply be re-run. Rows that fail to
// decrypt with oldKEK abort the whole rotation without being modified, so a
// wrong "old" KEK can never corrupt data.
//
// Operational contract (see the KEK management doc): stop the app servers,
// run this, point VAULT_KEK at the new value everywhere, then start the app
// servers again. Never run it while an app server is live: the in-memory
// channel cache would keep decrypting with the old KEK.
func RotateChannelKEK(oldKEK, newKEK []byte) (rotated int, err error) {
	if len(oldKEK) != 32 || len(newKEK) != 32 {
		return 0, errors.New("vault: both KEKs must be 32 bytes")
	}
	newID := vault.KEKID(newKEK)
	if vault.KEKID(oldKEK) == newID {
		return 0, errors.New("vault: old and new KEK are identical; nothing to rotate")
	}
	var channels []*Channel
	if err := DB.Where("key = ? AND key_ciphertext <> ''", vault.KeySentinel).Find(&channels).Error; err != nil {
		return 0, fmt.Errorf("vault: list vaulted channels: %w", err)
	}
	for _, ch := range channels {
		if ch.KekID == newID {
			continue
		}
		plaintext, err := vault.DecryptWithKEK(oldKEK, ch.KeyCiphertext, ch.OwnerUserID)
		if err != nil {
			return rotated, fmt.Errorf("vault: channel %d decrypt with old KEK failed, aborting: %w", ch.Id, err)
		}
		ciphertext, kekID, err := vault.EncryptWithKEK(newKEK, plaintext, ch.OwnerUserID)
		if err != nil {
			return rotated, fmt.Errorf("vault: channel %d encrypt with new KEK failed, aborting: %w", ch.Id, err)
		}
		updates := map[string]any{
			"key_ciphertext": ciphertext,
			"kek_id":         kekID,
		}
		if err := DB.Model(&Channel{}).Where("id = ?", ch.Id).Updates(updates).Error; err != nil {
			return rotated, fmt.Errorf("vault: channel %d persist rotated ciphertext: %w", ch.Id, err)
		}
		rotated++
	}
	if rotated > 0 {
		common.SysLog(fmt.Sprintf("vault: rotated KEK for %d channel(s), new kek_id %s", rotated, newID))
	}
	return rotated, nil
}
