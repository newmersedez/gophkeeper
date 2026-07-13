package otp_test

import (
	"testing"
	"time"

	"gophkeeper/internal/otp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 6238 test vector (SHA1, secret "12345678901234567890").
func TestGenerateKnownVector(t *testing.T) {
	t.Parallel()

	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // base32 of "12345678901234567890"
	ts := time.Unix(1111111109, 0).UTC()

	code, err := otp.Generate(secret, ts, 30, 6)
	require.NoError(t, err)
	assert.Equal(t, "081804", code)
}

func TestVerify(t *testing.T) {
	t.Parallel()

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	code, err := otp.Generate(secret, now, 30, 6)
	require.NoError(t, err)

	ok, err := otp.Verify(secret, code, now, 30, 6)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = otp.Verify(secret, "000000", now, 30, 6)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestInvalidSecret(t *testing.T) {
	t.Parallel()

	_, err := otp.Generate("!!!", time.Now(), 30, 6)
	assert.Error(t, err)
}
