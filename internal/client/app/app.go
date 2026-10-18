// Package app содержит use-case логику CLI-клиента.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gophkeeper/internal/client"
	"gophkeeper/internal/client/localstore"
	"gophkeeper/internal/crypto"
	"gophkeeper/internal/domain"
	"gophkeeper/internal/otp"

	"github.com/google/uuid"
)

// App — фасад клиентского приложения.
type App struct {
	API   *client.API
	Store *localstore.Store
	DataDir string
}

// New создаёт приложение с указанным API и локальным хранилищем.
func New(api *client.API, store *localstore.Store, dataDir string) *App {
	return &App{API: api, Store: store, DataDir: dataDir}
}

// DefaultDataDir возвращает ~/.gophkeeper.
func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gophkeeper"), nil
}

// Register регистрирует пользователя и сохраняет сессию.
func (a *App) Register(ctx context.Context, login, password string) error {
	if err := a.API.Register(ctx, login, password); err != nil {
		return err
	}
	a.Store.SetPassword(password)
	return a.persistSession(login, password)
}

// Login выполняет вход и сохраняет сессию.
func (a *App) Login(ctx context.Context, login, password string) error {
	if err := a.API.Login(ctx, login, password); err != nil {
		return err
	}
	a.Store.SetPassword(password)
	return a.persistSession(login, password)
}

func (a *App) persistSession(login, password string) error {
	if err := a.Store.SetMeta("token", a.API.Token()); err != nil {
		return err
	}
	if err := a.Store.SetMeta("login", login); err != nil {
		return err
	}
	// пароль хранится зашифрованным ключом, производным от login (упрощение для курса)
	enc, err := crypto.EncryptWithPassword([]byte(password), login+"|gophkeeper")
	if err != nil {
		return err
	}
	return a.Store.SetMeta("master", crypto.EncodeBase64(enc))
}

// RestoreSession восстанавливает токен и master-пароль из локального хранилища.
func (a *App) RestoreSession() error {
	token, err := a.Store.GetMeta("token")
	if err != nil {
		return fmt.Errorf("not logged in: %w", err)
	}
	login, err := a.Store.GetMeta("login")
	if err != nil {
		return err
	}
	masterB64, err := a.Store.GetMeta("master")
	if err != nil {
		return err
	}
	blob, err := crypto.DecodeBase64(masterB64)
	if err != nil {
		return err
	}
	passwordBytes, err := crypto.DecryptWithPassword(blob, login+"|gophkeeper")
	if err != nil {
		return err
	}
	a.API.SetToken(token)
	a.Store.SetPassword(string(passwordBytes))
	return nil
}

// AddItem добавляет новую запись локально.
func (a *App) AddItem(payload domain.ItemPayload) (*domain.LocalItem, error) {
	if !payload.Type.Valid() {
		return nil, fmt.Errorf("unsupported item type: %s", payload.Type)
	}
	item := domain.LocalItem{
		ID:        uuid.New(),
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		Deleted:   false,
		Dirty:     true,
		Payload:   payload,
	}
	if err := a.Store.SaveItem(item); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItem обновляет существующую запись.
func (a *App) UpdateItem(id uuid.UUID, payload domain.ItemPayload) error {
	existing, err := a.Store.GetItem(id)
	if err != nil {
		return err
	}
	existing.Payload = payload
	existing.Version++
	existing.UpdatedAt = time.Now().UTC()
	existing.Dirty = true
	existing.Deleted = false
	return a.Store.SaveItem(*existing)
}

// DeleteItem помечает запись удалённой.
func (a *App) DeleteItem(id uuid.UUID) error {
	existing, err := a.Store.GetItem(id)
	if err != nil {
		return err
	}
	existing.Deleted = true
	existing.Dirty = true
	existing.Version++
	existing.UpdatedAt = time.Now().UTC()
	return a.Store.SaveItem(*existing)
}

// GetItem возвращает локальную запись.
func (a *App) GetItem(id uuid.UUID) (*domain.LocalItem, error) {
	return a.Store.GetItem(id)
}

// ListItems возвращает локальные записи.
func (a *App) ListItems() ([]domain.LocalItem, error) {
	return a.Store.ListItems()
}

// Sync выполняет синхронизацию. binary=true использует gob-протокол.
func (a *App) Sync(ctx context.Context, binary bool) error {
	if err := a.RestoreSession(); err != nil {
		return err
	}

	dirty, err := a.Store.ListDirty()
	if err != nil {
		return err
	}

	var since time.Time
	if raw, err := a.Store.GetMeta("last_sync"); err == nil {
		since, _ = time.Parse(time.RFC3339Nano, raw)
	}

	wireItems := make([]client.Item, 0, len(dirty))
	for _, it := range dirty {
		raw, err := json.Marshal(it.Payload)
		if err != nil {
			return err
		}
		enc, err := crypto.EncryptWithPassword(raw, a.Store.Password())
		if err != nil {
			return err
		}
		wireItems = append(wireItems, client.Item{
			ID: it.ID.String(), Version: it.Version, UpdatedAt: it.UpdatedAt,
			Deleted: it.Deleted, Payload: enc,
		})
	}

	var resp *client.SyncResponse
	if binary {
		resp, err = a.API.SyncBinary(ctx, since, wireItems)
	} else {
		resp, err = a.API.SyncJSON(ctx, client.SyncRequest{Since: since, Items: wireItems})
	}
	if err != nil {
		return err
	}

	for _, remote := range resp.Items {
		id, err := uuid.Parse(remote.ID)
		if err != nil {
			continue
		}
		raw, err := crypto.DecryptWithPassword(remote.Payload, a.Store.Password())
		if err != nil {
			continue
		}
		var payload domain.ItemPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}

		local, err := a.Store.GetItem(id)
		if err == nil && local.Version >= remote.Version && !local.Dirty {
			continue
		}

		item := domain.LocalItem{
			ID: id, Version: remote.Version, UpdatedAt: remote.UpdatedAt,
			Deleted: remote.Deleted, Dirty: false, Payload: payload,
		}
		if err := a.Store.SaveItem(item); err != nil {
			return err
		}
	}

	for _, it := range dirty {
		_ = a.Store.MarkClean(it.ID)
	}

	return a.Store.SetMeta("last_sync", resp.ServerTime.UTC().Format(time.RFC3339Nano))
}

// OTPCode генерирует текущий TOTP для OTP-записи.
func (a *App) OTPCode(id uuid.UUID) (string, error) {
	item, err := a.Store.GetItem(id)
	if err != nil {
		return "", err
	}
	if item.Payload.Type != domain.ItemOTP || item.Payload.OTP == nil {
		return "", fmt.Errorf("item is not otp")
	}
	period := item.Payload.OTP.Period
	digits := item.Payload.OTP.Digits
	return otp.Generate(item.Payload.OTP.Secret, time.Now(), period, digits)
}
