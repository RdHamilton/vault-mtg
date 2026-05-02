package storage

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // PostgreSQL driver for migrations
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed all:migrations/postgres
var migrationsFS embed.FS

// MigrationManager handles database schema migrations.
type MigrationManager struct {
	migrate *migrate.Migrate
}

// NewMigrationManager creates a new migration manager for PostgreSQL.
// The dsn must be a PostgreSQL connection string.
func NewMigrationManager(dsn string) (*MigrationManager, error) {
	migrationsDir, err := fs.Sub(migrationsFS, "migrations/postgres")
	if err != nil {
		return nil, fmt.Errorf("failed to access migrations directory: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsDir, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to create source driver: %w", err)
	}

	// golang-migrate pgx/v5 driver expects "pgx5://" scheme.
	// If caller passes a standard postgres:// URL we normalise it.
	databaseURL := normalizePgxURL(dsn)

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration instance: %w", err)
	}

	return &MigrationManager{migrate: m}, nil
}

// normalizePgxURL converts a postgres:// or postgresql:// DSN to the
// pgx5:// scheme expected by golang-migrate's pgx/v5 driver.
// Key=value style DSNs are passed through unchanged (driver handles them).
func normalizePgxURL(dsn string) string {
	switch {
	case len(dsn) >= 11 && dsn[:11] == "postgresql:":
		return "pgx5:" + dsn[11:]
	case len(dsn) >= 9 && dsn[:9] == "postgres:":
		return "pgx5:" + dsn[9:]
	case len(dsn) >= 5 && dsn[:5] == "pgx5:":
		return dsn
	default:
		// Key=value DSN — wrap as pgx5://
		return "pgx5://" + dsn
	}
}

// Up applies all pending migrations.
func (mm *MigrationManager) Up() error {
	err := mm.migrate.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}

// Down rolls back the last migration.
func (mm *MigrationManager) Down() error {
	err := mm.migrate.Down()
	if err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}
	return nil
}

// Steps applies n migrations. Positive n applies up migrations, negative applies down.
func (mm *MigrationManager) Steps(n int) error {
	err := mm.migrate.Steps(n)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to migrate %d steps: %w", n, err)
	}
	return nil
}

// Version returns the current migration version and dirty state.
func (mm *MigrationManager) Version() (version uint, dirty bool, err error) {
	version, dirty, err = mm.migrate.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}
	return version, dirty, nil
}

// Goto migrates to a specific version.
func (mm *MigrationManager) Goto(version uint) error {
	err := mm.migrate.Migrate(version)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to migrate to version %d: %w", version, err)
	}
	return nil
}

// Force sets the migration version without running migrations.
// Use with caution — for recovering from failed migrations.
func (mm *MigrationManager) Force(version int) error {
	err := mm.migrate.Force(version)
	if err != nil {
		return fmt.Errorf("failed to force version %d: %w", version, err)
	}
	return nil
}

// Close closes the migration manager and releases resources.
func (mm *MigrationManager) Close() error {
	srcErr, dbErr := mm.migrate.Close()
	if srcErr != nil {
		return fmt.Errorf("failed to close source: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("failed to close database: %w", dbErr)
	}
	return nil
}
