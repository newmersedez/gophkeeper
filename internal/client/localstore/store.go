// Package localstore хранит локальный зашифрованный кэш записей клиента.
package localstore

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gophkeeper/internal/crypto"
	"gophkeeper/internal/domain"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound — запись не найдена.
var ErrNotFound = errors.New("item not found")

// Store — локальное SQLite-хранилище клиента.
type Store struct {
	db       *sql.DB
	password string
}

// Open открывает (или создаёт) локальную БД и применяет миграции.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func runMigrations(db *sql.DB) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("sqlite migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	// Не закрываем m через Close: драйвер шарит *sql.DB с Store.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// Close закрывает БД.
func (s *Store) Close() error {
	return s.db.Close()
}

// SetPassword задаёт master-пароль для шифрования полезной нагрузки.
func (s *Store) SetPassword(password string) {
	s.password = password
}

// Password возвращает текущий master-пароль.
func (s *Store) Password() string {
	return s.password
}

// SetMeta сохраняет строковый мета-параметр (например token, last_sync).
func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// GetMeta возвращает мета-параметр.
func (s *Store) GetMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

// SaveItem сохраняет расшифрованную запись (шифрует payload на диске).
func (s *Store) SaveItem(item domain.LocalItem) error {
	if s.password == "" {
		return errors.New("master password is not set")
	}
	raw, err := json.Marshal(item.Payload)
	if err != nil {
		return err
	}
	enc, err := crypto.EncryptWithPassword(raw, s.password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO items(id, version, updated_at, deleted, dirty, payload)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
 version=excluded.version,
 updated_at=excluded.updated_at,
 deleted=excluded.deleted,
 dirty=excluded.dirty,
 payload=excluded.payload`,
		item.ID.String(), item.Version, item.UpdatedAt.UTC(),
		boolToInt(item.Deleted), boolToInt(item.Dirty), enc,
	)
	return err
}

// GetItem возвращает запись по ID.
func (s *Store) GetItem(id uuid.UUID) (*domain.LocalItem, error) {
	row := s.db.QueryRow(`
SELECT id, version, updated_at, deleted, dirty, payload FROM items WHERE id = ?`, id.String())
	item, err := s.scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListItems возвращает активные (не удалённые) записи.
func (s *Store) ListItems() ([]domain.LocalItem, error) {
	rows, err := s.db.Query(`
SELECT id, version, updated_at, deleted, dirty, payload FROM items WHERE deleted = 0 ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []domain.LocalItem
	for rows.Next() {
		item, err := s.scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListDirty возвращает изменённые локально записи для push.
func (s *Store) ListDirty() ([]domain.LocalItem, error) {
	rows, err := s.db.Query(`
SELECT id, version, updated_at, deleted, dirty, payload FROM items WHERE dirty = 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []domain.LocalItem
	for rows.Next() {
		item, err := s.scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// MarkClean сбрасывает dirty-флаг.
func (s *Store) MarkClean(id uuid.UUID) error {
	_, err := s.db.Exec(`UPDATE items SET dirty = 0 WHERE id = ?`, id.String())
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanItem(row scanner) (domain.LocalItem, error) {
	var (
		item    domain.LocalItem
		idStr   string
		deleted int
		dirty   int
		enc     []byte
	)
	if err := row.Scan(&idStr, &item.Version, &item.UpdatedAt, &deleted, &dirty, &enc); err != nil {
		return item, err
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return item, err
	}
	item.ID = id
	item.Deleted = deleted != 0
	item.Dirty = dirty != 0

	if s.password == "" {
		return item, errors.New("master password is not set")
	}
	raw, err := crypto.DecryptWithPassword(enc, s.password)
	if err != nil {
		return item, err
	}
	if err := json.Unmarshal(raw, &item.Payload); err != nil {
		return item, err
	}
	return item, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
