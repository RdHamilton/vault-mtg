//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// postgresTestDB opens a PostgreSQL connection using the POSTGRES_TEST_DSN env var.
// Run integration tests with:
//
//	POSTGRES_TEST_DSN="postgres://user:pass@localhost/mtga_test?sslmode=disable" go test -tsc -tags integration ./internal/storage/repository/...
func postgresTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping PostgreSQL integration tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	setupUserTestSchema(t, db)
	return db
}

func setupUserTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE EXTENSION IF NOT EXISTS pgcrypto;

		DROP TABLE IF EXISTS accounts CASCADE;
		DROP TABLE IF EXISTS users CASCADE;

		CREATE TABLE users (
			id                  BIGSERIAL PRIMARY KEY,
			email               TEXT NOT NULL UNIQUE,
			api_key             TEXT NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(32), 'hex'),
			subscription_status TEXT NOT NULL DEFAULT 'free'
			                        CHECK (subscription_status IN ('free', 'pro')),
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE accounts (
			id        BIGSERIAL PRIMARY KEY,
			user_id   BIGINT REFERENCES users(id) ON DELETE CASCADE,
			name      TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		t.Fatalf("setup schema: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS accounts CASCADE")
		db.Exec("DROP TABLE IF EXISTS users CASCADE")
	})
}

func TestUserRepository_Create(t *testing.T) {
	db := postgresTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &models.User{
		Email:              "test@example.com",
		SubscriptionStatus: "free",
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if user.ID == 0 {
		t.Error("expected ID to be set after Create")
	}
	if user.APIKey == "" {
		t.Error("expected APIKey to be populated from database default")
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := postgresTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	created := &models.User{Email: "byid@example.com", SubscriptionStatus: "free"}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Email != created.Email {
		t.Errorf("email: got %q, want %q", got.Email, created.Email)
	}

	missing, err := repo.GetByID(ctx, 999999)
	if err != nil {
		t.Fatalf("GetByID missing: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing user")
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := postgresTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	created := &models.User{Email: "byemail@example.com", SubscriptionStatus: "free"}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByEmail(ctx, "byemail@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got == nil || got.ID != created.ID {
		t.Errorf("GetByEmail: got %v, want id %d", got, created.ID)
	}

	missing, err := repo.GetByEmail(ctx, "nobody@example.com")
	if err != nil {
		t.Fatalf("GetByEmail missing: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for unknown email")
	}
}

func TestUserRepository_GetByAPIKey(t *testing.T) {
	db := postgresTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	created := &models.User{Email: "bykey@example.com", SubscriptionStatus: "free"}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByAPIKey(ctx, created.APIKey)
	if err != nil {
		t.Fatalf("GetByAPIKey: %v", err)
	}
	if got == nil || got.ID != created.ID {
		t.Errorf("GetByAPIKey: got %v, want id %d", got, created.ID)
	}
}

func TestUserRepository_UpdateSubscriptionStatus(t *testing.T) {
	db := postgresTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &models.User{Email: "sub@example.com", SubscriptionStatus: "free"}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateSubscriptionStatus(ctx, user.ID, "pro"); err != nil {
		t.Fatalf("UpdateSubscriptionStatus: %v", err)
	}

	updated, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.SubscriptionStatus != "pro" {
		t.Errorf("status: got %q, want %q", updated.SubscriptionStatus, "pro")
	}
}

func TestUserRepository_Delete(t *testing.T) {
	db := postgresTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &models.User{Email: "delete@example.com", SubscriptionStatus: "free"}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	gone, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if gone != nil {
		t.Error("expected nil after delete")
	}
}

// Verify updated_at advances after UpdateSubscriptionStatus.
func TestUserRepository_UpdatedAtChanges(t *testing.T) {
	db := postgresTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &models.User{Email: "ts@example.com", SubscriptionStatus: "free"}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before := user.UpdatedAt

	time.Sleep(10 * time.Millisecond)
	if err := repo.UpdateSubscriptionStatus(ctx, user.ID, "pro"); err != nil {
		t.Fatalf("UpdateSubscriptionStatus: %v", err)
	}

	updated, _ := repo.GetByID(ctx, user.ID)
	if !updated.UpdatedAt.After(before) {
		t.Error("expected updated_at to advance after update")
	}
}
