// Package storage реализует PostgreSQL-хранилище сервера GophKeeper.
package storage

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"gophkeeper/internal/domain"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Ошибки хранилища.
var (
	ErrUserNotFound = errors.New("user not found")
	ErrItemNotFound = errors.New("item not found")
	ErrUserExists   = errors.New("user already exists")
)

// Storage — доступ к данным сервера через PostgreSQL.
type Storage struct {
	pool *pgxpool.Pool
}

// New открывает пул соединений к PostgreSQL и применяет миграции.
func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := runMigrations(dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Storage{pool: pool}, nil
}

func runMigrations(dsn string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// Close закрывает пул соединений.
func (s *Storage) Close() error {
	s.pool.Close()
	return nil
}

// CreateUser создаёт пользователя.
func (s *Storage) CreateUser(ctx context.Context, login, passwordHash string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users(id, login, password_hash, created_at) VALUES($1, $2, $3, $4)`,
		id, login, passwordHash, time.Now().UTC(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return uuid.Nil, ErrUserExists
		}
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

// GetUserByLogin возвращает пользователя по логину.
func (s *Storage) GetUserByLogin(ctx context.Context, login string) (*domain.User, error) {
	user := &domain.User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, login, password_hash, created_at FROM users WHERE login = $1`, login,
	).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// UpsertItem создаёт или обновляет запись сейфа (last-write-wins по version).
func (s *Storage) UpsertItem(ctx context.Context, item *domain.VaultItem) error {
	return upsertItem(ctx, s.pool, item)
}

// SyncItems атомарно применяет входящие изменения и возвращает записи, изменённые после since.
func (s *Storage) SyncItems(ctx context.Context, userID uuid.UUID, since time.Time, items []domain.VaultItem) ([]domain.VaultItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin sync tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i := range items {
		items[i].UserID = userID
		if err := upsertItem(ctx, tx, &items[i]); err != nil {
			return nil, err
		}
	}

	serverItems, err := listItemsSince(ctx, tx, userID, since)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit sync tx: %w", err)
	}
	return serverItems, nil
}

type execQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func upsertItem(ctx context.Context, db execQuerier, item *domain.VaultItem) error {
	_, err := db.Exec(ctx, `
INSERT INTO vault_items(id, user_id, version, updated_at, deleted, payload)
VALUES($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
	version = EXCLUDED.version,
	updated_at = EXCLUDED.updated_at,
	deleted = EXCLUDED.deleted,
	payload = EXCLUDED.payload
WHERE vault_items.user_id = EXCLUDED.user_id
  AND vault_items.version < EXCLUDED.version`,
		item.ID, item.UserID, item.Version, item.UpdatedAt.UTC(), item.Deleted, item.Payload,
	)
	if err != nil {
		return fmt.Errorf("upsert item: %w", err)
	}
	return nil
}

func listItemsSince(ctx context.Context, db execQuerier, userID uuid.UUID, since time.Time) ([]domain.VaultItem, error) {
	rows, err := db.Query(ctx, `
SELECT id, user_id, version, updated_at, deleted, payload
FROM vault_items
WHERE user_id = $1 AND updated_at > $2
ORDER BY updated_at ASC`, userID, since.UTC())
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()
	return scanItems(rows)
}

// ListItemsSince возвращает записи пользователя, изменённые после since.
func (s *Storage) ListItemsSince(ctx context.Context, userID uuid.UUID, since time.Time) ([]domain.VaultItem, error) {
	return listItemsSince(ctx, s.pool, userID, since)
}

// ListAllItems возвращает все записи пользователя.
func (s *Storage) ListAllItems(ctx context.Context, userID uuid.UUID) ([]domain.VaultItem, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, user_id, version, updated_at, deleted, payload
FROM vault_items
WHERE user_id = $1
ORDER BY updated_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list all items: %w", err)
	}
	defer rows.Close()
	return scanItems(rows)
}

// GetItem возвращает запись по ID.
func (s *Storage) GetItem(ctx context.Context, userID, itemID uuid.UUID) (*domain.VaultItem, error) {
	item := &domain.VaultItem{}
	err := s.pool.QueryRow(ctx, `
SELECT id, user_id, version, updated_at, deleted, payload
FROM vault_items
WHERE id = $1 AND user_id = $2`, itemID, userID,
	).Scan(&item.ID, &item.UserID, &item.Version, &item.UpdatedAt, &item.Deleted, &item.Payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotFound
		}
		return nil, fmt.Errorf("get item: %w", err)
	}
	return item, nil
}

// TruncateForTest очищает таблицы (для интеграционных тестов).
func (s *Storage) TruncateForTest(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `TRUNCATE vault_items, users CASCADE`)
	return err
}

func scanItems(rows pgx.Rows) ([]domain.VaultItem, error) {
	var items []domain.VaultItem
	for rows.Next() {
		var item domain.VaultItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.Version, &item.UpdatedAt, &item.Deleted, &item.Payload); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
