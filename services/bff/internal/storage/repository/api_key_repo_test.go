package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// fakeDB implements the repository.DB interface using in-memory slices.
// This is a lightweight substitute; integration tests that hit a real
// PostgreSQL instance are tracked separately.
type fakeDB struct {
	rows    []fakeRow
	nextID  int64
	execErr error
}

type fakeRow struct {
	id         int64
	userID     int64
	keyHash    string
	createdAt  time.Time
	lastUsedAt *time.Time
	revoked    bool
}

func (f *fakeDB) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	if f.execErr != nil {
		return nil, f.execErr
	}
	// UpdateLastUsedAt(id) — just a no-op in the fake.
	return nil, nil
}

func (f *fakeDB) QueryContext(_ context.Context, _ string, args ...any) (*sql.Rows, error) {
	// Not needed for the unit test cases we cover here.
	return nil, nil
}

func (f *fakeDB) QueryRowContext(_ context.Context, _ string, args ...any) *sql.Row {
	// Not easily faked without a real DB — integration tests cover this path.
	return nil
}

func TestAPIKeyRepository_Interface(t *testing.T) {
	// Confirm NewAPIKeyRepository compiles with a DB implementation.
	var db repository.DB = &fakeDB{}
	repo := repository.NewAPIKeyRepository(db)

	if repo == nil {
		t.Fatal("NewAPIKeyRepository returned nil")
	}
}

func TestAPIKeyRepository_UpdateLastUsedAt_PropagatesError(t *testing.T) {
	db := &fakeDB{execErr: context.DeadlineExceeded}
	repo := repository.NewAPIKeyRepository(db)

	err := repo.UpdateLastUsedAt(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error from UpdateLastUsedAt, got nil")
	}
}
