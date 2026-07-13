package localstore_test

import (
	"path/filepath"
	"testing"
	"time"

	"gophkeeper/internal/client/localstore"
	"gophkeeper/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStore(t *testing.T) {
	t.Parallel()

	store, err := localstore.Open(filepath.Join(t.TempDir(), "vault.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	store.SetPassword("master")
	assert.Equal(t, "master", store.Password())

	require.NoError(t, store.SetMeta("token", "abc"))
	v, err := store.GetMeta("token")
	require.NoError(t, err)
	assert.Equal(t, "abc", v)

	id := uuid.New()
	item := domain.LocalItem{
		ID: id, Version: 1, UpdatedAt: time.Now().UTC(), Dirty: true,
		Payload: domain.ItemPayload{Type: domain.ItemText, Title: "n", Text: &domain.TextData{Content: "hi"}},
	}
	require.NoError(t, store.SaveItem(item))

	got, err := store.GetItem(id)
	require.NoError(t, err)
	assert.Equal(t, "n", got.Payload.Title)
	assert.Equal(t, "hi", got.Payload.Text.Content)

	list, err := store.ListItems()
	require.NoError(t, err)
	assert.Len(t, list, 1)

	dirty, err := store.ListDirty()
	require.NoError(t, err)
	assert.Len(t, dirty, 1)

	require.NoError(t, store.MarkClean(id))
	dirty, err = store.ListDirty()
	require.NoError(t, err)
	assert.Empty(t, dirty)

	_, err = store.GetItem(uuid.New())
	assert.ErrorIs(t, err, localstore.ErrNotFound)
}
