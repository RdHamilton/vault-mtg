package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// migrateOnce ensures migrations are only run once per test binary execution.
var (
	migrateOnce sync.Once
	migrateErr  error
)

// repoTestDB returns a *sql.DB connected to the PostgreSQL database specified
// by DATABASE_URL. The test is skipped if DATABASE_URL is not set.
//
// Migrations are run once per test binary. Each call registers a t.Cleanup
// that truncates all application tables so tests are isolated from each other.
func repoTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	// Run migrations exactly once per test binary.
	migrateOnce.Do(func() {
		m, err := migrate.New("file://../migrations/postgres", normalizePgxDSN(dsn))
		if err != nil {
			migrateErr = fmt.Errorf("create migrator: %w", err)
			return
		}
		defer m.Close()
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			migrateErr = fmt.Errorf("run migrations: %w", err)
		}
	})
	if migrateErr != nil {
		t.Fatalf("migration failed: %v", migrateErr)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping database: %v", err)
	}

	t.Cleanup(func() {
		truncateAllTables(t, db)
		_ = db.Close()
	})

	return db
}

// truncateAllTables removes all rows from application tables so each test
// starts with a clean state. RESTART IDENTITY resets sequences so that
// auto-increment IDs are predictable (starting at 1) within each test.
func truncateAllTables(t *testing.T, db *sql.DB) {
	t.Helper()

	// Single TRUNCATE with CASCADE handles all FK dependencies automatically.
	// RESTART IDENTITY resets all serial/bigserial sequences.
	tables := []string{
		"draft_picks",
		"draft_card_ratings",
		"draft_color_statistics",
		"draft_match_results",
		"draft_temporal_trends",
		"draft_pattern_analysis",
		"draft_community_comparison",
		"rank_history",
		"currency_history",
		"quests",
		"deck_notes",
		"game_plays",
		"games",
		"matches",
		"deck_permutation_cards",
		"deck_permutations",
		"deck_archetypes",
		"deck_card_weights",
		"deck_performance_stats",
		"deck_tags",
		"deck_cards",
		"decks",
		"set_cards",
		"cfb_card_ratings",
		"draft_ratings",
		"improvement_suggestions",
		"ml_suggestions",
		"card_combination_stats",
		"user_play_patterns",
		"ml_model_metadata",
		"ml_training_data",
		"recommendation_feedback",
		"inventory",
		"collection",
		"settings",
		"migration_log",
		"processed_log_files",
		"draft_sessions",
		"deck_performance",
		"card_performance",
		"stats",
		"accounts",
		"users",
	}

	for _, table := range tables {
		if _, err := db.Exec("TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE"); err != nil {
			// Ignore errors for tables that don't exist in this schema version.
			_ = err
		}
	}
}

// normalizePgxDSN converts a postgres:// or postgresql:// DSN to the pgx5:// scheme
// expected by golang-migrate's pgx/v5 driver.
func normalizePgxDSN(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "postgresql:"):
		return "pgx5:" + dsn[11:]
	case strings.HasPrefix(dsn, "postgres:"):
		return "pgx5:" + dsn[9:]
	case strings.HasPrefix(dsn, "pgx5:"):
		return dsn
	default:
		return "pgx5://" + dsn
	}
}
