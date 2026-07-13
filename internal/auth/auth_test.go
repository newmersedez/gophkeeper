package auth_test

import (
	"testing"

	"gophkeeper/internal/auth"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	t.Parallel()

	hash, err := auth.HashPassword("secret")
	require.NoError(t, err)
	assert.True(t, auth.CheckPassword("secret", hash))
	assert.False(t, auth.CheckPassword("wrong", hash))
}

func TestGenerateAndValidateToken(t *testing.T) {
	t.Parallel()

	svc := auth.NewService("test-secret")
	userID := uuid.New()

	token, err := svc.GenerateToken(userID)
	require.NoError(t, err)

	got, err := svc.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)

	_, err = svc.ValidateToken("bad.token.value")
	assert.Error(t, err)

	other := auth.NewService("other-secret")
	_, err = other.ValidateToken(token)
	assert.Error(t, err)
}

func TestNewServiceEmptySecret(t *testing.T) {
	t.Parallel()

	svc := auth.NewService("")
	id := uuid.New()
	token, err := svc.GenerateToken(id)
	require.NoError(t, err)
	got, err := svc.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, id, got)
}
