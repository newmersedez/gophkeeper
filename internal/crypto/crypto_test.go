package crypto_test

import (
	"testing"

	"gophkeeper/internal/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptWithPassword(t *testing.T) {
	t.Parallel()

	plaintext := []byte(`{"type":"text","title":"note","text":{"content":"secret"}}`)
	password := "master-password"

	blob, err := crypto.EncryptWithPassword(plaintext, password)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, blob)

	got, err := crypto.DecryptWithPassword(blob, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)

	_, err = crypto.DecryptWithPassword(blob, "wrong")
	assert.ErrorIs(t, err, crypto.ErrDecryptionFailed)
}

func TestEncryptDecryptWithKey(t *testing.T) {
	t.Parallel()

	salt, err := crypto.NewSalt()
	require.NoError(t, err)
	key := crypto.DeriveKey("pass", salt)

	blob, err := crypto.Encrypt([]byte("hello"), key, salt)
	require.NoError(t, err)

	out, err := crypto.Decrypt(blob, key)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), out)
}

func TestExtractSaltAndBase64(t *testing.T) {
	t.Parallel()

	blob, err := crypto.EncryptWithPassword([]byte("x"), "p")
	require.NoError(t, err)

	salt, err := crypto.ExtractSalt(blob)
	require.NoError(t, err)
	assert.Len(t, salt, 16)

	enc := crypto.EncodeBase64(blob)
	decoded, err := crypto.DecodeBase64(enc)
	require.NoError(t, err)
	assert.Equal(t, blob, decoded)

	_, err = crypto.ExtractSalt([]byte("short"))
	assert.Error(t, err)
}

func TestInvalidKeySize(t *testing.T) {
	t.Parallel()

	_, err := crypto.Encrypt([]byte("a"), []byte("short"), nil)
	assert.Error(t, err)

	_, err = crypto.Decrypt([]byte("0123456789abcdefnonce!!!!cipher"), []byte("short"))
	assert.Error(t, err)
}
