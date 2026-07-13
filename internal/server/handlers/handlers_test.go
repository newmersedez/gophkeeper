package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/protocol"
	"gophkeeper/internal/server/handlers"
	"gophkeeper/internal/server/storage"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRouter(t *testing.T) (http.Handler, *storage.Storage, *auth.Service) {
	t.Helper()
	store := storage.OpenTest(t)
	authSvc := auth.NewService("test-secret")
	rt := handlers.NewRouter(store, authSvc, slog.Default())
	return rt.Routes(), store, authSvc
}

func TestRegisterLoginAndSync(t *testing.T) {
	handler, _, _ := setupRouter(t)

	regBody := `{"login":"bob","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBufferString(regBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var tokenResp struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokenResp))
	require.NotEmpty(t, tokenResp.Token)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(regBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	itemID := uuid.New()
	payload := map[string]any{
		"id":         itemID.String(),
		"version":    1,
		"updated_at": time.Now().UTC(),
		"deleted":    false,
		"payload":    []byte("enc-data"),
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/items/"+itemID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	syncBody, _ := json.Marshal(map[string]any{
		"since": time.Time{},
		"items": []any{},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/sync", bytes.NewReader(syncBody))
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var syncResp struct {
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &syncResp))
	assert.NotEmpty(t, syncResp.Items)
}

func TestRegisterConflictAndUnauthorized(t *testing.T) {
	handler, _, _ := setupRouter(t)

	body := `{"login":"dup","password":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBufferString(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBufferString(`{}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBinarySync(t *testing.T) {
	handler, store, authSvc := setupRouter(t)
	ctx := context.Background()
	userID, err := store.CreateUser(ctx, "bin", "hash")
	require.NoError(t, err)
	token, err := authSvc.GenerateToken(userID)
	require.NoError(t, err)

	itemID := uuid.New()
	reqBin := protocol.SyncRequest{
		Since: time.Time{},
		Items: []protocol.ItemEnvelope{{
			ID: itemID, Version: 1, UpdatedAt: time.Now().UTC(), Payload: []byte("bin"),
		}},
	}
	data, err := protocol.Encode(reqBin)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/binary", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp protocol.SyncResponse
	require.NoError(t, protocol.Decode(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Items)
}

func TestLoginInvalid(t *testing.T) {
	handler, _, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(`{"login":"no","password":"x"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
