package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/server/handlers"
	"gophkeeper/internal/server/storage"

	"log/slog"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerEdgeCases(t *testing.T) {
	store := storage.OpenTest(t)
	authSvc := auth.NewService("s")
	handler := handlers.NewRouter(store, authSvc, slog.Default()).Routes()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBufferString(`not-json`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(`not-json`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := `{"login":"ok","password":"pwd"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBufferString(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var tok struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tok))

	// wrong password
	req = httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(`{"login":"ok","password":"bad"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// bad item id
	req = httptest.NewRequest(http.MethodGet, "/api/v1/items/not-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// missing item
	req = httptest.NewRequest(http.MethodGet, "/api/v1/items/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// bad upsert payload
	req = httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewBufferString(`{"id":"bad"}`))
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// empty payload without deleted
	payload, _ := json.Marshal(map[string]any{
		"id": uuid.New().String(), "version": 1, "updated_at": time.Now(), "payload": []byte{},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// deleted item without payload ok
	payload, _ = json.Marshal(map[string]any{
		"id": uuid.New().String(), "version": 1, "updated_at": time.Now(), "deleted": true, "payload": []byte{},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// bad sync json
	req = httptest.NewRequest(http.MethodPost, "/api/v1/sync", bytes.NewBufferString(`{`))
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// bad binary
	req = httptest.NewRequest(http.MethodPost, "/api/v1/sync/binary", bytes.NewBufferString("xxx"))
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
