package vault

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withKEK(t *testing.T, raw string) {
	t.Helper()
	t.Setenv(EnvKEK, raw)
	SetProvider(envKEKProvider{})
}

func testKEK(t *testing.T) string {
	t.Helper()
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i + 1)
	}
	enc := base64.StdEncoding.EncodeToString(kek)
	withKEK(t, enc)
	return enc
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	testKEK(t)
	plaintext := "sk-test-12345\nsk-test-67890"
	ct, kekID, err := Encrypt(plaintext, 7)
	require.NoError(t, err)
	assert.NotEmpty(t, ct)
	assert.NotEmpty(t, kekID)
	assert.NotContains(t, ct, "sk-test")

	back, err := Decrypt(ct, kekID, 7)
	require.NoError(t, err)
	assert.Equal(t, plaintext, back)
}

func TestEncryptDecryptJSONBlob(t *testing.T) {
	testKEK(t)
	// Vertex AI style: the whole Key column is one opaque JSON blob.
	plaintext := `[{"type":"service_account","private_key":"secret"}]`
	ct, kekID, err := Encrypt(plaintext, 0)
	require.NoError(t, err)
	back, err := Decrypt(ct, kekID, 0)
	require.NoError(t, err)
	assert.Equal(t, plaintext, back)
}

func TestDEKIsPerOwner(t *testing.T) {
	testKEK(t)
	ct, kekID, err := Encrypt("sk-owner-a", 1)
	require.NoError(t, err)
	_, err = Decrypt(ct, kekID, 2)
	assert.Error(t, err, "decrypting with another owner's DEK must fail")
	back, err := Decrypt(ct, kekID, 1)
	require.NoError(t, err)
	assert.Equal(t, "sk-owner-a", back)
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	testKEK(t)
	ct, kekID, err := Encrypt("sk-test", 3)
	require.NoError(t, err)
	raw, _ := base64.StdEncoding.DecodeString(ct)
	raw[len(raw)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(raw)
	_, err = Decrypt(tampered, kekID, 3)
	assert.Error(t, err)
}

func TestDecryptWrongKEKFails(t *testing.T) {
	testKEK(t)
	ct, kekID, err := Encrypt("sk-test", 3)
	require.NoError(t, err)

	other := make([]byte, 32)
	for i := range other {
		other[i] = byte(255 - i)
	}
	withKEK(t, base64.StdEncoding.EncodeToString(other))
	// kek_id mismatch is caught before decryption is even attempted.
	_, err = Decrypt(ct, kekID, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kek_id")
}

func TestMissingKEKFailsClosed(t *testing.T) {
	withKEK(t, "")
	_, _, err := Encrypt("sk-test", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), EnvKEK)
	_, err = Decrypt("anything", "", 1)
	assert.Error(t, err)
	assert.False(t, IsConfigured())
}

func TestBadKEKLengthRejected(t *testing.T) {
	withKEK(t, base64.StdEncoding.EncodeToString([]byte("too-short")))
	_, _, err := Encrypt("sk-test", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestRefusesSentinelAndEmpty(t *testing.T) {
	testKEK(t)
	_, _, err := Encrypt("", 1)
	assert.Error(t, err)
	_, _, err = Encrypt(KeySentinel, 1)
	assert.Error(t, err)
}

func TestNonceRandomness(t *testing.T) {
	testKEK(t)
	ct1, _, err := Encrypt("same-plaintext", 1)
	require.NoError(t, err)
	ct2, _, err := Encrypt("same-plaintext", 1)
	require.NoError(t, err)
	assert.NotEqual(t, ct1, ct2, "AES-GCM must use a fresh random nonce per encryption")
}

func TestKekIDChangesWithKEK(t *testing.T) {
	testKEK(t)
	id1 := defaultProvider.ID()
	assert.True(t, strings.HasPrefix(id1, "env-"))
	other := make([]byte, 32)
	for i := range other {
		other[i] = byte(i + 100)
	}
	withKEK(t, base64.StdEncoding.EncodeToString(other))
	id2 := defaultProvider.ID()
	assert.NotEqual(t, id1, id2)
}

func TestMain(m *testing.M) {
	os.Unsetenv(EnvKEK)
	SetProvider(envKEKProvider{})
	m.Run()
}
