package app_test

import (
	"context"
	"encoding/base32"
	"log/slog"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppRegisterAddSyncOTP(t *testing.T) {
	srvStore := storage.OpenTest(t)

	authSvc := auth.NewService("secret")
	router := handlers.NewRouter(srvStore, authSvc, slog.Default())
	server := httptest.NewServer(router.Routes())
	t.Cleanup(server.Close)

	store, err := localstore.Open(filepath.Join(t.TempDir(), "client.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	api := client.New(server.URL, server.Client())
	application := app.New(api, store, t.TempDir())

	ctx := context.Background()
	require.NoError(t, application.Register(ctx, "carol", "pass"))

	item, err := application.AddItem(domain.ItemPayload{
		Type: domain.ItemText, Title: "note",
		Text:     &domain.TextData{Content: "secret note"},
		Metadata: map[string]string{"site": "example.com"},
	})
	require.NoError(t, err)

	otpItem, err := application.AddItem(domain.ItemPayload{
		Type: domain.ItemOTP, Title: "gh",
		// ggignore — demo TOTP seed ("Hello!"), not a real secret
		OTP: &domain.OTPData{
			Secret: base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("Hello!")),
			Period: 30,
			Digits: 6,
		},
	})
	require.NoError(t, err)

	require.NoError(t, application.Sync(ctx, false))
	require.NoError(t, application.Sync(ctx, true))

	list, err := application.ListItems()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)

	got, err := application.GetItem(item.ID)
	require.NoError(t, err)
	assert.Equal(t, "secret note", got.Payload.Text.Content)

	code, err := application.OTPCode(otpItem.ID)
	require.NoError(t, err)
	assert.Len(t, code, 6)

	require.NoError(t, application.DeleteItem(item.ID))
	require.NoError(t, application.Sync(ctx, false))
}

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
