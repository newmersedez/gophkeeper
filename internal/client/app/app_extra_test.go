package app_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/client"
	"gophkeeper/internal/client/app"
	"gophkeeper/internal/client/localstore"
	"gophkeeper/internal/domain"
	"gophkeeper/internal/server/handlers"
	"gophkeeper/internal/server/storage"

	"log/slog"

	"github.com/stretchr/testify/require"
)

func TestAppUpdateAndLogin(t *testing.T) {
	srvStore := storage.OpenTest(t)

	server := httptest.NewServer(handlers.NewRouter(srvStore, auth.NewService("s"), slog.Default()).Routes())
	t.Cleanup(server.Close)

	store, err := localstore.Open(filepath.Join(t.TempDir(), "c.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	api := client.New(server.URL, server.Client())
	application := app.New(api, store, t.TempDir())
	ctx := context.Background()

	require.NoError(t, application.Register(ctx, "u", "p"))

	store2, err := localstore.Open(filepath.Join(t.TempDir(), "c2.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store2.Close() })
	app2 := app.New(client.New(server.URL, server.Client()), store2, t.TempDir())
	require.NoError(t, app2.Login(ctx, "u", "p"))

	item, err := application.AddItem(domain.ItemPayload{
		Type: domain.ItemCredentials, Title: "mail",
		Credentials: &domain.CredentialsData{Login: "a", Password: "b"},
	})
	require.NoError(t, err)

	require.NoError(t, application.UpdateItem(item.ID, domain.ItemPayload{
		Type: domain.ItemCredentials, Title: "mail2",
		Credentials: &domain.CredentialsData{Login: "a2", Password: "b2"},
	}))

	require.NoError(t, application.Sync(ctx, false))
	require.NoError(t, app2.Sync(ctx, true))

	list, err := app2.ListItems()
	require.NoError(t, err)
	require.NotEmpty(t, list)

	_, err = app.DefaultDataDir()
	require.NoError(t, err)
}
