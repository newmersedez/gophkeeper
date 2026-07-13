// Package handlers реализует HTTP API сервера GophKeeper.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/domain"
	"gophkeeper/internal/protocol"
	"gophkeeper/internal/server/middleware"
	"gophkeeper/internal/server/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AuthStorage — хранилище для аутентификации.
type AuthStorage interface {
	CreateUser(ctx context.Context, login, passwordHash string) (uuid.UUID, error)
	GetUserByLogin(ctx context.Context, login string) (*domain.User, error)
}

// VaultStorage — хранилище записей сейфа.
type VaultStorage interface {
	UpsertItem(ctx context.Context, item *domain.VaultItem) error
	ListItemsSince(ctx context.Context, userID uuid.UUID, since time.Time) ([]domain.VaultItem, error)
	ListAllItems(ctx context.Context, userID uuid.UUID) ([]domain.VaultItem, error)
	GetItem(ctx context.Context, userID, itemID uuid.UUID) (*domain.VaultItem, error)
}

// Storage объединяет зависимости хендлеров.
type Storage interface {
	AuthStorage
	VaultStorage
}

// Router связывает HTTP-маршруты.
type Router struct {
	store  Storage
	auth   *auth.Service
	logger *slog.Logger
}

// NewRouter создаёт роутер API.
func NewRouter(store Storage, authSvc *auth.Service, logger *slog.Logger) *Router {
	return &Router{store: store, auth: authSvc, logger: logger}
}

// Routes возвращает http.Handler со всеми маршрутами.
func (rt *Router) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", rt.Register)
		r.Post("/login", rt.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(rt.auth))
			r.Get("/items", rt.ListItems)
			r.Get("/items/{id}", rt.GetItem)
			r.Post("/items", rt.UpsertItemJSON)
			r.Post("/sync", rt.SyncJSON)
			r.Post("/sync/binary", rt.SyncBinary)
		})
	})

	return r
}

type credentialsRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

// Register регистрирует нового пользователя.
func (rt *Router) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Login == "" || req.Password == "" {
		http.Error(w, "login and password required", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		rt.logger.Error("hash password", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	userID, err := rt.store.CreateUser(r.Context(), req.Login, hash)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			http.Error(w, "login already taken", http.StatusConflict)
			return
		}
		rt.logger.Error("create user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	token, err := rt.auth.GenerateToken(userID)
	if err != nil {
		rt.logger.Error("generate token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

// Login аутентифицирует пользователя.
func (rt *Router) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	user, err := rt.store.GetUserByLogin(r.Context(), req.Login)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		rt.logger.Error("get user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := rt.auth.GenerateToken(user.ID)
	if err != nil {
		rt.logger.Error("generate token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

type itemDTO struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Deleted   bool      `json:"deleted"`
	Payload   []byte    `json:"payload"`
}

type syncRequestJSON struct {
	Since time.Time `json:"since"`
	Items []itemDTO `json:"items"`
}

type syncResponseJSON struct {
	ServerTime time.Time `json:"server_time"`
	Items      []itemDTO `json:"items"`
}

// UpsertItemJSON принимает одну запись (зашифрованный payload).
func (rt *Router) UpsertItemJSON(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var dto itemDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	item, err := dtoToVault(userID, dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := rt.store.UpsertItem(r.Context(), item); err != nil {
		rt.logger.Error("upsert item", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ListItems возвращает все записи пользователя.
func (rt *Router) ListItems(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	items, err := rt.store.ListAllItems(r.Context(), userID)
	if err != nil {
		rt.logger.Error("list items", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]itemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, vaultToDTO(it))
	}
	writeJSON(w, out)
}

// GetItem возвращает одну запись.
func (rt *Router) GetItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	item, err := rt.store.GetItem(r.Context(), userID, itemID)
	if err != nil {
		if errors.Is(err, storage.ErrItemNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		rt.logger.Error("get item", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, vaultToDTO(*item))
}

// SyncJSON выполняет двустороннюю синхронизацию в JSON.
func (rt *Router) SyncJSON(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req syncRequestJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	resp, err := rt.doSync(r, userID, req.Since, req.Items)
	if err != nil {
		rt.logger.Error("sync", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp)
}

// SyncBinary выполняет синхронизацию через gob-бинарный протокол.
func (rt *Router) SyncBinary(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	var req protocol.SyncRequest
	if err := protocol.Decode(body, &req); err != nil {
		http.Error(w, "invalid binary payload", http.StatusBadRequest)
		return
	}

	items := make([]itemDTO, 0, len(req.Items))
	for _, e := range req.Items {
		items = append(items, itemDTO{
			ID:        e.ID.String(),
			Version:   e.Version,
			UpdatedAt: e.UpdatedAt,
			Deleted:   e.Deleted,
			Payload:   e.Payload,
		})
	}

	respJSON, err := rt.doSync(r, userID, req.Since, items)
	if err != nil {
		rt.logger.Error("binary sync", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := protocol.SyncResponse{ServerTime: respJSON.ServerTime}
	for _, it := range respJSON.Items {
		id, _ := uuid.Parse(it.ID)
		resp.Items = append(resp.Items, protocol.ItemEnvelope{
			ID: id, Version: it.Version, UpdatedAt: it.UpdatedAt, Deleted: it.Deleted, Payload: it.Payload,
		})
	}

	data, err := protocol.Encode(resp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (rt *Router) doSync(r *http.Request, userID uuid.UUID, since time.Time, items []itemDTO) (syncResponseJSON, error) {
	for _, dto := range items {
		item, err := dtoToVault(userID, dto)
		if err != nil {
			continue
		}
		if err := rt.store.UpsertItem(r.Context(), item); err != nil {
			return syncResponseJSON{}, err
		}
	}

	serverItems, err := rt.store.ListItemsSince(r.Context(), userID, since)
	if err != nil {
		return syncResponseJSON{}, err
	}

	out := make([]itemDTO, 0, len(serverItems))
	for _, it := range serverItems {
		out = append(out, vaultToDTO(it))
	}
	return syncResponseJSON{ServerTime: time.Now().UTC(), Items: out}, nil
}

func dtoToVault(userID uuid.UUID, dto itemDTO) (*domain.VaultItem, error) {
	id, err := uuid.Parse(dto.ID)
	if err != nil {
		return nil, errors.New("invalid item id")
	}
	if len(dto.Payload) == 0 && !dto.Deleted {
		return nil, errors.New("payload required")
	}
	updated := dto.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}
	version := dto.Version
	if version == 0 {
		version = 1
	}
	return &domain.VaultItem{
		ID:        id,
		UserID:    userID,
		Version:   version,
		UpdatedAt: updated,
		Deleted:   dto.Deleted,
		Payload:   dto.Payload,
	}, nil
}

func vaultToDTO(item domain.VaultItem) itemDTO {
	return itemDTO{
		ID:        item.ID.String(),
		Version:   item.Version,
		UpdatedAt: item.UpdatedAt.UTC(),
		Deleted:   item.Deleted,
		Payload:   item.Payload,
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}
