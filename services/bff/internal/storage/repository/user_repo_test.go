package repository_test

import (
	"context"
	"testing"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// TestUserRepository_Interface verifies NewUserRepository compiles correctly
// with any DB implementation.
func TestUserRepository_Interface(t *testing.T) {
	// fakeDB is defined in api_key_repo_test.go in the same package.
	var db repository.DB = &fakeDB{}
	repo := repository.NewUserRepository(db)

	if repo == nil {
		t.Fatal("NewUserRepository returned nil")
	}
}

// TestUserRepository_UpsertByClerkUserID_CreatesRow verifies that upserting a
// brand-new Clerk user ID inserts a row and returns it.
// Requires TEST_DATABASE_URL — skipped otherwise.
func TestUserRepository_UpsertByClerkUserID_CreatesRow(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewUserRepository(db)

	clerkID := "user_test_create_" + t.Name()

	u, err := repo.UpsertByClerkUserID(context.Background(), clerkID)
	if err != nil {
		t.Fatalf("UpsertByClerkUserID: %v", err)
	}

	if u == nil {
		t.Fatal("UpsertByClerkUserID returned nil user")
	}

	if u.ID == 0 {
		t.Error("expected non-zero user ID")
	}

	if u.ClerkUserID == nil || *u.ClerkUserID != clerkID {
		t.Errorf("ClerkUserID: want %q, got %v", clerkID, u.ClerkUserID)
	}

	wantEmail := clerkID + "@clerk.local"
	if u.Email != wantEmail {
		t.Errorf("Email placeholder: want %q, got %q", wantEmail, u.Email)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE clerk_user_id = $1", clerkID)
	})
}

// TestUserRepository_UpsertByClerkUserID_IdempotentOnConflict verifies that
// upserting the same Clerk user ID twice returns the same user row both times.
func TestUserRepository_UpsertByClerkUserID_IdempotentOnConflict(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewUserRepository(db)

	clerkID := "user_test_idempotent_" + t.Name()

	first, err := repo.UpsertByClerkUserID(context.Background(), clerkID)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	second, err := repo.UpsertByClerkUserID(context.Background(), clerkID)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("idempotent upsert IDs differ: first=%d second=%d", first.ID, second.ID)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE clerk_user_id = $1", clerkID)
	})
}

// TestUserRepository_GetByClerkUserID_NotFound verifies that a lookup for an
// unknown Clerk user ID returns (nil, nil) — no error.
func TestUserRepository_GetByClerkUserID_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewUserRepository(db)

	u, err := repo.GetByClerkUserID(context.Background(), "user_definitely_does_not_exist_xyz_999")
	if err != nil {
		t.Fatalf("GetByClerkUserID: %v", err)
	}

	if u != nil {
		t.Errorf("expected nil for unknown clerk ID, got %+v", u)
	}
}
