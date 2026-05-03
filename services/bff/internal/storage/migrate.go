package storage

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed all:migrations/postgres
var migrationsFS embed.FS

// RunMigrations applies all pending PostgreSQL migrations embedded in the binary.
// It is a no-op if the schema is already up to date.
func RunMigrations(databaseURL string) error {
	sub, err := fs.Sub(migrationsFS, "migrations/postgres")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	src, err := iofs.New(sub, ".")
	if err != nil {
		return fmt.Errorf("migration iofs: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, normalizePgxURL(databaseURL))
	if err != nil {
		return fmt.Errorf("migration init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration up: %w", err)
	}
	return nil
}

func normalizePgxURL(dsn string) string {
	switch {
	case len(dsn) >= 11 && dsn[:11] == "postgresql:":
		return "pgx5:" + dsn[11:]
	case len(dsn) >= 9 && dsn[:9] == "postgres:":
		return "pgx5:" + dsn[9:]
	case len(dsn) >= 5 && dsn[:5] == "pgx5:":
		return dsn
	default:
		return "pgx5://" + dsn
	}
}
