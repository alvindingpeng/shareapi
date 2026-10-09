// Package vault implements envelope encryption for channel credentials.
//
// Design (Phase 1 credential vault):
//
//	KEK (key-encryption key, from the VAULT_KEK environment variable for now,
//	     swappable for a cloud KMS via the KEKProvider interface)
//	 └─ DEK per channel owner (derived with HKDF-SHA256, salted by owner_user_id)
//	     └─ AES-256-GCM encrypts the whole channel Key blob as one opaque value
//
// The encrypted blob is stored in channels.key_ciphertext (base64). The legacy
// channels.key column keeps the sentinel value KeySentinel and never holds
// plaintext again once a channel has been migrated.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// KeySentinel is written to channels.key once the real credential lives in the
// vault. It is never a valid upstream credential: it is non-empty (so legacy
// NOT NULL constraints keep working) and unmistakable in audits.
const KeySentinel = "__vault__"

// EnvKEK is the environment variable holding the base64-encoded 32-byte KEK.
const EnvKEK = "VAULT_KEK"

// dekInfo domain-separates the HKDF output for channel data-encryption keys.
const dekInfo = "shareapi/channel-dek/v1"

// KEKProvider abstracts where the key-encryption key comes from so a cloud
// KMS can replace the environment variable later without touching callers.
type KEKProvider interface {
	// GetKEK returns the 32-byte key-encryption key.
	GetKEK() ([]byte, error)
	// ID returns the kek_id stamped on ciphertexts encrypted with this KEK.
	// It must change whenever the KEK value changes so rotation is detectable.
	ID() string
}

// envKEKProvider reads the KEK from the VAULT_KEK environment variable.
type envKEKProvider struct{}

var defaultProvider KEKProvider = envKEKProvider{}

// SetProvider swaps the KEK source (used by tests and future KMS wiring).
func SetProvider(p KEKProvider) {
	if p != nil {
		defaultProvider = p
	}
}

// IsConfigured reports whether a KEK is available for encryption.
func IsConfigured() bool {
	_, err := defaultProvider.GetKEK()
	return err == nil
}

func (envKEKProvider) GetKEK() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(EnvKEK))
	if raw == "" {
		return nil, fmt.Errorf("vault: %s is not configured; refusing to handle channel credentials", EnvKEK)
	}
	kek, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("vault: %s must be base64-encoded: %w", EnvKEK, err)
	}
	if len(kek) != 32 {
		return nil, fmt.Errorf("vault: %s must decode to 32 bytes, got %d", EnvKEK, len(kek))
	}
	return kek, nil
}

func (envKEKProvider) ID() string {
	kek, err := envKEKProvider{}.GetKEK()
	if err != nil {
		return "env-unknown"
	}
	sum := sha256.Sum256(kek)
	return "env-" + hex.EncodeToString(sum[:])[:12]
}

// deriveDEK derives the per-owner data-encryption key from the KEK.
func deriveDEK(kek []byte, ownerUserID int) ([]byte, error) {
	salt := []byte("shareapi-owner-" + strconv.Itoa(ownerUserID))
	dek := make([]byte, 32)
	r := hkdf.New(sha256.New, kek, salt, []byte(dekInfo))
	if _, err := r.Read(dek); err != nil {
		return nil, fmt.Errorf("vault: derive DEK: %w", err)
	}
	return dek, nil
}

func gcmForDEK(dek []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("vault: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("vault: gcm: %w", err)
	}
	return gcm, nil
}

// Encrypt encrypts the whole channel key blob for the given owner.
// It returns the base64 ciphertext and the kek_id to stamp on the row.
func Encrypt(plaintext string, ownerUserID int) (ciphertext string, kekID string, err error) {
	if plaintext == "" {
		return "", "", errors.New("vault: refusing to encrypt an empty credential")
	}
	if plaintext == KeySentinel {
		return "", "", errors.New("vault: refusing to encrypt the sentinel value")
	}
	kek, err := defaultProvider.GetKEK()
	if err != nil {
		return "", "", err
	}
	defer zeroBytes(kek)
	dek, err := deriveDEK(kek, ownerUserID)
	if err != nil {
		return "", "", err
	}
	defer zeroBytes(dek)
	gcm, err := gcmForDEK(dek)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", fmt.Errorf("vault: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), defaultProvider.ID(), nil
}

// Decrypt reverses Encrypt. kekID must match the provider's current ID;
// a mismatch means the KEK was rotated and the row needs re-encryption.
func Decrypt(ciphertext string, kekID string, ownerUserID int) (string, error) {
	if ciphertext == "" {
		return "", errors.New("vault: empty ciphertext")
	}
	if kekID != "" && kekID != defaultProvider.ID() {
		return "", fmt.Errorf("vault: kek_id %q does not match the active KEK; re-encrypt this channel", kekID)
	}
	kek, err := defaultProvider.GetKEK()
	if err != nil {
		return "", err
	}
	defer zeroBytes(kek)
	dek, err := deriveDEK(kek, ownerUserID)
	if err != nil {
		return "", err
	}
	defer zeroBytes(dek)
	gcm, err := gcmForDEK(dek)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("vault: ciphertext must be base64: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("vault: ciphertext too short")
	}
	nonce, sealed := raw[:nonceSize], raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("vault: decrypt failed (wrong KEK or tampered data): %w", err)
	}
	return string(plain), nil
}

// IsVaulted reports whether a channels.key value means "credential in vault".
func IsVaulted(key string) bool {
	return key == KeySentinel
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
