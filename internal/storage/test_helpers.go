package storage

import (
	"os"
	"testing"
)

// setupTestService creates a test service backed by a PostgreSQL database.
// The DATABASE_URL environment variable must be set; otherwise the test is skipped.
func setupTestService(t *testing.T) *Service {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	// Run migrations
	migrationMgr, err := NewMigrationManager(dsn)
	if err != nil {
		t.Fatalf("Failed to create migration manager: %v", err)
	}

	if err := migrationMgr.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	_ = migrationMgr.Close()

	// Open database
	config := DefaultConfig()
	config.DatabaseURL = dsn
	db, err := Open(config)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create service
	service := NewService(db)

	t.Cleanup(func() {
		_ = db.Close()
	})

	return service
}
