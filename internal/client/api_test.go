package client_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/client"
	"gophkeeper/internal/server/handlers"
	"gophkeeper/internal/server/storage"

	"log/slog"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRegisterLoginListSync(t *testing.T) {
	store := storage.OpenTest(t)

	srv := httptest.NewServer(handlers.NewRouter(store, auth.NewService("s"), slog.Default()).Routes())
	t.Cleanup(srv.Close)

	api := client.New(srv.URL, srv.Client())
	ctx := context.Background()

	require.NoError(t, api.Register(ctx, "eve", "secret"))
	assert.NotEmpty(t, api.Token())

	api2 := client.New(srv.URL, srv.Client())
	require.NoError(t, api2.Login(ctx, "eve", "secret"))

	item := client.Item{
		ID: uuid.New().String(), Version: 1, UpdatedAt: time.Now().UTC(), Payload: []byte("enc"),
	}
	resp, err := api2.SyncJSON(ctx, client.SyncRequest{Items: []client.Item{item}})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Items)

	resp, err = api2.SyncBinary(ctx, time.Time{}, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Items)

	list, err := api2.ListItems(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, list)
}

func TestAPIErrors(t *testing.T) {
	store := storage.OpenTest(t)
	srv := httptest.NewServer(handlers.NewRouter(store, auth.NewService("s"), slog.Default()).Routes())
	t.Cleanup(srv.Close)

	api := client.New(srv.URL, srv.Client())
	err := api.Login(context.Background(), "nope", "x")
	assert.Error(t, err)
}
