package storage_test

import (
	"context"
	"testing"
	"time"

	"gophkeeper/internal/domain"
	"gophkeeper/internal/server/storage"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageUsersAndItems(t *testing.T) {
	s := storage.OpenTest(t)
	ctx := context.Background()

	id, err := s.CreateUser(ctx, "alice", "hash")
	require.NoError(t, err)

	_, err = s.CreateUser(ctx, "alice", "hash2")
	assert.ErrorIs(t, err, storage.ErrUserExists)

	user, err := s.GetUserByLogin(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, id, user.ID)

	_, err = s.GetUserByLogin(ctx, "missing")
	assert.ErrorIs(t, err, storage.ErrUserNotFound)

	itemID := uuid.New()
	item := &domain.VaultItem{
		ID: itemID, UserID: id, Version: 1, UpdatedAt: time.Now().UTC(),
		Payload: []byte("cipher"),
	}
	require.NoError(t, s.UpsertItem(ctx, item))

	got, err := s.GetItem(ctx, id, itemID)
	require.NoError(t, err)
	assert.Equal(t, []byte("cipher"), got.Payload)

	item.Version = 2
	item.Payload = []byte("cipher2")
	require.NoError(t, s.UpsertItem(ctx, item))

	item.Version = 1
	item.Payload = []byte("old")
	require.NoError(t, s.UpsertItem(ctx, item)) // ignored, older version

	got, err = s.GetItem(ctx, id, itemID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), got.Version)
	assert.Equal(t, []byte("cipher2"), got.Payload)

	all, err := s.ListAllItems(ctx, id)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	since := time.Now().UTC().Add(time.Hour)
	fresh, err := s.ListItemsSince(ctx, id, since)
	require.NoError(t, err)
	assert.Empty(t, fresh)

	_, err = s.GetItem(ctx, id, uuid.New())
	assert.ErrorIs(t, err, storage.ErrItemNotFound)
}

func TestSyncItemsTransaction(t *testing.T) {
	s := storage.OpenTest(t)
	ctx := context.Background()

	userID, err := s.CreateUser(ctx, "sync-user", "hash")
	require.NoError(t, err)

	firstID := uuid.New()
	secondID := uuid.New()
	now := time.Now().UTC()

	items := []domain.VaultItem{
		{ID: firstID, UserID: userID, Version: 1, UpdatedAt: now, Payload: []byte("a")},
		{ID: secondID, UserID: userID, Version: 1, UpdatedAt: now.Add(time.Second), Payload: []byte("b")},
	}

	out, err := s.SyncItems(ctx, userID, time.Time{}, items)
	require.NoError(t, err)
	require.Len(t, out, 2)

	stale := []domain.VaultItem{
		{ID: firstID, UserID: userID, Version: 1, UpdatedAt: now, Payload: []byte("stale")},
	}
	_, err = s.SyncItems(ctx, userID, now.Add(time.Hour), stale)
	require.NoError(t, err)

	got, err := s.GetItem(ctx, userID, firstID)
	require.NoError(t, err)
	assert.Equal(t, []byte("a"), got.Payload)
	assert.Equal(t, int64(1), got.Version)
}
