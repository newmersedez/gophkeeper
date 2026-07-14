package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

// TestDSN возвращает DSN тестовой PostgreSQL БД.
func TestDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URI"); dsn != "" {
		return dsn
	}
	return "postgres://localhost:5432/gophkeeper_test?sslmode=disable"
}

// OpenTest открывает хранилище для тестов или пропускает тест, если БД недоступна.
// Использует file lock, чтобы параллельные пакеты не мешали друг другу на общей БД.
func OpenTest(t *testing.T) *Storage {
	t.Helper()

	lockPath := filepath.Join(os.TempDir(), "gophkeeper-test-pg.lock")
	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		t.Fatalf("lock test db: %v", err)
	}
	t.Cleanup(func() { _ = lock.Unlock() })

	ctx := context.Background()
	store, err := New(ctx, TestDSN())
	if err != nil {
		t.Skip("PostgreSQL not available:", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.TruncateForTest(ctx); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return store
}
