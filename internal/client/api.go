// Package client реализует HTTP/бинарный API-клиент GophKeeper.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gophkeeper/internal/protocol"

	"github.com/google/uuid"
)

// Item — DTO записи сейфа на проводе.
type Item struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Deleted   bool      `json:"deleted"`
	Payload   []byte    `json:"payload"`
}

// SyncRequest — JSON-запрос синхронизации.
type SyncRequest struct {
	Since time.Time `json:"since"`
	Items []Item    `json:"items"`
}

// SyncResponse — JSON-ответ синхронизации.
type SyncResponse struct {
	ServerTime time.Time `json:"server_time"`
	Items      []Item    `json:"items"`
}

// API — клиент удалённого сервера.
type API struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// New создаёт API-клиент.
func New(baseURL string, httpClient *http.Client) *API {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &API{baseURL: baseURL, httpClient: httpClient}
}

// SetToken сохраняет JWT для авторизованных запросов.
func (a *API) SetToken(token string) {
	a.token = token
}

// Token возвращает текущий JWT.
func (a *API) Token() string {
	return a.token
}

// Register регистрирует пользователя и сохраняет токен.
func (a *API) Register(ctx context.Context, login, password string) error {
	token, err := a.auth(ctx, "/api/v1/register", login, password)
	if err != nil {
		return err
	}
	a.token = token
	return nil
}

// Login выполняет вход и сохраняет токен.
func (a *API) Login(ctx context.Context, login, password string) error {
	token, err := a.auth(ctx, "/api/v1/login", login, password)
	if err != nil {
		return err
	}
	a.token = token
	return nil
}

func (a *API) auth(ctx context.Context, path, login, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"login": login, "password": password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, string(data))
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	return out.Token, nil
}

// SyncJSON синхронизирует записи через JSON API.
func (a *API) SyncJSON(ctx context.Context, req SyncRequest) (*SyncResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/v1/sync", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	a.setAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(data))
	}
	var out SyncResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SyncBinary синхронизирует записи через gob-протокол.
func (a *API) SyncBinary(ctx context.Context, since time.Time, items []Item) (*SyncResponse, error) {
	req := protocol.SyncRequest{Since: since}
	for _, it := range items {
		id, err := uuid.Parse(it.ID)
		if err != nil {
			continue
		}
		req.Items = append(req.Items, protocol.ItemEnvelope{
			ID: id, Version: it.Version, UpdatedAt: it.UpdatedAt, Deleted: it.Deleted, Payload: it.Payload,
		})
	}
	body, err := protocol.Encode(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/v1/sync/binary", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	a.setAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/octet-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(data))
	}

	var bin protocol.SyncResponse
	if err := protocol.Decode(data, &bin); err != nil {
		return nil, err
	}
	out := &SyncResponse{ServerTime: bin.ServerTime}
	for _, e := range bin.Items {
		out.Items = append(out.Items, Item{
			ID: e.ID.String(), Version: e.Version, UpdatedAt: e.UpdatedAt, Deleted: e.Deleted, Payload: e.Payload,
		})
	}
	return out, nil
}

// ListItems возвращает все записи с сервера.
func (a *API) ListItems(ctx context.Context) ([]Item, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/v1/items", nil)
	if err != nil {
		return nil, err
	}
	a.setAuth(httpReq)
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(data))
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (a *API) setAuth(req *http.Request) {
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
}
